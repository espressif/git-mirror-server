package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("please specify the path to a config file, an example config is available at https://github.com/espressif/git-mirror-server/blob/master/example-config.toml")
	}
	cfg, repos, err := parseConfig(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		log.Fatalf("failed to create %s, %s", cfg.BasePath, err)
	}

	if err := initSentry(cfg.SentryDSN); err != nil {
		log.Fatalf("sentry init: %s", err)
	}
	defer flushSentry()

	healthCheck(cfg, repos)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM, syscall.SIGINT)

	var wg sync.WaitGroup
	for _, r := range repos {
		wg.Add(1)
		go func(r repo) {
			defer wg.Done()
			timer := time.NewTimer(0)
			defer timer.Stop()
			// Drain the initial fire — first iteration runs immediately via the loop body
			if !timer.Stop() {
				<-timer.C
			}
			for {
				log.Printf("updating %s", r.Name)
				out, err := mirror(ctx, cfg, r)
				scanner := bufio.NewScanner(strings.NewReader(out))
				for scanner.Scan() {
					log.Printf("%s: %s", r.Name, scanner.Text())
				}
				if err != nil {
					log.Printf("error updating %s, %s", r.Name, err)
					captureError(err, r.Name, "mirror")
				} else {
					log.Printf("updated %s", r.Name)
				}
				timer.Reset(r.Interval.Duration)
				select {
				case <-ctx.Done():
					log.Printf("stopping updates for %s", r.Name)
					return
				case <-timer.C:
				}
			}
		}(r)
	}

	gitBackend := &cgi.Handler{
		Path: "/usr/bin/git",
		Args: []string{"http-backend"},
		Dir:  cfg.BasePath,
		Env: []string{
			"GIT_PROJECT_ROOT=" + cfg.BasePath,
			"GIT_HTTP_EXPORT_ALL=true",
		},
	}

	semaphore := make(chan struct{}, cfg.MaxConcurrentConnections)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		select {
		case semaphore <- struct{}{}:
		case <-r.Context().Done():
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		defer func() { <-semaphore }()
		gitBackend.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	go func() {
		sig := <-shutdownChan
		log.Printf("received %s, shutting down", sig)
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %s, forcing close", err)
			srv.Close()
		}
	}()

	log.Printf("starting git HTTP server on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("failed to start server, %s", err)
	}

	wg.Wait()
	log.Printf("shutdown complete")
}
