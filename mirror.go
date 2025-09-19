package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
)

func mirror(cfg config, r repo) (string, error) {
	repoPath := path.Join(cfg.BasePath, r.Name)
	outStr := ""
	if _, err := os.Stat(repoPath); err == nil {
		// Directory exists, update.
		cmd := exec.Command("git", "remote", "update", "--prune")
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		outStr = string(out)
		if err != nil {
			return "", fmt.Errorf("failed to update remote in %s: %w", repoPath, err)
		}
		if err := refreshMultiPackIndex(cfg, r); err != nil {
			log.Printf("error refreshing multi-pack index for %s: %s", r.Name, err)
		} else {
			log.Printf("successfully refreshed multi-pack index for %s", r.Name)
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
		if err := refreshBitmapIndex(cfg, r); err != nil {
			log.Printf("error refreshing bitmap index for %s: %s", r.Name, err)
		} else {
			log.Printf("successfully refreshed bitmap index for %s", r.Name)
		}
		return string(out), err
	} else {
		return "", fmt.Errorf("failed to stat %s, %s", repoPath, err)
	}
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
