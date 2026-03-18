package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(`
[[Repo]]
Origin = "https://example.com/repo.git"
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, repos, err := parseConfig(cfgPath)
	if err != nil {
		t.Fatalf("parseConfig failed: %s", err)
	}

	if cfg.MultiPackIndexInterval != 0 {
		t.Errorf("expected default MultiPackIndexInterval=0, got %d", cfg.MultiPackIndexInterval)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
}

func TestParseConfigWarnsOnDeprecatedBitmapIndexInterval(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(`
BitmapIndexInterval = 50

[[Repo]]
Origin = "https://example.com/repo.git"
BitmapIndexInterval = 25
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := parseConfig(cfgPath)
	if err != nil {
		t.Fatalf("parseConfig failed: %s", err)
	}

	if !strings.Contains(buf.String(), "BitmapIndexInterval") {
		t.Fatal("expected deprecation warning for BitmapIndexInterval")
	}
}

func TestParseConfigNoWarningWithoutDeprecatedFields(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(`
[[Repo]]
Origin = "https://example.com/repo.git"
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := parseConfig(cfgPath)
	if err != nil {
		t.Fatalf("parseConfig failed: %s", err)
	}

	if strings.Contains(buf.String(), "BitmapIndexInterval") {
		t.Fatal("unexpected deprecation warning when BitmapIndexInterval is not set")
	}
}
