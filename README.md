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

By default it will update the mirror every **1 minute** and will serve the mirror over HTTP using port **8080**. You can specify as many repos as you want by having multiple `[[repo]]` sections.

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

## Advanced configuration

See [the example config](example-config.toml) for more advanced configurations.

## Authentication and authorization

If you wish to control access to the mirror or specific repositories, consider proxying to `git-mirror` using a web server such as Nginx.
