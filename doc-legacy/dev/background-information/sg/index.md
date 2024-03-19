# `sg` - the Sourcegraph developer tool

```none
          _____                    _____
         /\    \                  /\    \
        /::\    \                /::\    \
       /::::\    \              /::::\    \
      /::::::\    \            /::::::\    \
     /:::/\:::\    \          /:::/\:::\    \
    /:::/__\:::\    \        /:::/  \:::\    \
    \:::\   \:::\    \      /:::/    \:::\    \
  ___\:::\   \:::\    \    /:::/    / \:::\    \
 /\   \:::\   \:::\    \  /:::/    /   \:::\ ___\
/::\   \:::\   \:::\____\/:::/____/  ___\:::|    |
\:::\   \:::\   \::/    /\:::\    \ /\  /:::|____|
 \:::\   \:::\   \/____/  \:::\    /::\ \::/    /
  \:::\   \:::\    \       \:::\   \:::\ \/____/
   \:::\   \:::\____\       \:::\   \:::\____\
    \:::\  /:::/    /        \:::\  /:::/    /
     \:::\/:::/    /          \:::\/:::/    /
      \::::::/    /            \::::::/    /
       \::::/    /              \::::/    /
        \::/    /                \::/____/
         \/____/

```

[`sg`](https://github.com/sourcegraph/sourcegraph/tree/main/dev/sg) is the CLI tool that Sourcegraph developers can use to develop Sourcegraph.
Learn more about the tool's overall vision in [`sg` Vision](./vision.md), and how to use it in the [usage section](#usage).

> NOTE: Have feedback or ideas? Feel free to [open a discussion](https://github.com/sourcegraph/sourcegraph/discussions/categories/developer-experience)! Sourcegraph teammates can also leave a message in [#dev-experience](https://sourcegraph.slack.com/archives/C01N83PS4TU).

## Quickstart

1. Run the following to download and install `sg`:

   ```sh
   curl --proto '=https' --tlsv1.2 -sSLf https://install.sg.dev | sh
   ```

2. In your clone of [`sourcegraph/sourcegraph`](https://github.com/sourcegraph/sourcegraph), start the default Sourcegraph environment:

   ```sh
   sg start
   ```

3. Once the `enterprise-web` process has finished compilation, open [`https://sourcegraph.test:3443`](https://sourcegraph.test:3443/) in your browser.

A more detailed introduction is available in the [development quickstart guide](../../setup/quickstart.md).

## Installation

Run the following command in a terminal:

```sh
curl --proto '=https' --tlsv1.2 -sSLf https://install.sg.dev | sh
```

That will download the latest release of `sg` from [here](https://github.com/sourcegraph/sg/releases), put it in a temporary location and run `sg install` to install it to a permanent location in your `$PATH`.

For other installation options, see [Advanced installation](#advanced-installation).

## Updates

Once set up, `sg` will automatically check for updates and update itself if a change is detected in your local copy of `origin/main`.
To force a manual update of `sg`, run:

```sh
sg update
```

In order to temporarily turn off automatic updates, run your commands with the `-skip-auto-update` flag or `SG_SKIP_AUTO_UPDATE` environment variable:

```sh
sg -skip-auto-update [cmds ...]
```

On the next command run, if a new version is detected, `sg` will auto update before running.

To see what's changed, use `sg version changelog`.

### Help

You can get help about commands locally in a variety of ways:

```sh
sg help # show all available commands

# learn about a specific command or subcommand
sg <command> -h
sg <command> --help

sg help -full # full reference
```

### Autocompletion

If you have used `sg setup`, you should have autocompletions set up for `sg`. To enable it, type out a partial command and press the <kbd>Tab</kbd> key twice. For example:

```none
sg start<tab><tab>
```

To get autocompletions for the available flags for a command, type out a command and `-` and press the <kbd>Tab</kbd> key twice. For example:

```none
sg start -<tab><tab>
```

Both of the above work if you provide partial values as well to narrow down the suggestions. For example, the following will suggest run sets that start with `web-`:

```none
sg start web-<tab><tab>
```

## Configuration

Default `sg` behaviour is configured through the [`sg.config.yaml` file in the root of the `sourcegraph/sourcegraph` repository](https://github.com/sourcegraph/sourcegraph/blob/main/sg.config.yaml). Take a look at that file to see which commands are run in which environment, how these commands set setup, what environment variables they use, and more.

**To modify your configuration locally, you can overwrite chunks of configuration by creating a `sg.config.overwrite.yaml` file in the root of the repository.** It's `.gitignore`d so you won't accidentally commit those changes.

If an `sg.config.overwrite.yaml` file exists, its contents will be merged with the content of `sg.config.yaml`, overwriting where there are conflicts. This is useful for running custom command sets or adding environment variables
specific to your work.

You can run `sg run debug-env` to see the environment variables passed `sg`'s child processes.

### Configuration examples

#### Changing database configuration

In order to change the default database configuration, the username and the database, for example, create an `sg.config.overwrite.yaml` file that looks like this:

```yaml
env:
  PGUSER: 'mrnugget'
  PGDATABASE: 'my-database'
```

That works for all the other `env` variables in `sg.config.yaml` too.

#### Defining a custom environment by setting a `commandset`

You can customize what boots up in your development environment by defining a `commandSet` in your `sg.config.overwrite.yaml`.

For example, the following defines a commandset called `minimal-batches` that boots up a minimal environment to work on Batch Changes:

```yaml
commandsets:
  minimal-batches:
    checks:
      - docker
      - redis
      - postgres
    commands:
      - enterprise-frontend
      - enterprise-worker
      - enterprise-repo-updater
      - enterprise-web
      - gitserver
      - searcher
      - symbols
      - caddy
      - zoekt-indexserver-0
      - zoekt-indexserver-1
      - zoekt-webserver-0
      - zoekt-webserver-1
      - batches-executor-firecracker
```

With that in `sg.config.overwrite.yaml` you can now run `sg start minimal-batches`.

#### Run `gitserver` in a Docker container

`sg start` runs many of the services (defined in the `commands` section of `sg.config.yaml`) as binaries that it compiles and runs according to the settings in their `cmd` and `install` sections. Sometimes while developing, you need to run some of the services isolated from your local environment. This example shows what to add to `sg.config.overwrite.yaml` so that `gitserver` will run in a Docker container. The `gitserver` service already has a build script that generates a Docker image; this configuration will use that script in the `install` section, and use the `env` defined in `sg.config.yaml` to pass environment variables to `docker` in the `run` section.

**A few things to note about this configuration**
- `PGHOST` is set to `host.docker.internal` so that `gitserver` running in the container can connect to the database that's running on your local machine. See [the Docker documentation](https://docs.docker.com/desktop/networking/#i-want-to-connect-from-a-container-to-a-service-on-the-host) for more information about `host.docker.internal`. In order to use `host.docker.internal` here, you will need to add it to `/etc/hosts` so that the services not running in Docker containers will be able to use it also. If you are using a different database, you will need to adjust `PGHOST` to suit.
-  `${SRC_FRONTEND_INTERNAL##*:}`, `${HOSTNAME##*:}`, and `${SRC_PROF_HTTP##*:}` use shell parameter expansion to pull the port number from the environment variables defined in `sg.config.yaml`. This parameter expansion works in at least the `ksh`, `bash` and `zsh` shells; might not work in others.
- The `gitserver` Docker containers will be left running after `sg` has terminated. You will need to manually stop them.
- The Prometheus agent gets metrics about the `/data/repos` mount. Because the Docker operating system is Linux, the metric function uses the `sysfs` pseudo filesystem and assumes that `/data/repos` is on a block device.  However, Docker bind mounts (which is what `-v ${SRC_REPOS_DIR}:/data/repos` creates) create virtual filesystems, not block filesystems. This will result in error messages in the container about "skipping metric registration" with a reason containing "failed to evaluate sysfs symlink". You can ignore those errors unless your development work involves the Prometheus metrics, in which case you will need to create Docker volumes and mount those instead of the bind mounts (Docker volumes are mounted as block devices). Using a Docker volume means that the repo dirs will not be on your local filesystem in the same location as `SRC_REPOS_DIR`.

```yaml
env:
  # MUST ADD ENTRY TO /etc/hosts: 127.0.0.1 host.docker.internal
  PGHOST: host.docker.internal
commands:
  gitserver:
    install: |
      config=""
      [[ $(uname) == "Darwin" ]] && config="--config darwin-docker"
      bazel build //cmd/gitserver:image_tarball ${config} && \
      docker load --input $(bazel cquery //cmd/gitserver:image_tarball ${config} --output=files)
  gitserver-0:
    cmd: |
      docker inspect gitserver-${GITSERVER_INDEX} >/dev/null 2>&1 && docker stop gitserver-${GITSERVER_INDEX}
      docker run \
      --rm \
      -e "GITSERVER_EXTERNAL_ADDR=${GITSERVER_EXTERNAL_ADDR}" \
      -e "GITSERVER_ADDR=0.0.0.0:${HOSTNAME##*:}" \
      -e "SRC_FRONTEND_INTERNAL=host.docker.internal:${SRC_FRONTEND_INTERNAL##*:}" \
      -e "SRC_PROF_HTTP=0.0.0.0:${SRC_PROF_HTTP##*:}" \
      -e "HOSTNAME=${HOSTNAME}" \
      -p ${GITSERVER_ADDR}:${HOSTNAME##*:} \
      -p ${SRC_PROF_HTTP}:${SRC_PROF_HTTP##*:} \
      -v ${SRC_REPOS_DIR}:/data/repos \
      --detach \
      --name gitserver-${GITSERVER_INDEX} \
      gitserver:candidate
    env:
      GITSERVER_INDEX: 0
  gitserver-1:
    cmd: |
      docker inspect gitserver-${GITSERVER_INDEX} >/dev/null 2>&1 && docker stop gitserver-${GITSERVER_INDEX}
      docker run \
      --rm \
      -e "GITSERVER_EXTERNAL_ADDR=${GITSERVER_EXTERNAL_ADDR}" \
      -e "GITSERVER_ADDR=0.0.0.0:${HOSTNAME##*:}" \
      -e "SRC_FRONTEND_INTERNAL=host.docker.internal:${SRC_FRONTEND_INTERNAL##*:}" \
      -e "SRC_PROF_HTTP=0.0.0.0:${SRC_PROF_HTTP##*:}" \
      -e "HOSTNAME=${HOSTNAME}" \
      -p ${GITSERVER_ADDR}:${HOSTNAME##*:} \
      -p ${SRC_PROF_HTTP}:${SRC_PROF_HTTP##*:} \
      -v ${SRC_REPOS_DIR}:/data/repos \
      --detach \
      --name gitserver-${GITSERVER_INDEX} \
      gitserver:candidate
    env:
      GITSERVER_INDEX: 1
```

### Attach a debugger

To attach the [Delve](https://github.com/go-delve/delve) debugger, pass the environment variable `DELVE=true` into `sg`. [Read more here](https://docs.sourcegraph.com/dev/how-to/debug_live_code#debug-go-code)

### Offline development

Sometimes you will want to develop Sourcegraph but it just so happens you will be on a plane or a
train or perhaps a beach, and you will have no WiFi. And you may raise your fist toward heaven and
say something like, "Why, we can put a man on the moon, so why can't we develop high-quality code
search without an Internet connection?" But lower your hand back to your keyboard and fret no
further, you *can* develop Sourcegraph with no connectivity by setting the
`OFFLINE` environment variable:

```bash
OFFLINE=true sg start
```

Ensure that the `sourcegraph/syntax-highlighter:insiders` image is already available locally. If not, pull it with the following command before going offline to ensure that offline mode works seamlessly:

```bash
docker pull -q sourcegraph/syntax-highlighter:insiders
```

## `sg` and pre-commit hooks

When `sg setup` is run, it will automatically install pre-commit hooks (using [pre-commit.com](https://pre-commit.com)), with a [provided configuration](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/.pre-commit-config.yaml) that will perform a series of fast checks before each commit you create locally.

Amongst that list of checks, is a [script](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/check-tokens.sh) that tries to detect the presence of tokens that would have been accidentally committed. While it's implementation is rather simple and won't catch all tokens (this is covered by automated scans in CI), it's enough to catch common mistakes and save you from having to rotate secrets, as they never left your computer. Due to the importance of such a measure, it's an opt-out process instead of opt-in.

Therefore, it's strongly recommended to keep the pre-commit git hook. In the eventuality of the pre-commit detecting a false positive, you can disable it through `sg setup disable-pre-commit` and prevent `sg setup` from installing it by passing a flag `sg setup --skip-pre-commit`.

### Exceptions

There are legitimate cases where code contains what appears to be a Sourcegraph token but isn't usable on any existing deployments.
Testing code for generating tokens is good example.

You can tell pre-commit to simply skip these files by adding a `// pre-commit:ignore_sourcegraph_token` top-level comment, as
shown in the example below:

```
package accesstoken

// pre-commit:ignore_sourcegraph_token

import (
	"testing"
)

func TestGenerateDotcomUserGatewayAccessToken(t *testing.T) {
	type args struct {
		apiToken string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "valid token 1",
			args:    args{apiToken: "0123456789abcdef0123456789abcdef01234567"},
			want:    "LOOKS_LIKE_A_REAL_TOKEN",
			wantErr: false,
		},
  // (...)
```

## Contributing to `sg`

Want to hack on `sg`? Great! Here's how:

1. Read through the [`sg` Vision](./vision.md) to get an idea of what `sg` should be in the long term.
2. Explore the [`sg` source code](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/dev/sg).
3. Look at the open [`sg` issues](https://github.com/sourcegraph/sourcegraph/issues?q=is%3Aopen+is%3Aissue+label%3Asg).

When you want to hack on `sg` it's best to be in the `dev/sg` directory and run it from there:

```sh
cd dev/sg
go run . -config ../../sg.config.yaml start
```

The `-config` can be anything you want, of course.

Have questions or need help? Feel free to [open a discussion](https://github.com/sourcegraph/sourcegraph/discussions/categories/developer-experience)! Sourcegraph teammates can also leave a message in [#dev-experience](https://sourcegraph.slack.com/archives/C01N83PS4TU).

> NOTE: For Sourcegraph teammates, we have a weekly [`sg` hack hour](https://handbook.sourcegraph.com/departments/product-engineering/engineering/enablement/dev-experience#sg-hack-hour) you can hop in to if you're interested in contributing!

## Advanced installation

### Dockerized sg

A `sourcegraph/sg` Docker image is available:

```dockerfile
# ...
COPY --from us.gcr.io/sourcegraph-dev/sg:insiders /usr/local/bin/sg ./sg
# ...
```

### Manually building the binary

> NOTE: **This method requires that Go has already been installed according to the [development quickstart guide](../../setup/quickstart.md).**

If you want full control over where the `sg` binary ends up, use this option.

In the root of `sourcegraph/sourcegraph`, run:

```sh
go build -o ~/my/path/sg ./dev/sg
```

Then make sure that `~/my/path` is in your `$PATH`.

> NOTE: **For Linux users:** A command called [sg](https://www.man7.org/linux/man-pages/man1/sg.1.html) is already available at `/usr/bin/sg`. To use the Sourcegraph `sg` CLI, you need to make sure that its location comes first in `PATH`. For example, by prepending `$GOPATH/bin`:
>
> `export PATH=$GOPATH/bin:$PATH`
>
> Instead of the more conventional:
>
> `export PATH=$PATH:$GOPATH/bin`
>
> Or you may add an alias to your `.bashrc`:
>
> `alias sg=$HOME/go/bin/sg`
