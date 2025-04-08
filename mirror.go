package main

import (
	"fmt"
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
			fmt.Fprintf(os.Stderr, "failed to update remote in %s, %s", repoPath, err)
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
			fmt.Fprintf(os.Stderr, "failed to clone %s, %s", r.Origin, err)
		}
		return string(out), err
	} else {
		return "", fmt.Errorf("failed to stat %s, %s", repoPath, err)
	}
	return outStr, nil
}
