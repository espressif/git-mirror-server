package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type repoCounter struct {
	fetchCount atomic.Uint64
}

var repoCounters = make(map[string]*repoCounter)
var repoCountersMu sync.Mutex

const maxLogOutput = 512
const healthCheckTimeout = 10 * time.Minute

func truncateOutput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLogOutput {
		return s
	}
	return s[:maxLogOutput] + "... (truncated)"
}

// safeRepoPath returns the cleaned repo path and validates it stays within basePath.
func safeRepoPath(basePath, name string) (string, error) {
	p := filepath.Clean(filepath.Join(basePath, name))
	rel, err := filepath.Rel(basePath, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("repo %s resolves outside base path", name)
	}
	return p, nil
}

func healthCheck(cfg config, repos map[string]repo) {
	for name, r := range repos {
		repoPath, err := safeRepoPath(cfg.BasePath, r.Name)
		if err != nil {
			log.Printf("warning: %s, skipping", err)
			continue
		}

		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		removeLockFiles(repoPath, name)

		ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)

		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			cancel()
			revParseErr := fmt.Errorf("repo %s is not a valid git repo (rev-parse failed: %w, output: %s)", name, err, truncateOutput(string(out)))
			log.Printf("warning: %s, removing", revParseErr)
			captureError(revParseErr, name, "health-check")
			if err := os.RemoveAll(repoPath); err != nil {
				log.Printf("error removing %s: %s", repoPath, err)
			}
			continue
		}

		cmd = exec.CommandContext(ctx, "git", "fsck", "--no-dangling", "--connectivity-only")
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			cancel()
			fsckErr := fmt.Errorf("repo %s failed fsck: %w, output: %s", name, err, truncateOutput(string(out)))
			log.Printf("warning: %s, removing", fsckErr)
			captureError(fsckErr, name, "health-check")
			if err := os.RemoveAll(repoPath); err != nil {
				log.Printf("error removing %s: %s", repoPath, err)
			}
			continue
		}

		cancel()
		log.Printf("health check passed for %s", name)
	}
}

func removeLockFiles(repoPath string, name string) {
	if err := filepath.WalkDir(repoPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("error accessing %s during lock cleanup: %s", p, err)
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".lock") {
			log.Printf("removing stale lock file %s in %s", p, name)
			if err := os.Remove(p); err != nil {
				log.Printf("error removing lock file %s: %s", p, err)
			}
		}
		return nil
	}); err != nil {
		log.Printf("error walking directory %s: %s", repoPath, err)
	}
}

func mirror(ctx context.Context, cfg config, r repo) (string, error) {
	repoPath, err := safeRepoPath(cfg.BasePath, r.Name)
	if err != nil {
		return "", err
	}
	outStr := ""

	repoCountersMu.Lock()
	if repoCounters[r.Name] == nil {
		repoCounters[r.Name] = &repoCounter{}
	}
	counter := repoCounters[r.Name]
	repoCountersMu.Unlock()

	if _, err := os.Stat(repoPath); err == nil {
		cmd := exec.CommandContext(ctx, "git", "remote", "update", "--prune")
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		outStr = string(out)
		if err != nil {
			return outStr, fmt.Errorf("failed to update remote in %s: %w", repoPath, err)
		}

	} else if os.IsNotExist(err) {
		parent := filepath.Dir(repoPath)
		if err := os.MkdirAll(parent, 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directory for cloning %s, %s", repoPath, err)
		}
		cmd := exec.CommandContext(ctx, "git", "clone", "--mirror", r.Origin, repoPath)
		cmd.Dir = parent
		out, err := cmd.CombinedOutput()
		if err != nil {
			if rmErr := os.RemoveAll(repoPath); rmErr != nil {
				log.Printf("error cleaning up partial clone directory %s: %s", repoPath, rmErr)
			}
			return "", fmt.Errorf("failed to clone %s: %w\noutput: %s", r.Origin, err, truncateOutput(string(out)))
		}
		return string(out), err
	} else {
		return "", fmt.Errorf("failed to stat %s, %s", repoPath, err)
	}

	if err := refreshCommitGraph(ctx, cfg, r); err != nil {
		log.Printf("error refreshing commit-graph for %s: %s", r.Name, err)
		captureError(err, r.Name, "commit-graph")
	}

	count := counter.fetchCount.Load()
	if r.MultiPackIndexInterval > 0 && count > 0 && count%uint64(r.MultiPackIndexInterval) == 0 {
		if err := prunePacked(ctx, cfg, r); err != nil {
			log.Printf("error pruning packed objects for %s: %s", r.Name, err)
			captureError(err, r.Name, "prune-packed")
		}
		if err := refreshMultiPackIndex(ctx, cfg, r); err != nil {
			log.Printf("error refreshing multi-pack index for %s: %s", r.Name, err)
			captureError(err, r.Name, "multi-pack-index")
		} else {
			log.Printf("successfully refreshed multi-pack index for %s (fetch #%d)", r.Name, count)
		}
	}

	counter.fetchCount.Add(1)

	return outStr, nil
}

func refreshCommitGraph(ctx context.Context, cfg config, r repo) error {
	repoPath, err := safeRepoPath(cfg.BasePath, r.Name)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "commit-graph", "write", "--reachable")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to write commit-graph for %s: %s, output: %s", repoPath, err, truncateOutput(string(out)))
	}

	return nil
}

func hasPackFiles(repoPath string) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(repoPath, "objects", "pack"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".pack") {
			return true, nil
		}
	}
	return false, nil
}

func refreshMultiPackIndex(ctx context.Context, cfg config, r repo) error {
	repoPath, err := safeRepoPath(cfg.BasePath, r.Name)
	if err != nil {
		return err
	}

	hasPacks, err := hasPackFiles(repoPath)
	if err != nil {
		return fmt.Errorf("failed to check pack files in %s: %w", repoPath, err)
	}
	if !hasPacks {
		return nil
	}

	cmd := exec.CommandContext(ctx, "git", "multi-pack-index", "write")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to write multi-pack-index %s: %s, output: %s", repoPath, err, truncateOutput(string(out)))
	}

	// Bitmap is best-effort: it requires full object closure in packs,
	// which may not hold in a mirror that receives incremental fetches.
	cmd = exec.CommandContext(ctx, "git", "multi-pack-index", "write", "--bitmap")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("warning: failed to write multi-pack bitmap for %s (non-fatal): err=%v, output=%s", r.Name, err, truncateOutput(string(out)))
	}

	return nil
}

func prunePacked(ctx context.Context, cfg config, r repo) error {
	repoPath, err := safeRepoPath(cfg.BasePath, r.Name)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "prune-packed")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to prune packed objects in %s: %s, output: %s", repoPath, err, truncateOutput(string(out)))
	}

	return nil
}
