# Quickstart step 1: Install dependencies

> NOTE: Please see install instructions for [macOS](#macos) and [Ubuntu](#ubuntu) in succeeding sections.

Sourcegraph has the following dependencies:
- [Git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git) (v2.18 or higher)
- [Go](https://golang.org/doc/install) (v1.14 or higher)
- [Node JS](https://nodejs.org/en/download/) (see current recommended version in [.nvmrc](https://github.com/sourcegraph/sourcegraph/blob/main/.nvmrc))
- [make](https://www.gnu.org/software/make/)
- [Docker](https://docs.docker.com/engine/installation/) (v18 or higher)
  - For macOS we recommend using Docker for Mac instead of `docker-machine`
- [PostgreSQL](https://wiki.postgresql.org/wiki/Detailed_installation_guides) (v11 or higher)
- [Redis](http://redis.io/) (v5.0.7 or higher)
- [Yarn](https://yarnpkg.com) (v1.10.1 or higher)
- [SQLite](https://www.sqlite.org/index.html) tools
- [Golang Migrate](https://github.com/golang-migrate/migrate/) (v4.7.0 or higher)
- [Comby](https://github.com/comby-tools/comby/) (v0.11.3 or higher)
- [Watchman](https://facebook.github.io/watchman/)

The following are two recommendations for installing these dependencies:

## macOS

1.  Install [Homebrew](https://brew.sh).
2.  Install [Docker for Mac](https://docs.docker.com/docker-for-mac/).

    optionally via `brew`

    ```
    brew install --cask docker
    ```

3.  Install Go, Node Version Manager, PostgreSQL, Redis, Git, golang-migrate, Comby, SQLite tools, and jq with the following command:

    ```
    brew install go yarn redis postgresql git gnu-sed golang-migrate comby sqlite pcre FiloSottile/musl-cross/musl-cross jq watchman
    ```

4.  Install the Node Version Manager (`nvm`) using:

    ```
    NVM_VERSION="$(curl https://api.github.com/repos/nvm-sh/nvm/releases/latest | jq -r .name)"
    curl -L https://raw.githubusercontent.com/nvm-sh/nvm/"$NVM_VERSION"/install.sh -o install-nvm.sh
    sh install-nvm.sh
    ```

    After the install script is finished, re-source your shell profile (e.g.,
    `source ~/.zshrc`) or restart your terminal session to pick up the `nvm`
    definitions. Re-running the install script will update the installation.

    Note: `nvm` is implemented as a shell function, so it may not show up in
    the output of `which nvm`. Use `type nvm` to verify whether it is set up.
    There is also a Homebrew package for `nvm`, but it is unsupported by the
    `nvm` maintainers.

    * For fish shell users, you will want to install `bass` which you can get via `omf`:

        ```
        curl -L https://get.oh-my.fish | fish
        omf install bass
        ```

    * Then add the following to your `config.fish`:

        ```
        function nvm
          bass source ~/.nvm/nvm.sh --no-use ';' nvm $argv
        end

        set -x NVM_DIR ~/.nvm
        ```

5.  Install the current recommended version of Node JS by running the following
    from the working directory of a sourcegraph repository clone:

    ```
    nvm install
    nvm use --delete-prefix
    ```

    After doing this, `node -v` should show the same version mentioned in
    `.nvmrc` at the root of the sourcegraph repository.

    Note: Although there is a Homebrew package for Node, we advise using `nvm`
    instead, to ensure you get a Node version compatible with the current state
    of the sourcegraph repository.

6.  Configure PostgreSQL and Redis to start automatically

    ```
    brew services start postgresql
    brew services start redis
    ```

    (You can stop them later by calling `stop` instead of `start` above.)

7.  Ensure `psql`, the PostgreSQL command line client, is on your `$PATH`.
    Homebrew does not put it there by default. Homebrew gives you the command to run to insert `psql` in your path in the "Caveats" section of `brew info postgresql`. Alternatively, you can use the command below. It might need to be adjusted depending on your Homebrew prefix (`/usr/local` below) and shell (bash below).

    ```
    hash psql || { echo 'export PATH="/usr/local/opt/postgresql/bin:$PATH"' >> ~/.bash_profile }
    source ~/.bash_profile
    ```

8.  Open a new Terminal window to ensure `psql` is now on your `$PATH`.

## Ubuntu


1. Add package repositories:

    ```
    # Go
    sudo add-apt-repository ppa:longsleep/golang-backports

    # Docker
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
    sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"

    # Yarn
    curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | sudo apt-key add -
    echo "deb https://dl.yarnpkg.com/debian/ stable main" | sudo tee /etc/apt/sources.list.d/yarn.list
    ```

2. Update repositories:

    ```
    sudo apt-get update
    ```

3. Install dependencies:

    ```
    sudo apt install -y make git-all postgresql postgresql-contrib redis-server libpcre3-dev libsqlite3-dev pkg-config golang-go musl-tools docker-ce docker-ce-cli containerd.io yarn jq libnss3-tools

    # install golang-migrate (you must rename the extracted binary to `golang-migrate` and move the binary into your $PATH)
    curl -L https://github.com/golang-migrate/migrate/releases/download/v4.7.0/migrate.linux-amd64.tar.gz | tar xvz

    # install comby (you must rename the extracted binary to `comby` and move the binary into your $PATH)
    curl -L https://github.com/comby-tools/comby/releases/download/0.11.3/comby-0.11.3-x86_64-linux.tar.gz | tar xvz

    # install watchman (you must put the binary and shared libraries on your $PATH and $LD_LIBRARY_PATH)
    curl -LO https://github.com/facebook/watchman/releases/download/v2020.07.13.00/watchman-v2020.07.13.00-linux.zip
    unzip watchman-*-linux.zip
    sudo mkdir -p /usr/local/{bin,lib} /usr/local/var/run/watchman
    sudo cp bin/* /usr/local/bin
    sudo cp lib/* /usr/local/lib
    sudo chmod 755 /usr/local/bin/watchman
    sudo chmod 2777 /usr/local/var/run/watchman
    # On Linux, you may need to run the following in addition:
    watchman watch <path to sourcegraph repository>

    # nvm (to manage Node.js)
    NVM_VERSION="$(curl https://api.github.com/repos/nvm-sh/nvm/releases/latest | jq -r .name)"
    curl -L https://raw.githubusercontent.com/nvm-sh/nvm/"$NVM_VERSION"/install.sh -o install-nvm.sh
    sh install-nvm.sh

    # in repo dir: install current recommendend version of Node JS
    nvm install
    ```

4. Configure startup services

    ```
    sudo systemctl enable postgresql
    sudo systemctl enable redis-server.service
    ```

5. (optional) You can also run Redis using Docker

    In this case you should not enable the `redis-server.service` from the previous step.

    ```
    dockerd # if docker isn't already running
    docker run -p 6379:6379 -v $REDIS_DATA_DIR redis
    # $REDIS_DATA_DIR should be an absolute path to a folder where you intend to store Redis data
    ```

    You need to have Redis running when you start the dev server later on. If you have issues running Docker, try [adding your user to the docker group][dockerGroup], and/or [updating the socket file permissions][socketPermissions], or try running these commands under `sudo`.

    [dockerGroup]: https://stackoverflow.com/a/48957722
    [socketPermissions]: https://stackoverflow.com/a/51362528

## (optional) asdf

[asdf](https://github.com/asdf-vm/asdf) is a CLI tool that manages runtime versions for a number of different languages and tools. It can be likened to a language-agnostic version of [nvm](https://github.com/nvm-sh/nvm) or [pyenv](https://github.com/pyenv/pyenv).

We use asdf in buildkite to lock the versions of the tools that we use on a per-commit basis.

### Install

#### asdf binary

See the [installation instructions on the official asdf documentation](https://asdf-vm.com/#/core-manage-asdf?id=install).

#### Plugins

sourcegraph/sourcegraph uses the following plugins:

- [Go](https://github.com/kennyp/asdf-golang)

```bash
asdf plugin add golang
```

- [NodeJS](https://github.com/asdf-vm/asdf-nodejs)

```bash
asdf plugin add nodejs

# Import the Node.js release team's OpenPGP keys to main keyring
bash ~/.asdf/plugins/nodejs/bin/import-release-team-keyring

# Have asdf read .nvmrc for auto-switching between node version
## Add the following to $HOME/.asdfrc:
legacy_version_file = yes
```

- [Yarn](https://github.com/twuni/asdf-yarn)

```bash
asdf plugin add yarn
```

### Usage instructions

[asdf](https://github.com/asdf-vm/asdf) uses versions specified in [.tool-versions](https://github.com/sourcegraph/sourcegraph/blob/main/.tool-versions) whenever a command is run from one of `sourcegraph/sourcegraph`'s subdirectories.

You can install the all the versions specified in [.tool-versions](https://github.com/sourcegraph/sourcegraph/blob/main/.tool-versions) by running `asdf install`.

[< Previous](index.md) | [Next >](quickstart_2_initialize_database.md)
