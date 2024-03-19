# Dependencies 

The Sourcegraph codebase requires a few dependencies to be installed in order to run locally. We recommend and support a default approach that is suited for everyone, regardless of their familiarity with the topic.

## Databases

### macOS

#### Using Homebrew (recommended!)

1. `brew install postgres redis`
1. `brew services start postgresql`
1. `brew services start redis`


#### Using packaged applications

Requirements: 

- The `github.com/sourcegraph/sourcegraph` repository cloned in a folder of your choice.

Instructions:

- Install _Postgres.app_:
  1. Open your browser and navigate to this page: [https://postgresapp.com](https://postgresapp.com)
  1. Follow the installation instructions.
- Install _Redis.app_:
  1. Open your browser and navigate to this page: [https://jpadilla.github.io/redisapp/](https://jpadilla.github.io/redisapp/)
  1. Click on the Download button and install the app in the archive.
  1. Open a terminal and type: `sudo mkdir -p /etc/paths.d && echo /Applications/Redis.app/Contents/Resources/Vendor/redis/bin | sudo tee /etc/paths.d/redisapp`

### Any OS

#### Docker 

- Assuming that `docker` is installed on your system, you can run `sg run redis-postgres`  to start the databases.

## Languages

### MacOs

#### Homebrew (recommended)

1. `brew install go yarn`
1. `brew install nodejs`

#### Using `asdf` for everything
 
Requirements: 

- The `github.com/sourcegraph/sourcegraph` repository cloned in a folder of your choice.
- [`Homebrew`](https://brew.shell) is installed.

Instructions:

1. Open a terminal and type: 
1. `brew install asdf`
1. `echo ' . /opt/homebrew/opt/asdf/libexec/asdf.sh' >> ~/.zshrc:`
1. `zsh`
1. `asdf version`
  - this should print something similar to `v0.8.1` (the numbers are not important) 
  - if you get `zsh: command not found: asdf` then something did not work.
  <!--- TODO replace this with `sg setup2 checks -->
1. `asdf plugin add golang`
1. `asdf plugin add yarn`
1. `asdf plugin add nodejs`
1. We now need to be in the sourcegraph repository folder
1. `cd WHERE_THE_SOURCEGRAPH_FOLDER_IS`
  - if you are not comfortable with the shell:
    1. Type `cd` in the terminal
    1. Drag and drop the folder containing Sourcegraph code from the Finder to the terminal window.
    1. Type Enter
1. `asdf install` 
1. `pushd ..`
1. `popd`
1. `go version`
  - this should print something similar to `go version go1.17.1 darwin/arm64`
  <!--- TODO replace this with `sg setup2 checks -->

### Any OS

#### Using `nvm` to install NodeJS

It's common for frontend developers to prefer using [`nvm`](https://github.com/nvm-sh/nvm) to manage `nodejs` versions.

1. `NVM_VERSION="$(curl https://api.github.com/repos/nvm-sh/nvm/releases/latest | jq -r .name)"`
1. `curl -L https://raw.githubusercontent.com/nvm-sh/nvm/"$NVM_VERSION"/install.sh -o /tmp/install-nvm.sh`
1. `sh /tmp/install-nvm.sh`
1. `export NVM_DIR="$HOME/.nvm"`
1. `[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"`
1. `cd WHERE_THE_SOURCEGRAPH_FOLDER_IS`
1. `nvm install`
1. `nvm use --delete-prefix`

