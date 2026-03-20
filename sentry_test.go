package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

func TestInitSentryEmptyDSN(t *testing.T) {
	if err := initSentry(""); err != nil {
		t.Fatalf("initSentry with empty DSN should succeed: %s", err)
	}
}

func TestInitSentryInvalidDSN(t *testing.T) {
	if err := initSentry("not-a-valid-dsn"); err == nil {
		t.Fatal("initSentry with invalid DSN should return error")
	}
}

func TestCaptureErrorNilIsNoop(t *testing.T) {
	captureError(nil, "test-repo", "test-op")
}

func TestCaptureErrorSetsTags(t *testing.T) {
	transport := &TransportMock{}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:       "https://key@sentry.io/1",
		Transport: transport,
	}); err != nil {
		t.Fatal(err)
	}
	defer sentry.Init(sentry.ClientOptions{Dsn: ""})

	testErr := errors.New("clone failed")
	captureError(testErr, "my-repo", "clone")

	if len(transport.events) == 0 {
		t.Fatal("expected at least one event captured")
	}
	ev := transport.events[0]
	if ev.Tags["repo"] != "my-repo" {
		t.Errorf("expected repo tag 'my-repo', got %q", ev.Tags["repo"])
	}
	if ev.Tags["operation"] != "clone" {
		t.Errorf("expected operation tag 'clone', got %q", ev.Tags["operation"])
	}
}

func TestSentryDSNInConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(`
SentryDSN = "https://key@sentry.io/1"

[[Repo]]
Origin = "https://example.com/repo.git"
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := parseConfig(cfgPath)
	if err != nil {
		t.Fatalf("parseConfig failed: %s", err)
	}

	if cfg.SentryDSN != "https://key@sentry.io/1" {
		t.Errorf("expected SentryDSN to be parsed, got %q", cfg.SentryDSN)
	}
}

type TransportMock struct {
	events []*sentry.Event
}

func (t *TransportMock) Configure(sentry.ClientOptions)          {}
func (t *TransportMock) SendEvent(event *sentry.Event)           { t.events = append(t.events, event) }
func (t *TransportMock) Flush(_ time.Duration) bool              { return true }
func (t *TransportMock) FlushWithContext(_ context.Context) bool { return true }
func (t *TransportMock) Close()                                  {}
