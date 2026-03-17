package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
)

// Counter tracks fetch counts for each repo
type repoCounter struct {
	mu         sync.Mutex
	fetchCount uint64
}

var repoCounters = make(map[string]*repoCounter)
var repoCountersMu sync.Mutex

func mirror(cfg config, r repo) (string, error) {
	repoPath := path.Join(cfg.BasePath, r.Name)
	outStr := ""

	// Initialize counter for this repo if it doesn't exist
	repoCountersMu.Lock()
	if repoCounters[r.Name] == nil {
		repoCounters[r.Name] = &repoCounter{}
	}
	counter := repoCounters[r.Name]
	repoCountersMu.Unlock()

	if _, err := os.Stat(repoPath); err == nil {
		// Directory exists, update.
		cmd := exec.Command("git", "remote", "update", "--prune")
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		outStr = string(out)
		if err != nil {
			return "", fmt.Errorf("failed to update remote in %s: %w", repoPath, err)
		}

	} else if os.IsNotExist(err) {
		// Clone
		parent := path.Dir(repoPath)
		if err := os.MkdirAll(parent, 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directory for cloning %s, %s", repoPath, err)
		}
		cmd := exec.Command("git", "clone", "--mirror", r.Origin, repoPath)
		cmd.Dir = parent
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to clone %s: %w", r.Origin, err)
		}
		return string(out), err
	} else {
		return "", fmt.Errorf("failed to stat %s, %s", repoPath, err)
	}

	// Commit-graph is cheap and safe to run after every fetch
	if err := refreshCommitGraph(cfg, r); err != nil {
		log.Printf("error refreshing commit-graph for %s: %s", r.Name, err)
	}

	// Multi-pack-index with bitmap runs every N fetches
	if r.MultiPackIndexInterval > 0 && counter.fetchCount%uint64(r.MultiPackIndexInterval) == 0 {
		if err := refreshMultiPackIndex(cfg, r); err != nil {
			log.Printf("error refreshing multi-pack index for %s: %s", r.Name, err)
		} else {
			log.Printf("successfully refreshed multi-pack index for %s (fetch #%d)", r.Name, counter.fetchCount)
		}
	}

	// Increment fetch counter (only on successful fetch)
	counter.mu.Lock()
	counter.fetchCount++
	counter.mu.Unlock()

	return outStr, nil
}

func refreshCommitGraph(cfg config, r repo) error {
	repoPath := path.Join(cfg.BasePath, r.Name)

	cmd := exec.Command("git", "commit-graph", "write", "--reachable")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to write commit-graph for %s: %s, output: %s", repoPath, err, string(out))
	}

	return nil
}

// Write multi-pack-index with bitmap across all packs without full repack
func refreshMultiPackIndex(cfg config, r repo) error {
	repoPath := path.Join(cfg.BasePath, r.Name)

	// Run git multi-pack-index write with bitmap
	mpiCmd := exec.Command("git", "multi-pack-index", "write", "--bitmap")
	mpiCmd.Dir = repoPath
	if out, err := mpiCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to write multi-pack-index %s: %s, output: %s", repoPath, err, string(out))
	}

	return nil
}
