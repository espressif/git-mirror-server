package main

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"time"

	"github.com/getsentry/sentry-go"
)

var urlPattern = regexp.MustCompile(`https?://[^\s]+`)

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
// URLs in the error message are sanitized to strip credentials.
// No-op when Sentry is not initialized (DSN empty).
func captureError(err error, repoName, operation string) {
	if err == nil {
		return
	}
	sanitized := fmt.Errorf("%s", sanitizeURLs(err.Error()))
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("repo", repoName)
		scope.SetTag("operation", operation)
	})
	hub.CaptureException(sanitized)
}

// sanitizeURLs redacts userinfo (credentials/tokens) from any URLs in s.
func sanitizeURLs(s string) string {
	return urlPattern.ReplaceAllStringFunc(s, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		if u.User != nil {
			u.User = url.User("REDACTED")
		}
		return u.String()
	})
}
