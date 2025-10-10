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

	// Check if we need to run multi-pack index
	if r.MultiPackIndexInterval > 0 && counter.fetchCount%uint64(r.MultiPackIndexInterval) == 0 {
		if err := refreshMultiPackIndex(cfg, r); err != nil {
			log.Printf("error refreshing multi-pack index for %s: %s", r.Name, err)
		} else {
			log.Printf("successfully refreshed multi-pack index for %s (fetch #%d)", r.Name, counter.fetchCount)
		}
	}

	// Check if we need to run bitmap index
	if r.BitmapIndexInterval > 0 && counter.fetchCount%uint64(r.BitmapIndexInterval) == 0 {
		if err := refreshBitmapIndex(cfg, r); err != nil {
			log.Printf("error refreshing bitmap index for %s: %s", r.Name, err)
		} else {
			log.Printf("successfully refreshed bitmap index for %s (fetch #%d)", r.Name, counter.fetchCount)
		}
	}

	// Increment fetch counter (only on successful fetch)
	counter.mu.Lock()
	counter.fetchCount++
	counter.mu.Unlock()

	return outStr, nil
}

// Rebuild git bitmap index for the repo to speed up fetches once in a while
func refreshBitmapIndex(cfg config, r repo) error {
	repoPath := path.Join(cfg.BasePath, r.Name)

	// Run git repack with bitmap index
	repackCmd := exec.Command("git", "repack", "-Ad", "--write-bitmap-index", "--pack-kept-objects")
	repackCmd.Dir = repoPath
	if out, err := repackCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to repack %s: %s, output: %s", repoPath, err, string(out))
	}

	return nil
}

// Quickly write multi-pack-index with bitmap without full repack
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
