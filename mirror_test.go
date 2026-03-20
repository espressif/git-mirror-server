package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	if err := refreshCommitGraph(context.Background(), cfg, r); err != nil {
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

	if err := refreshMultiPackIndex(context.Background(), cfg, r); err != nil {
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
	if _, err := mirror(context.Background(), cfg, r); err != nil {
		t.Fatalf("initial mirror failed: %s", err)
	}

	// Add commit to source
	gitCmd(t, srcDir, "commit", "--allow-empty", "-m", "second")

	// Second call: update
	if _, err := mirror(context.Background(), cfg, r); err != nil {
		t.Fatalf("mirror update failed: %s", err)
	}

	bareDir := filepath.Join(cfg.BasePath, r.Name)
	cgPath := filepath.Join(bareDir, "objects", "info", "commit-graph")
	if _, err := os.Stat(cgPath); os.IsNotExist(err) {
		t.Fatal("commit-graph was not created after mirror update")
	}
}

func TestHealthCheckRemovesLockFiles(t *testing.T) {
	srcDir, cfg, r := setupTestEnv(t)
	bareDir := filepath.Join(cfg.BasePath, r.Name)
	if err := os.MkdirAll(filepath.Dir(bareDir), 0755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, filepath.Dir(bareDir), "clone", "--mirror", srcDir, bareDir)

	lockFiles := []string{
		filepath.Join(bareDir, "objects", "info", "commit-graph.lock"),
		filepath.Join(bareDir, "objects", "pack", "multi-pack-index.lock"),
		filepath.Join(bareDir, "refs", "heads", "main.lock"),
		filepath.Join(bareDir, "packed-refs.lock"),
	}
	for _, lf := range lockFiles {
		if err := os.MkdirAll(filepath.Dir(lf), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lf, []byte{}, 0644); err != nil {
			t.Fatal(err)
		}
	}

	repos := map[string]repo{r.Name: r}
	healthCheck(cfg, repos)

	for _, lf := range lockFiles {
		if _, err := os.Stat(lf); !os.IsNotExist(err) {
			t.Fatalf("lock file %s should have been removed", lf)
		}
	}
	if _, err := os.Stat(bareDir); os.IsNotExist(err) {
		t.Fatal("valid repo directory should not be removed")
	}
}

func TestHealthCheckRemovesCorruptedRepo(t *testing.T) {
	_, cfg, r := setupTestEnv(t)
	bareDir := filepath.Join(cfg.BasePath, r.Name)
	if err := os.MkdirAll(bareDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Just a directory with a file — not a valid git repo
	if err := os.WriteFile(filepath.Join(bareDir, "garbage"), []byte("not a repo"), 0644); err != nil {
		t.Fatal(err)
	}

	repos := map[string]repo{r.Name: r}
	healthCheck(cfg, repos)

	if _, err := os.Stat(bareDir); !os.IsNotExist(err) {
		t.Fatal("corrupted repo directory should have been removed")
	}
}

func TestHealthCheckFsckFailure(t *testing.T) {
	srcDir, cfg, r := setupTestEnv(t)
	bareDir := filepath.Join(cfg.BasePath, r.Name)
	if err := os.MkdirAll(filepath.Dir(bareDir), 0755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, filepath.Dir(bareDir), "clone", "--mirror", srcDir, bareDir)

	// Corrupt the repo by removing all loose objects so fsck finds missing objects.
	// HEAD and refs remain valid so rev-parse still passes.
	objectsDir := filepath.Join(bareDir, "objects")
	entries, err := os.ReadDir(objectsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) == 2 {
			if err := os.RemoveAll(filepath.Join(objectsDir, e.Name())); err != nil {
				t.Fatalf("failed to remove object dir %s: %s", e.Name(), err)
			}
		}
	}
	packDir := filepath.Join(objectsDir, "pack")
	packs, err := filepath.Glob(filepath.Join(packDir, "*.pack"))
	if err != nil {
		t.Fatalf("failed to glob pack files: %s", err)
	}
	for _, p := range packs {
		if err := os.Remove(p); err != nil {
			t.Fatalf("failed to remove pack file %s: %s", p, err)
		}
		idx := strings.TrimSuffix(p, ".pack") + ".idx"
		if err := os.Remove(idx); err != nil {
			t.Fatalf("failed to remove idx file %s: %s", idx, err)
		}
	}

	repos := map[string]repo{r.Name: r}
	healthCheck(cfg, repos)

	if _, err := os.Stat(bareDir); !os.IsNotExist(err) {
		t.Fatal("repo with fsck failure should have been removed")
	}
}

func TestHealthCheckSkipsNonExistent(t *testing.T) {
	_, cfg, r := setupTestEnv(t)
	bareDir := filepath.Join(cfg.BasePath, r.Name)

	repos := map[string]repo{r.Name: r}
	healthCheck(cfg, repos)

	if _, err := os.Stat(bareDir); !os.IsNotExist(err) {
		t.Fatal("non-existent repo directory should not be created")
	}
}

func TestHealthCheckValidRepo(t *testing.T) {
	srcDir, cfg, r := setupTestEnv(t)
	bareDir := filepath.Join(cfg.BasePath, r.Name)
	if err := os.MkdirAll(filepath.Dir(bareDir), 0755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, filepath.Dir(bareDir), "clone", "--mirror", srcDir, bareDir)

	repos := map[string]repo{r.Name: r}
	healthCheck(cfg, repos)

	if _, err := os.Stat(bareDir); os.IsNotExist(err) {
		t.Fatal("valid repo directory should still exist after health check")
	}
}

func TestMirrorMultiPackIndexOnInterval(t *testing.T) {
	srcDir, cfg, r := setupTestEnv(t)
	r.MultiPackIndexInterval = 2

	// Clone
	if _, err := mirror(context.Background(), cfg, r); err != nil {
		t.Fatalf("initial mirror failed: %s", err)
	}

	bareDir := filepath.Join(cfg.BasePath, r.Name)
	// Repack loose objects so MIDX has pack files to index
	gitCmd(t, bareDir, "repack", "-d")
	midxPath := filepath.Join(bareDir, "objects", "pack", "multi-pack-index")

	// First update (fetchCount=0, 0%2==0 → writes MIDX)
	gitCmd(t, srcDir, "commit", "--allow-empty", "-m", "second")
	if _, err := mirror(context.Background(), cfg, r); err != nil {
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
	if _, err := mirror(context.Background(), cfg, r); err != nil {
		t.Fatalf("mirror update 2 failed: %s", err)
	}
	if _, err := os.Stat(midxPath); !os.IsNotExist(err) {
		t.Fatal("multi-pack-index should NOT exist after second update (fetchCount=1)")
	}
}

func TestMirrorCloneFailureRemovesPartialDir(t *testing.T) {
	_, cfg, r := setupTestEnv(t)
	r.Origin = "/nonexistent/repo"

	repoPath := filepath.Join(cfg.BasePath, r.Name)

	_, err := mirror(context.Background(), cfg, r)
	if err == nil {
		t.Fatal("expected clone to fail with invalid origin")
	}

	if _, statErr := os.Stat(repoPath); !os.IsNotExist(statErr) {
		t.Fatal("partial clone directory should have been removed after failure")
	}
}
