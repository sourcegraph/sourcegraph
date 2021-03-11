#!/usr/bin/env bash

set -euf -o pipefail

bash_error="Please upgrade bash to version 4. Currently on ${BASH_VERSION}."

if [[ ${BASH_VERSION:0:1} -lt 4 ]]; then
  case ${OSTYPE} in
    darwin)
      echo "${bash_error}"
      echo
      echo "  brew install bash"
      exit 1
      ;;
    linux-gnu)
      echo "${bash_error}"
      echo
      echo "  Use your OS package manager to upgrade."
      echo "  eg: apt-get install --only-upgrade bash OR yum -y update bash"
      exit 1
      ;;
  esac
fi

unset CDPATH
cd "$(dirname "${BASH_SOURCE[0]}")/.." # cd to repo root dir

if [ -f .env ]; then
  set -o allexport
  # shellcheck disable=SC1091
  source .env
  set +o allexport
fi

# Verify postgresql config.
hash psql 2>/dev/null || {
  # "brew install postgresql@9.6" does not put psql on the $PATH by default;
  # try to fix this automatically if we can.
  hash brew 2>/dev/null && {
    if [[ -x "$(brew --prefix)/opt/postgresql@9.6/bin/psql" ]]; then
      PATH="$(brew --prefix)/opt/postgresql@9.6/bin:$PATH"
      export PATH
    fi
  }
}
if ! psql -wc '\x' >/dev/null; then
  echo "FAIL: postgreSQL config invalid or missing OR postgreSQL is still starting up."
  echo "You probably need, at least, PGUSER and PGPASSWORD set in the environment."
  exit 1
fi

export PGSSLMODE=disable

# Target single database during development
export CODEINTEL_PGPORT="${PGPORT:-}"
export CODEINTEL_PGHOST="${PGHOST:-}"
export CODEINTEL_PGUSER="${PGUSER:-}"
export CODEINTEL_PGPASSWORD="${PGPASSWORD:-}"
export CODEINTEL_PGDATABASE="${PGDATABASE:-}"
export CODEINTEL_PGSSLMODE="${PGSSLMODE:-}"
export CODEINTEL_PGDATASOURCE="${PGDATASOURCE:-}"
export CODEINTEL_PG_ALLOW_SINGLE_DB=true

# Code Insights uses a separate database, because it's easier to run TimescaleDB in
# Docker than install as a Postgres extension in dev environments.
export CODEINSIGHTS_PGDATASOURCE=postgres://postgres:password@127.0.0.1:5435/postgres
export DB_STARTUP_TIMEOUT=120s # codeinsights-db needs more time to start in some instances.

# Default to "info" level debugging, and "condensed" log format (nice for human readers)
export SRC_LOG_LEVEL=${SRC_LOG_LEVEL:-info}
export SRC_LOG_FORMAT=${SRC_LOG_FORMAT:-condensed}
export GITHUB_BASE_URL=${GITHUB_BASE_URL:-http://127.0.0.1:3180}
export SRC_REPOS_DIR=$HOME/.sourcegraph/repos
export INSECURE_DEV=1
# In dev we only expect to have one gitserver instance
export SRC_GIT_SERVER_1=127.0.0.1:3178
export SRC_GIT_SERVERS=$SRC_GIT_SERVER_1
export GOLANGSERVER_SRC_GIT_SERVERS=host.docker.internal:3178
export SEARCHER_URL=http://127.0.0.1:3181
export REPO_UPDATER_URL=http://127.0.0.1:3182
export REDIS_ENDPOINT=127.0.0.1:6379
export QUERY_RUNNER_URL=http://localhost:3183
export SYMBOLS_URL=http://localhost:3184
export SRC_SYNTECT_SERVER=http://localhost:9238
export SRC_FRONTEND_INTERNAL=localhost:3090
export SRC_PROF_HTTP=

SRC_PROF_SERVICES=$(cat dev/src-prof-services.json)
export SRC_PROF_SERVICES

export OVERRIDE_AUTH_SECRET=sSsNGlI8fBDftBz0LDQNXEnP6lrWdt9g0fK6hoFvGQ
export DEPLOY_TYPE=dev
export CTAGS_COMMAND="${CTAGS_COMMAND:=cmd/symbols/universal-ctags-dev}"
export CTAGS_PROCESSES=2
export ZOEKT_HOST=localhost:3070
export USE_ENHANCED_LANGUAGE_DETECTION=${USE_ENHANCED_LANGUAGE_DETECTION:-1}
export GRAFANA_SERVER_URL=http://localhost:3370
export PROMETHEUS_URL="${PROMETHEUS_URL:-"http://localhost:9090"}"

# Jaeger config to get UI to work with reverse proxy, see https://www.jaegertracing.io/docs/1.11/deployment/#ui-base-path
export JAEGER_SERVER_URL=http://localhost:16686

# Caddy / HTTPS configuration
export SOURCEGRAPH_HTTPS_DOMAIN="${SOURCEGRAPH_HTTPS_DOMAIN:-"sourcegraph.test"}"
export SOURCEGRAPH_HTTPS_PORT="${SOURCEGRAPH_HTTPS_PORT:-"3443"}"

# Enable sharded indexed search mode
[ -n "${DISABLE_SEARCH_SHARDING-}" ] || export INDEXED_SEARCH_SERVERS="localhost:3070 localhost:3071"

# webpack-dev-server is a proxy running on port 3080 that (1) serves assets, waiting to respond
# until they are (re)built and (2) otherwise proxies to Caddy running on port 3081 (which proxies to
# Sourcegraph running on port 3082). That is why Sourcegraph listens on 3082 despite the externalURL
# having port 3080.
export SRC_HTTP_ADDR=":3082"
export WEBPACK_DEV_SERVER=1

if [ -z "${DEV_NO_CONFIG-}" ]; then
  export SITE_CONFIG_FILE=${SITE_CONFIG_FILE:-./dev/site-config.json}
  export GLOBAL_SETTINGS_FILE=${GLOBAL_SETTINGS_FILE:-./dev/global-settings.json}
  export SITE_CONFIG_ALLOW_EDITS=true
  export GLOBAL_SETTINGS_ALLOW_EDITS=true
fi

# WebApp
export NODE_ENV=development
export NODE_OPTIONS="--max_old_space_size=4096"

# Ensure SQLite for symbols is built
./dev/libsqlite3-pcre/build.sh
LIBSQLITE3_PCRE="$(./dev/libsqlite3-pcre/build.sh libpath)"
export LIBSQLITE3_PCRE

# Increase ulimit (not needed on Windows/WSL)
# shellcheck disable=SC2015
type ulimit >/dev/null && ulimit -n 10000 || true

# Check Go version and install Go tooling
export GO111MODULE=on
go run ./internal/version/minversion

# Install necessary Go tools
mkdir -p .bin
export GOBIN="${PWD}/.bin"
export GO111MODULE=on

INSTALL_GO_TOOLS=(
  "github.com/mattn/goreman@v0.3.4"
)

# Need to go to a temp directory for tools or we update our go.mod. We use
# GOPROXY=direct to avoid always consulting a proxy for dlv.
RANDOMT_TEMP=$(mktemp -d)
cp .tool-versions "${RANDOMT_TEMP}"
pushd "${RANDOMT_TEMP}" >/dev/null || exit 1
if ! GOPROXY=direct go get -v "${INSTALL_GO_TOOLS[@]}" 2>go-install.log; then
  cat go-install.log
  echo >&2 "failed to install prerequisite tools, aborting."
  exit 1
fi
rm -rf "${RANDOMT_TEMP}"
popd >/dev/null || exit 1

# Put .bin:node_modules/.bin onto the $PATH
export PATH="$PWD/.bin:$PWD/node_modules/.bin:$PATH"

# Now we create a temporary Procfile that includes all our build processes
tmp_install_procfile=$(mktemp -t procfile_install_XXXXXXX)

cat >"${tmp_install_procfile}" <<EOF
yarn: cd $(pwd) && yarn --no-progress
go-install: cd $(pwd) && ./dev/go-install.sh
ctags-image: cd $(pwd) && ./cmd/symbols/build-ctags.sh
EOF

# Kick off all build processes in parallel
goreman --set-ports=false --exit-on-error -f "${tmp_install_procfile}" start

# Once we've built the Go code and the frontend code, we build the frontend
# code once in the background to make sure editor codeintel works.
# This is fast if no changes were made.
# Don't fail if it errors as this is only for codeintel, not for the build.
trap 'kill $build_ts_pid; exit' EXIT
(yarn --silent run build-ts || true) &
build_ts_pid="$!"

# Now launch the services in $PROCFILE
export PROCFILE=${PROCFILE:-dev/Procfile}

only="${SRC_DEV_ONLY:-}"
except="${SRC_DEV_EXCEPT:-}"
while [[ "$#" -gt 0 ]]; do
  case $1 in
    -e | --except)
      except="$2"
      shift
      ;;
    -o | --only)
      only="$2"
      shift
      ;;
    *)
      echo "Unknown parameter passed: $1"
      exit 1
      ;;
  esac
  shift
done

if [ -n "${only}" ] || [ -n "${except}" ]; then
  services=${only:-$except}

  # "frontend,grafana,gitserver" -> "^(frontend|grafana|gitserver):"
  services_pattern="^(${services//,/|}):"

  if [ -n "${except}" ]; then
    grep_args="-vE"
  else
    grep_args="-E"
  fi

  tmp_procfile=$(mktemp -t procfile_XXXXXXX)
  grep ${grep_args} "${services_pattern}" "${PROCFILE}" >"${tmp_procfile}"
  export PROCFILE=${tmp_procfile}
fi

if [ -n "${only}" ]; then
  printf >&2 "\n--- Starting binaries %s...\n\n" "${only}"
elif [ -n "${except}" ]; then
  printf >&2 "\n--- Starting all binaries, except %s...\n\n" "${except}"
else
  printf >&2 "\n--- Starting all binaries...\n\n"
fi

export GOREMAN="goreman --set-ports=false --exit-on-error -f ${PROCFILE}"

if ! [ "$(id -u)" = 0 ] && command -v authbind; then
  # ignoring because $GOREMAN is used in other handle-change.sh
  # shellcheck disable=SC2086
  # Support using authbind to bind to port 443 as non-root
  exec authbind --deep $GOREMAN start
else
  # ignoring because $GOREMAN is used in other handle-change.sh
  # shellcheck disable=SC2086
  exec $GOREMAN start
fi
