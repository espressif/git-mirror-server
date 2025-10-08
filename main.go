package main

import (
	"bufio"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"strings"
	"time"
)

func main() {
	// Parse config.
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

	// Run background threads to keep mirrors up to date.
	for _, r := range repos {
		go func(r repo) {
			for {
				log.Printf("updating %s", r.Name)
				out, err := mirror(cfg, r)
				scanner := bufio.NewScanner(strings.NewReader(out))
				for scanner.Scan() {
					log.Printf("%s: %s", r.Name, scanner.Text())
				}
				if err != nil {
					log.Printf("error updating %s, %s", r.Name, err)
				} else {
					log.Printf("updated %s", r.Name)
				}
				time.Sleep(r.Interval.Duration)
			}
		}(r)
	}

	// Run full repack with bitmap generation once in a while
	go func() {
		for {
			time.Sleep(cfg.BitmapInterval.Duration)
			for _, r := range repos {
				log.Printf("updating bitmap for %s", r.Name)
				if err := refreshBitmapIndex(cfg, r); err != nil {
					log.Printf("error updating bitmap for %s: %s", r.Name, err)
				} else {
					log.Printf("bitmap updated for %s", r.Name)
				}
			}
		}
	}()

	// Set up git http-backend CGI handler
	gitBackend := &cgi.Handler{
		Path: "/usr/bin/git",
		Args: []string{"http-backend"},
		Dir:  cfg.BasePath,
		Env: []string{
			"GIT_PROJECT_ROOT=" + cfg.BasePath,
			"GIT_HTTP_EXPORT_ALL=true",
		},
	}

	// Create a semaphore to limit concurrent connections
	semaphore := make(chan struct{}, cfg.MaxConcurrentConnections)

	// Wrap the handler with concurrent connection limiting
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Acquire semaphore
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		gitBackend.ServeHTTP(w, r)
	})

	log.Printf("starting git HTTP server on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, nil); err != nil {
		log.Fatalf("failed to start server, %s", err)
	}
}
