# git-mirror-server - simple Git mirrors

`git-mirror` is designed to create and serve read-only mirrors of your Git repositories locally or wherever you choose.
The server uses Git's smart HTTP protocol via `git http-backend` to provide efficient repository access.


A major design goal of `git-mirror` is that it should just work with as little configuration as possible.

## Get started

Download and extract the latest release from the [releases page](https://github.com/espressif/git-mirror-server/releases).

Create `config.toml` similar to:

```toml
[[repo]]
Origin = "https://github.com/espressif/git-mirror-server.git"
```

By default it will update the mirror every **15 minutes** and will serve the mirror over HTTP using port **8080**. You can specify as many repos as you want by having multiple `[[repo]]` sections.

Run `git-mirror` with the path to the config file:

```bash
$ ./git-mirror config.toml
2015/05/07 11:08:06 starting web server on :8080
2015/05/07 11:08:06 updating github.com/espressif/git-mirror-server.git
2015/05/07 11:08:08 updated github.com/espressif/git-mirror-server.git
```

Now you can clone from your mirror on the default port of `8080`:

```bash
$ git clone http://localhost:8080/github.com/espressif/git-mirror-server.git
Cloning into 'git-mirror-server'...
Checking connectivity... done.
```

## Using with docker

The docker image is available on Docker Hub [espressif/git-mirror](https://hub.docker.com/r/espressif/git-mirror)


You can run it with docker:

```
docker run --rm -ti -v /your_config/path:/config espressif/git-mirror /config/config.toml
```

Or with docker compose, for example:

```
services:
  git-mirror:
    image: espressif/git-mirror
    ports:
      - "8080:8080"
    command: ["/etc/git-mirror/config.toml"]
    volumes:
      - /opt/git-mirror/data:/git-mirror
      - /opt/git-mirror/config:/etc/git-mirror
    restart: always
```

## Configuration Options

### Global Settings

- **`ListenAddr`** (string, default: `:8080`) - The address and port the web server listens on for serving mirrors
- **`Interval`** (duration, default: `15m`) - Default interval for updating mirrors; can be overridden per repository
- **`BasePath`** (string, default: `.`) - Base path for storing mirror data, can be absolute or relative
- **`MaxConcurrentConnections`** (int, default: `32`) - Limits the number of concurrent HTTP connections to prevent overload
- **`MultiPackIndexInterval`** (int, default: `0`) - Number of fetches after which to refresh the multi-pack index; Disabled by default (`0`)
- **`BitmapIndexInterval`** (int, default: `0`) - Number of fetches after which to rebuild the bitmap index with full repack; Disabled by default (`0`)
- **`SentryDSN`** (string, default: empty) - Sentry DSN for error reporting. When set, clone/fetch failures, health-check failures, and maintenance errors are reported to Sentry with repo and operation tags

### Repository Settings

Each `[[Repo]]` section supports:

- **`Origin`** (string, required) - The URL of the repository to mirror (supports HTTPS and SSH)
- **`Name`** (string, optional) - Custom name for accessing the mirror; auto-generated from Origin if not specified
- **`Interval`** (duration, optional) - Override the global update interval for this specific repository
- **`MultiPackIndexInterval`** (int, optional) - Override the global multi-pack index refresh interval for this repository
- **`BitmapIndexInterval`** (int, optional) - Override the global bitmap index rebuild interval for this repository

### Performance Optimization

The server uses two index refresh strategies:

1. **Multi-pack index** - Lightweight operation that improves fetch performance without repacking. Runs more frequently (default: 0, disabled).
2. **Bitmap index** - Full repack with bitmap generation for maximum performance. More resource-intensive, runs less frequently (default: 0, disabled).

Both can be disabled per repository by setting their interval to `0`, or customized based on repository size and usage patterns.

### Example Configuration

See [the example config](example-config.toml) for a complete configuration example.

## Authentication and authorization

If you wish to control access to the mirror or specific repositories, consider proxying to `git-mirror` using a web server such as Nginx.
