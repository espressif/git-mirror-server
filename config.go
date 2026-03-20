package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type duration struct {
	time.Duration
}

type config struct {
	ListenAddr               string
	Interval                 duration
	MultiPackIndexInterval   int
	BasePath                 string
	MaxConcurrentConnections int
	SentryDSN                string
	Repo                     []repo
}

type repo struct {
	Name                   string
	Origin                 string
	Interval               duration
	MultiPackIndexInterval int
}

func (d *duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

func parseConfig(filename string) (cfg config, repos map[string]repo, err error) {
	// Parse the raw TOML file.
	raw, err := os.ReadFile(filename)
	if err != nil {
		err = fmt.Errorf("unable to read config file %s, %s", filename, err)
		return
	}
	md, err := toml.Decode(string(raw), &cfg)
	if err != nil {
		err = fmt.Errorf("unable to load config %s, %s", filename, err)
		return
	}

	for _, key := range md.Undecoded() {
		if key[len(key)-1] == "BitmapIndexInterval" {
			log.Printf("warning: BitmapIndexInterval in %s is deprecated and has no effect; bitmap indexes are now managed via MultiPackIndexInterval using multi-pack-index", filename)
			break
		}
	}

	// Set defaults if required.
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.Interval.Duration == 0 {
		cfg.Interval.Duration = 15 * time.Minute
	}
	if cfg.MultiPackIndexInterval < 0 {
		cfg.MultiPackIndexInterval = 0
	}
	if cfg.BasePath == "" {
		cfg.BasePath = "."
	}
	if cfg.BasePath, err = filepath.Abs(cfg.BasePath); err != nil {
		err = fmt.Errorf("unable to get absolute path to base path, %s", err)
	}

	// Set a default max concurrent connections if not specified
	if cfg.MaxConcurrentConnections <= 0 {
		cfg.MaxConcurrentConnections = 32
	}

	// Fetch repos, injecting default values where needed.
	if len(cfg.Repo) == 0 {
		err = fmt.Errorf("no repos found in config %s, please define repos under [[repo]] sections", filename)
		return
	}
	repos = map[string]repo{}
	for i, r := range cfg.Repo {
		if r.Origin == "" {
			err = fmt.Errorf("origin required for repo %d in config %s", i+1, filename)
			return
		}

		// Generate a name if there isn't one already
		if r.Name == "" {
			if u, err := url.Parse(r.Origin); err == nil && u.Scheme != "" {
				r.Name = u.Host + u.Path
			} else {
				parts := strings.Split(r.Origin, "@")
				if l := len(parts); l > 0 {
					r.Name = strings.Replace(parts[l-1], ":", "/", -1)
				}
			}
		}
		if r.Name == "" {
			err = fmt.Errorf("could not generate name for Origin %s in config %s, please manually specify a Name", r.Origin, filename)
		}
		if _, ok := repos[r.Name]; ok {
			err = fmt.Errorf("multiple repos with name %s in config %s", r.Name, filename)
			return
		}

		if r.Interval.Duration == 0 {
			r.Interval.Duration = cfg.Interval.Duration
		}
		if r.MultiPackIndexInterval < 0 {
			r.MultiPackIndexInterval = cfg.MultiPackIndexInterval
		}
		repos[r.Name] = r
	}
	return
}
