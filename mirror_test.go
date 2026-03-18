package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s failed: %s\n%s", args, dir, err, out)
	}
}

func setupTestEnv(t *testing.T) (srcDir string, cfg config, r repo) {
	t.Helper()
	tmpDir := t.TempDir()

	srcDir = filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, srcDir, "init", "-b", "main")
	gitCmd(t, srcDir, "commit", "--allow-empty", "-m", "initial")

	cfg = config{BasePath: filepath.Join(tmpDir, "mirrors")}
	r = repo{
		Name:                   "test-repo",
		Origin:                 srcDir,
		Interval:               duration{time.Second},
		MultiPackIndexInterval: 1,
	}

	resetCounters()
	return
}

func resetCounters() {
	repoCountersMu.Lock()
	repoCounters = make(map[string]*repoCounter)
	repoCountersMu.Unlock()
}

func TestRefreshCommitGraph(t *testing.T) {
	srcDir, cfg, r := setupTestEnv(t)
	bareDir := filepath.Join(cfg.BasePath, r.Name)
	if err := os.MkdirAll(filepath.Dir(bareDir), 0755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, filepath.Dir(bareDir), "clone", "--mirror", srcDir, bareDir)

	if err := refreshCommitGraph(cfg, r); err != nil {
		t.Fatalf("refreshCommitGraph failed: %s", err)
	}

	cgPath := filepath.Join(bareDir, "objects", "info", "commit-graph")
	if _, err := os.Stat(cgPath); os.IsNotExist(err) {
		t.Fatal("commit-graph file was not created")
	}
}

func TestRefreshMultiPackIndex(t *testing.T) {
	srcDir, cfg, r := setupTestEnv(t)
	bareDir := filepath.Join(cfg.BasePath, r.Name)
	if err := os.MkdirAll(filepath.Dir(bareDir), 0755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, filepath.Dir(bareDir), "clone", "--mirror", srcDir, bareDir)
	// Repack loose objects into a pack file so MIDX has something to index
	gitCmd(t, bareDir, "repack", "-d")

	if err := refreshMultiPackIndex(cfg, r); err != nil {
		t.Fatalf("refreshMultiPackIndex failed: %s", err)
	}

	midxPath := filepath.Join(bareDir, "objects", "pack", "multi-pack-index")
	if _, err := os.Stat(midxPath); os.IsNotExist(err) {
		t.Fatal("multi-pack-index file was not created")
	}
}

func TestMirrorCreatesCommitGraphOnUpdate(t *testing.T) {
	srcDir, cfg, r := setupTestEnv(t)

	// First call: clone
	if _, err := mirror(cfg, r); err != nil {
		t.Fatalf("initial mirror failed: %s", err)
	}

	// Add commit to source
	gitCmd(t, srcDir, "commit", "--allow-empty", "-m", "second")

	// Second call: update
	if _, err := mirror(cfg, r); err != nil {
		t.Fatalf("mirror update failed: %s", err)
	}

	bareDir := filepath.Join(cfg.BasePath, r.Name)
	cgPath := filepath.Join(bareDir, "objects", "info", "commit-graph")
	if _, err := os.Stat(cgPath); os.IsNotExist(err) {
		t.Fatal("commit-graph was not created after mirror update")
	}
}

func TestMirrorMultiPackIndexOnInterval(t *testing.T) {
	srcDir, cfg, r := setupTestEnv(t)
	r.MultiPackIndexInterval = 2

	// Clone
	if _, err := mirror(cfg, r); err != nil {
		t.Fatalf("initial mirror failed: %s", err)
	}

	bareDir := filepath.Join(cfg.BasePath, r.Name)
	// Repack loose objects so MIDX has pack files to index
	gitCmd(t, bareDir, "repack", "-d")
	midxPath := filepath.Join(bareDir, "objects", "pack", "multi-pack-index")

	// First update (fetchCount=0, 0%2==0 → writes MIDX)
	gitCmd(t, srcDir, "commit", "--allow-empty", "-m", "second")
	if _, err := mirror(cfg, r); err != nil {
		t.Fatalf("mirror update 1 failed: %s", err)
	}
	if _, err := os.Stat(midxPath); os.IsNotExist(err) {
		t.Fatal("multi-pack-index should exist after first update (fetchCount=0)")
	}

	// Remove MIDX to verify it's NOT recreated on next update
	if err := os.Remove(midxPath); err != nil {
		t.Fatal(err)
	}

	// Second update (fetchCount=1, 1%2!=0 → should NOT write MIDX)
	gitCmd(t, srcDir, "commit", "--allow-empty", "-m", "third")
	if _, err := mirror(cfg, r); err != nil {
		t.Fatalf("mirror update 2 failed: %s", err)
	}
	if _, err := os.Stat(midxPath); !os.IsNotExist(err) {
		t.Fatal("multi-pack-index should NOT exist after second update (fetchCount=1)")
	}
}
