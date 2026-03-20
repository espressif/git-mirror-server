package main

import (
	"fmt"
	"log"
	"time"

	"github.com/getsentry/sentry-go"
)

func initSentry(dsn string) error {
	if dsn == "" {
		return nil
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		AttachStacktrace: true,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Sentry: %w", err)
	}
	log.Printf("Sentry error reporting enabled")
	return nil
}

func flushSentry() {
	sentry.Flush(5 * time.Second)
}

// captureError reports an error to Sentry with repo/operation context.
// No-op when Sentry is not initialized (DSN empty).
func captureError(err error, repoName, operation string) {
	if err == nil {
		return
	}
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("repo", repoName)
		scope.SetTag("operation", operation)
	})
	hub.CaptureException(err)
}
