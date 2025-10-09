# AGENTS.md

## Project Overview

This is a simple Git mirror server written in Go that creates and serves read-only mirrors of Git repositories. The server uses Git's smart HTTP protocol via `git http-backend` to provide efficient repository access.

### Key Features
- Creates read-only mirrors of Git repositories
- Automatically updates mirrors at configurable intervals
- Serves mirrors over HTTP using Git's smart protocol
- Supports both HTTPS and SSH origin repositories
- Docker support for easy deployment
- Counter-based multi-pack and bitmap index generation for improved performance
- Per-repository configuration for fetch intervals and index refresh frequencies

### Architecture
The application consists of three main Go files:
1. `main.go` - Entry point, HTTP server setup, and background update processes
2. `config.go` - Configuration parsing from TOML files
3. `mirror.go` - Git mirror operations (clone, update, bitmap generation)

## Contribution Guide for AI Agents

### Code Style
- Follow existing Go conventions and formatting
- Use lowercase for package-private functions (only capitalize when needed across packages)
- Error messages should be clear and include relevant context
- Logging should be informative but not excessive

### Testing
- Run `go test` to ensure existing tests pass
- Run `go vet` to check for code issues
- Run `gofmt` to ensure proper formatting
- Test Docker image build process

### Common Commands
- `go build` - Build the application
- `go run main.go config.toml` - Run with a config file
- `go test` - Run tests
- `go vet ./...` - Check for potential issues
- `gofmt -s -l .` - Check code formatting

### Git Workflow
1. Create a feature branch from main
2. Make focused changes for a single feature
3. Ensure code compiles and runs correctly
4. Update documentation as needed
5. Commit with clear, descriptive messages
6. Push and create a pull request

### Docker Image Management
- Docker images are published to espressif/git-mirror on DockerHub
- Images are tagged with version numbers (e.g., v1.2.3)
- Major.minor and major version tags are also created (e.g., v1.2, v1)
- The latest tag is updated only on official releases
- Pre-release versions (with suffixes) only get the exact version tag

### Configuration Changes
- Add new configuration options to the `config` struct in `config.go`
- Set appropriate defaults in the `parseConfig` function
- Update `example-config.toml` with documentation for new options
- Ensure backward compatibility with existing configurations

### Error Handling
- Always check and handle errors appropriately
- Log errors with sufficient context for debugging
- Don't ignore errors from important operations like bitmap index generation
- Use `fmt.Errorf` with context when returning errors

### Background Processes
- Mirror updates run in goroutines with configurable intervals
- Multi-pack index and bitmap index operations are counter-based, running after a specific number of fetches
- Each repo maintains its own fetch counter to trigger index refresh operations
- Multi-pack index refreshes are lightweight and run more frequently (default: 0, disabled)
- Bitmap index rebuilds involve full repacks and run less frequently (default: 0, disabled)
- Both operations can be disabled per-repo by setting their interval to 0
- Ensure proper synchronization when accessing shared resources (counters use mutexes)
- Use semaphores or other synchronization primitives when limiting concurrent operations

### Documentation Updates
- Update README.md when adding significant features
- Keep example configuration files up to date
- Document new command-line options or behaviors
