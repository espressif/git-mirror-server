# AGENTS.md

## Overview

Git mirror server in Go. Clones repos as `--mirror`, serves them via `git http-backend` (CGI), and updates them in background goroutines. Three files: `main.go` (server), `config.go` (TOML config), `mirror.go` (git operations).

## Development

- Use red/green TDD: write failing tests first, then implement to make them pass
- `go test && go vet ./... && gofmt -s -l .`
- Keep `example-config.toml` in sync with config struct changes
- Old config fields must be silently ignored (TOML decoder drops unknown keys) — log a deprecation warning instead of erroring
- Update this AGENTS.md and README.md if you find information here outdated.

## Key Constraints

- All maintenance git operations (`commit-graph write`, `multi-pack-index write`) must be safe for concurrent HTTP readers — no deleting pack files. Never use `git repack -d` in the serving path.
- Background goroutines run one per repo with no parallelism within a repo, but `git http-backend` serves concurrent reads on the same repo directory simultaneously.

## Repository

This project is developed in https://github.com/espressif/git-mirror-server. Only create PRs to this repo.
