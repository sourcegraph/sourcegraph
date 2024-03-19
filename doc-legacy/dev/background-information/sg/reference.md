<!-- DO NOT EDIT: generated via: go generate ./dev/sg -->

# sg reference

sg - The Sourcegraph developer tool!

Learn more: https://docs.sourcegraph.com/dev/background-information/sg

```sh
sg [GLOBAL FLAGS] command [COMMAND FLAGS] [ARGUMENTS...]
```

Global flags:

* `--config, -c="<value>"`: load sg configuration from `file` (default: sg.config.yaml)
* `--disable-analytics`: disable event logging (logged to '~/.sourcegraph/events')
* `--disable-output-detection`: use fixed output configuration instead of detecting terminal capabilities
* `--disable-overwrite`: disable loading additional sg configuration from overwrite file (see -overwrite)
* `--no-dev-private`: disable checking for dev-private - only useful for automation or ci
* `--overwrite, -o="<value>"`: load sg configuration from `file` that is gitignored and can be used to, for example, add credentials (default: sg.config.overwrite.yaml)
* `--skip-auto-update`: prevent sg from automatically updating itself
* `--verbose, -v`: toggle verbose mode


## sg start

🌟 Starts the given commandset. Without a commandset it starts the default Sourcegraph dev environment.

Use this to start your Sourcegraph environment!

Available commandsets in `sg.config.yaml`:

* api-only
* app
* batches 🦡
* batches-kubernetes
* codeintel
* codeintel-bazel
* codeintel-kubernetes
* cody-gateway
* dotcom
* enterprise
* enterprise-bazel
* enterprise-bazel-sveltekit
* enterprise-codeinsights
* enterprise-codeintel 🧠
* enterprise-codeintel-bazel
* enterprise-codeintel-multi-queue-executor
* enterprise-e2e
* enterprise-sveltekit
* iam
* monitoring
* monitoring-alerts
* otel
* qdrant
* single-program
* web-standalone
* web-standalone-prod
* web-sveltekit-standalone

```sh
# Run default environment, Sourcegraph enterprise:
$ sg start

# List available environments (defined under 'commandSets' in 'sg.config.yaml'):
$ sg start -help

# Run the enterprise environment with code-intel enabled:
$ sg start enterprise-codeintel

# Run the environment for Batch Changes development:
$ sg start batches

# Override the logger levels for specific services
$ sg start --debug=gitserver --error=enterprise-worker,enterprise-frontend enterprise

# View configuration for a commandset
$ sg start -describe oss
```

Flags:

* `--crit, -c="<value>"`: Services to set at info crit level.
* `--debug, -d="<value>"`: Services to set at debug log level.
* `--describe`: Print details about the selected commandset
* `--error, -e="<value>"`: Services to set at info error level.
* `--except="<value>"`: List of services of the specified command set to NOT start
* `--info, -i="<value>"`: Services to set at info log level.
* `--only="<value>"`: List of services of the specified command set to start. Commands NOT in this list will NOT be started.
* `--warn, -w="<value>"`: Services to set at warn log level.

## sg run

Run the given commands.

Runs the given command. If given a whitespace-separated list of commands it runs the set of commands.

Available commands in `sg.config.yaml`:

* batches-executor
* batches-executor-firecracker
* batches-executor-kubernetes
* batcheshelper-builder
* bext
* blobstore
* caddy
* codeintel-executor
* codeintel-executor-firecracker
* codeintel-executor-kubernetes
* codeintel-worker
* cody-app: Cody App
* cody-gateway
* debug-env: Debug env vars
* docsite: Docsite instance serving the docs
* embeddings
* executor-kubernetes-template
* executor-template
* frontend: Frontend
* gitserver
* gitserver-0
* gitserver-1
* gitserver-template
* grafana
* jaeger
* loki
* monitoring-generator
* multiqueue-executor
* otel-collector: OpenTelemetry collector
* pings
* postgres_exporter
* prometheus
* qdrant
* redis-postgres: Dockerized version of redis and postgres
* repo-updater
* searcher
* server: Run an all-in-one sourcegraph/server image
* sourcegraph: Single-program distribution
* storybook
* symbols
* syntax-highlighter
* tauri: App shell (Tauri)
* telemetry-gateway
* web-integration-build-prod: Build production web application for integration tests
* web-integration-build: Build development web application for integration tests
* web-standalone-http-prod: Standalone web frontend (production) with API proxy to a configurable URL
* web-standalone-http: Standalone web frontend (dev) with API proxy to a configurable URL
* web-sveltekit-prod-watch: Builds the prod version of the SvelteKit web app and rebuilds on changes
* web-sveltekit-standalone: Standalone SvelteKit web frontend (dev) with API proxy to a configurable URL
* web: Enterprise version of the web app
* worker
* zoekt-index-0
* zoekt-index-1
* zoekt-indexserver-template
* zoekt-web-0
* zoekt-web-1
* zoekt-web-template

```sh
# Run specific commands
$ sg run gitserver
$ sg run frontend

# List available commands (defined under 'commands:' in 'sg.config.yaml')
$ sg run -help

# Run multiple commands
$ sg run gitserver frontend repo-updater

# View configuration for a command
$ sg run -describe jaeger
```

Flags:

* `--describe`: Print details about selected run target
* `--legacy`: Force run to pick the non-bazel variant of the command

## sg ci

Interact with Sourcegraph's Buildkite continuous integration pipelines.

Note that Sourcegraph's CI pipelines are under our enterprise license: https://github.com/sourcegraph/sourcegraph/blob/main/LICENSE.enterprise

```sh
# Preview what a CI run for your current changes will look like
$ sg ci preview

# Check on the status of your changes on the current branch in the Buildkite pipeline
$ sg ci status
# Check on the status of a specific branch instead
$ sg ci status --branch my-branch
# Block until the build has completed (it will send a system notification)
$ sg ci status --wait
# Get status for a specific build number
$ sg ci status --build 123456

# Pull logs of failed jobs to stdout
$ sg ci logs
# Push logs of most recent main failure to local Loki for analysis
# You can spin up a Loki instance with 'sg run loki grafana'
$ sg ci logs --branch main --out http://127.0.0.1:3100
# Get the logs for a specific build number, useful when debugging
$ sg ci logs --build 123456

# Manually trigger a build on the CI with the current branch
$ sg ci build
# Manually trigger a build on the CI on the current branch, but with a specific commit
$ sg ci build --commit my-commit
# Manually trigger a main-dry-run build of the HEAD commit on the current branch
$ sg ci build main-dry-run
$ sg ci build --force main-dry-run
# Manually trigger a main-dry-run build of a specified commit on the current ranch
$ sg ci build --force --commit my-commit main-dry-run
# View the available special build types
$ sg ci build --help
```

### sg ci preview

Preview the pipeline that would be run against the currently checked out branch.


Flags:

* `--branch, -b="<value>"`: Branch `name` of build to target (defaults to current branch)
* `--format="<value>"`: Output format for the preview (one of 'markdown', 'json', or 'yaml') (default: markdown)

### sg ci bazel

Fires a CI build running a given bazel command.

Arguments: `[--web|--wait] [test|build] <target1> <target2> ... <bazel flags>`

Flags:

* `--wait`: Wait until build completion and then print logs for the Bazel command
* `--web`: Print the web URL for the build and return immediately

### sg ci status

Get the status of the CI run associated with the currently checked out branch.


Flags:

* `--branch, -b="<value>"`: Branch `name` of build to target (defaults to current branch)
* `--build, -n="<value>"`: Override branch detection with a specific build `number`
* `--commit, -c="<value>"`: Override branch detection with the latest build for `commit`
* `--pipeline, -p="<value>"`: Select a custom Buildkite `pipeline` in the Sourcegraph org (default: sourcegraph)
* `--wait`: Wait by blocking until the build is finished
* `--web, --view, -w`: Open build page in web browser (--view is DEPRECATED and will be removed in the future)

### sg ci build

Manually request a build for the currently checked out commit and branch (e.g. to trigger builds on forks or with special run types).


Optionally provide a run type to build with.

This command is useful when:

- you want to trigger a build with a particular run type, such as 'main-dry-run'
- triggering builds for PRs from forks (such as those from external contributors), which do not trigger Buildkite builds automatically for security reasons (we do not want to run insecure code on our infrastructure by default!)

Supported run types when providing an argument for 'sg ci build [runtype]':

* _manually_triggered_external - Manually Triggered External Build
* bext-nightly - Manually triggered browser extension nightly release build
* main-dry-run - Main dry run
* docker-images-patch - Patch image
* docker-images-patch-notest - Patch image without testing
* executor-patch-notest - Build executor without testing
* docker-images-candidates-notest - Build all candidates without testing
* backend-integration - Backend integration tests
* bazel-do - Bazel command

For run types that require branch arguments, you will be prompted for an argument, or you
can provide it directly (for example, 'sg ci build [runtype] <argument>').

```sh
# Start a main-dry-run build
$ sg ci build main-dry-run

# Publish a custom image build
$ sg ci build docker-images-patch

# Publish a custom Prometheus image build without running tests
$ sg ci build docker-images-patch-notest prometheus

# Publish all images without testing
$ sg ci build docker-images-candidates-notest
```

Flags:

* `--commit, -c="<value>"`: `commit` from the current branch to build (defaults to current commit)
* `--pipeline, -p="<value>"`: Select a custom Buildkite `pipeline` in the Sourcegraph org (default: sourcegraph)

### sg ci logs

Get logs from CI builds (e.g. to grep locally).

Get logs from CI builds, and output them in stdout or push them to Loki. By default only gets failed jobs - to change this, use the '--state' flag.

The '--job' flag can be used to narrow down the logs returned - you can provide either the ID, or part of the name of the job you want to see logs for.

To send logs to a Loki instance, you can provide --out=http://127.0.0.1:3100 after spinning up an instance with 'sg run loki grafana'.
From there, you can start exploring logs with the Grafana explore panel.



Flags:

* `--branch, -b="<value>"`: Branch `name` of build to target (defaults to current branch)
* `--build, -n="<value>"`: Override branch detection with a specific build `number`
* `--commit, -c="<value>"`: Override branch detection with the latest build for `commit`
* `--job, -j="<value>"`: ID or name of the job to export logs for
* `--out, -o="<value>"`: Output `format`: one of [terminal|simple|json], or a URL pointing to a Loki instance, such as http://127.0.0.1:3100 (default: terminal)
* `--overwrite-state="<value>"`: `state` to overwrite the job state metadata
* `--pipeline, -p="<value>"`: Select a custom Buildkite `pipeline` in the Sourcegraph org (default: sourcegraph)
* `--state, -s="<value>"`: Job `state` to export logs for (provide an empty value for all states) (default: failed)

### sg ci docs

Render reference documentation for build pipeline types.


### sg ci open

Open Sourcegraph's Buildkite page in browser.

Arguments: `[pipeline]`

## sg test

Run the given test suite.

Testsuites are defined in sg configuration.

Available testsuites in `sg.config.yaml`:

* backend
* backend-integration
* bazel-e2e
* bazel-web-integration
* bext
* bext-build
* bext-e2e
* bext-integration
* client
* docsite
* web-e2e
* web-integration
* web-integration:debug
* web-regression

```sh
# Run different test suites:
$ sg test backend
$ sg test backend-integration
$ sg test client
$ sg test web-e2e

# List available test suites:
$ sg test -help

# Arguments are passed along to the command
$ sg test backend-integration -run TestSearch
```

## sg lint

Run all or specified linters on the codebase.

To run all checks, don't provide an argument. You can also provide multiple arguments to run linters for multiple targets.

```sh
# Run all possible checks
$ sg lint

# Run only go related checks
$ sg lint go

# Run only shell related checks
$ sg lint shell

# Run only client related checks
$ sg lint client

# List all available check groups
$ sg lint --help
```

Flags:

* `--annotations`: Write helpful output to ./annotations directory
* `--fail-fast, --ff`: Exit immediately if an issue is encountered (not available with '-fix')
* `--fix, -f`: Try to fix any lint issues
* `--skip-format-check, --sfc`: Skip file formatting check

### sg lint urls

Check for broken urls in the codebase.


### sg lint go

Check go code for linting errors, forbidden imports, generated files, etc.


### sg lint graphql

Checks the graphql code for linting errors [bazel].


### sg lint docs

Documentation checks.


### sg lint dockerfiles

Check Dockerfiles for Sourcegraph best practices.


### sg lint client

Check client code for linting errors, forbidden imports, etc.


### sg lint svg

Check svg assets.


### sg lint shell

Check shell code for linting errors, formatting, etc.


### sg lint protobuf

Check protobuf code for linting errors, formatting, etc.


### sg lint bazel generated

Ensures documentation and source generated by Bazel is up to date.


### sg lint format

Check client code and docs for formatting errors.


### sg lint format

Check client code and docs for formatting errors.


## sg generate

Run code and docs generation tasks.

If no target is provided, all target are run with default arguments.

```sh
$ sg --verbose generate ... # Enable verbose output
```

Flags:

* `--quiet, -q`: Suppress all output but errors from generate tasks

### sg generate buf

Re-generate protocol buffer bindings using buf.


### sg generate go

Run go generate [packages...] on the codebase.


### sg generate bazel

Run the bazel target //dev:write_all_generated.


## sg db

Interact with local Sourcegraph databases for development.

```sh
# Delete test databases
$ sg db delete-test-dbs

# Reset the Sourcegraph 'frontend' database
$ sg db reset-pg

# Reset the 'frontend' and 'codeintel' databases
$ sg db reset-pg -db=frontend,codeintel

# Reset all databases ('frontend', 'codeintel', 'codeinsights')
$ sg db reset-pg -db=all

# Reset the redis database
$ sg db reset-redis

# Create a site-admin user whose email and password are foo@sourcegraph.com and sourcegraph.
$ sg db add-user -username=foo

# Create an access token for the user created above.
$ sg db add-access-token -username=foo
```

### sg db delete-test-dbs

Drops all databases that have the prefix `sourcegraph-test-`.


### sg db reset-pg

Drops, recreates and migrates the specified Sourcegraph database.

If -db is not set, then the "frontend" database is used (what's set as PGDATABASE in env or the sg.config.yaml). If -db is set to "all" then all databases are reset and recreated.


Flags:

* `--db="<value>"`: The target database instance. (default: frontend)

### sg db reset-redis

Drops, recreates and migrates the specified Sourcegraph Redis database.

```sh
$ sg db reset-redis
```

### sg db update-user-external-services

Manually update a user's external services.

Patches the table 'user_external_services' with a custom OAuth token for the provided user. Used in dev/test environments. Set PGDATASOURCE to a valid connection string to patch an external database.


Flags:

* `--extsvc.display-name="<value>"`: The display name of the GitHub instance connected to the Sourcegraph instance (as listed under Site admin > Manage code hosts)
* `--github.baseurl="<value>"`: The base url of the GitHub instance to connect to
* `--github.client-id="<value>"`: The client ID of an OAuth app on the GitHub instance
* `--github.token="<value>"`: GitHub token with a scope to read all user data
* `--github.username="<value>"`: Username of the account on the GitHub instance (default: sourcegraph)
* `--oauth.token="<value>"`: OAuth token to patch for the provided user
* `--sg.username="<value>"`: Username of the user account on Sourcegraph (default: sourcegraph)

### sg db add-user

Create an admin sourcegraph user.

Run 'sg db add-user -username bob' to create an admin user whose email is bob@sourcegraph.com. The password will be printed if the operation succeeds


Flags:

* `--password="<value>"`: Password for user (default: sourcegraphsourcegraph)
* `--username="<value>"`: Username for user (default: sourcegraph)

### sg db add-access-token

Create a sourcegraph access token.

Run 'sg db add-access-token -username bob' to create an access token for the given username. The access token will be printed if the operation succeeds


Flags:

* `--note="<value>"`: Note attached to the token
* `--sudo`: Set true to make a site-admin level token
* `--username="<value>"`: Username for user (default: sourcegraph)

## sg migration

Modifies and runs database migrations.

```sh
# Migrate local default database up all the way
$ sg migration up

# Migrate specific database down one migration
$ sg migration downto --db codeintel --target <version>

# Add new migration for specific database
$ sg migration add --db codeintel 'add missing index'

# Squash migrations for default database
$ sg migration squash
```

### sg migration add

Add a new migration file.

Available schemas:

* frontend
* codeintel
* codeinsights

Arguments: `<name>`

Flags:

* `--schema, --db="<value>"`: The target database `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights' (default: frontend)

### sg migration revert

Revert the migrations defined on the given commit.

Available schemas:

* frontend
* codeintel
* codeinsights

Arguments: `<commit>`

### sg migration up

Apply all migrations.

Available schemas:

* frontend
* codeintel
* codeinsights

```sh
$ sg migration up [-db=<schema>]
```

Flags:

* `--ignore-single-dirty-log`: Ignore a single previously failed attempt if it will be immediately retried by this operation.
* `--ignore-single-pending-log`: Ignore a single pending migration attempt if it will be immediately retried by this operation.
* `--noop-privileged`: Skip application of privileged migrations, but record that they have been applied. This assumes the user has already applied the required privileged migrations with elevated permissions.
* `--privileged-hash="<value>"`: Running --noop-privileged without this flag will print instructions and supply a value for use in a second invocation. Multiple privileged hash flags (for distinct schemas) may be supplied. Future (distinct) up operations will require a unique hash.
* `--schema, --db="<value>"`: The target `schema(s)` to modify. Comma-separated values are accepted. Possible values are 'frontend', 'codeintel', 'codeinsights' and 'all'. (default: "all")
* `--skip-oobmigration-validation`: Do not attempt to validate the progress of out-of-band migrations.
* `--skip-upgrade-validation`: Do not attempt to compare the previous instance version with the target instance version for upgrade compatibility. Please refer to https://docs.sourcegraph.com/admin/updates#update-policy for our instance upgrade compatibility policy.
* `--unprivileged-only`: Refuse to apply privileged migrations.

### sg migration upto

Ensure a given migration has been applied - may apply dependency migrations.

Available schemas:

* frontend
* codeintel
* codeinsights

```sh
$ sg migration upto -db=<schema> -target=<target>,<target>,...
```

Flags:

* `--ignore-single-dirty-log`: Ignore a single previously failed attempt if it will be immediately retried by this operation.
* `--ignore-single-pending-log`: Ignore a single pending migration attempt if it will be immediately retried by this operation.
* `--noop-privileged`: Skip application of privileged migrations, but record that they have been applied. This assumes the user has already applied the required privileged migrations with elevated permissions.
* `--privileged-hash="<value>"`: Running --noop-privileged without this flag will print instructions and supply a value for use in a second invocation. Future (distinct) upto operations will require a unique hash.
* `--schema, --db="<value>"`: The target `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights'
* `--target="<value>"`: The `migration` to apply. Comma-separated values are accepted.
* `--unprivileged-only`: Refuse to apply privileged migrations.

### sg migration undo

Revert the last migration applied - useful in local development.

Available schemas:

* frontend
* codeintel
* codeinsights

```sh
$ sg migration undo -db=<schema>
```

Flags:

* `--schema, --db="<value>"`: The target `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights'

### sg migration downto

Revert any applied migrations that are children of the given targets - this effectively "resets" the schema to the target version.

Available schemas:

* frontend
* codeintel
* codeinsights

```sh
$ sg migration downto -db=<schema> -target=<target>,<target>,...
```

Flags:

* `--ignore-single-dirty-log`: Ignore a single previously failed attempt if it will be immediately retried by this operation.
* `--ignore-single-pending-log`: Ignore a single pending migration attempt if it will be immediately retried by this operation.
* `--noop-privileged`: Skip application of privileged migrations, but record that they have been applied. This assumes the user has already applied the required privileged migrations with elevated permissions.
* `--privileged-hash="<value>"`: Running --noop-privileged without this flag will print instructions and supply a value for use in a second invocation. Future (distinct) downto operations will require a unique hash.
* `--schema, --db="<value>"`: The target `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights'
* `--target="<value>"`: The migration to apply. Comma-separated values are accepted.
* `--unprivileged-only`: Refuse to apply privileged migrations.

### sg migration validate

Validate the current schema.

Available schemas:

* frontend
* codeintel
* codeinsights


Flags:

* `--schema, --db="<value>"`: The target `schema(s)` to validate. Comma-separated values are accepted. Possible values are 'frontend', 'codeintel', 'codeinsights' and 'all'. (default: "all")
* `--skip-out-of-band-migrations`: Do not attempt to validate out-of-band migration status.

### sg migration describe

Describe the current database schema.

Available schemas:

* frontend
* codeintel
* codeinsights


Flags:

* `--force`: Force write the file if it already exists.
* `--format="<value>"`: The target output format.
* `--no-color`: If writing to stdout, disable output colorization.
* `--out="<value>"`: The file to write to. If not supplied, stdout is used.
* `--schema, --db="<value>"`: The target `schema` to describe. Possible values are 'frontend', 'codeintel' and 'codeinsights'

### sg migration drift

Detect differences between the current database schema and the expected schema.

Available schemas:

* frontend
* codeintel
* codeinsights


Flags:

* `--auto-fix, --autofix`: Database goes brrrr.
* `--file="<value>"`: The target schema description file.
* `--ignore-migrator-update`: Ignore the running migrator not being the latest version. It is recommended to use the latest migrator version.
* `--schema, --db="<value>"`: The target `schema` to compare. Possible values are 'frontend', 'codeintel' and 'codeinsights'
* `--skip-version-check`: Skip validation of the instance's current version.
* `--version="<value>"`: The target schema version. Can be a version (e.g. 5.0.2) or resolvable as a git revlike on the Sourcegraph repository (e.g. a branch, tag or commit hash). (default: HEAD)

### sg migration add-log

Add an entry to the migration log.

Available schemas:

* frontend
* codeintel
* codeinsights

```sh
$ sg migration add-log -db=<schema> -version=<version> [-up=true|false]
```

Flags:

* `--schema, --db="<value>"`: The target `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights'
* `--up`: The migration direction.
* `--version="<value>"`: The migration `version` to log. (default: 0)

### sg migration leaves

Identify the migration leaves for the given commit.

Available schemas:

* frontend
* codeintel
* codeinsights

Arguments: `<commit>`

### sg migration squash

Collapse migration files from historic releases together.

Available schemas:

* frontend
* codeintel
* codeinsights

Arguments: `<current-release>`

Flags:

* `--in-container`: Launch Postgres in a Docker container for squashing; do not use the host
* `--in-timescaledb-container`: Launch TimescaleDB in a Docker container for squashing; do not use the host
* `--schema, --db="<value>"`: The target database `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights' (default: frontend)
* `--skip-data`: Skip writing data rows into the squashed migration
* `--skip-teardown`: Skip tearing down the database created to run all registered migrations

### sg migration squash-all

Collapse schema definitions into a single SQL file.

Available schemas:

* frontend
* codeintel
* codeinsights


Flags:

* `--in-container`: Launch Postgres in a Docker container for squashing; do not use the host
* `--in-timescaledb-container`: Launch TimescaleDB in a Docker container for squashing; do not use the host
* `--schema, --db="<value>"`: The target database `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights' (default: frontend)
* `--skip-data`: Skip writing data rows into the squashed migration
* `--skip-teardown`: Skip tearing down the database created to run all registered migrations
* `-f="<value>"`: The output filepath

### sg migration visualize

Output a DOT visualization of the migration graph.

Available schemas:

* frontend
* codeintel
* codeinsights


Flags:

* `--schema, --db="<value>"`: The target database `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights' (default: frontend)
* `-f="<value>"`: The output filepath

### sg migration rewrite

Rewrite schemas definitions as they were at a particular version.

Available schemas:

* frontend
* codeintel
* codeinsights


Flags:

* `--rev="<value>"`: The target revision
* `--schema, --db="<value>"`: The target database `schema` to modify. Possible values are 'frontend', 'codeintel' and 'codeinsights' (default: frontend)

## sg insights

Tools to interact with Code Insights data.


### sg insights decode-id

Decodes an encoded insight ID found on the frontend into a view unique_id.

Run 'sg insights decode-id' to decode 1+ frontend IDs which can then be used for SQL queries


### sg insights series-ids

Gets all insight series ID from the base64 encoded frontend ID.

Run 'sg insights series-ids' to decode a frontend ID and find all related series IDs


## sg telemetry

Operations relating to Sourcegraph telemetry.


### sg telemetry allowlist

Edit the usage data allow list.


Utility that will generate SQL to add and remove events from the usage data allow list.
https://docs.sourcegraph.com/dev/background-information/data-usage-pipeline#allow-list

Events are keyed by event name and passed in as additional arguments to the add and remove subcommands.


```sh
# Generate SQL to add events from the allow list
$ sg telemetry allowlist add EVENT_ONE EVENT_TWO

# Generate SQL to remove events from the allow list
$ sg telemetry allowlist remove EVENT_ONE EVENT_TWO

# Automatically generate migration files associated with the allow list modification
$ sg telemetry allowlist add --migration EVENT_ONE EVENT_TWO

# Provide a specific migration name for the migration files
$ sg telemetry allowlist add --migration --name my_migration_name EVENT_ONE EVENT_TWO
```

#### sg telemetry allowlist add

Generate the SQL required to add events to the allow list.

```sh
# Generate SQL to add events from the allow list
$ sg telemetry allowlist add EVENT_ONE EVENT_TWO

# Automatically generate migration files associated with the allow list modification
$ sg telemetry allowlist add --migration EVENT_ONE EVENT_TWO

# Provide a specific migration name for the migration files
$ sg telemetry allowlist add --migration --name my_migration_name EVENT_ONE EVENT_TWO
```

Flags:

* `--migration`: Create migration files with the generated SQL.
* `--name="<value>"`: Specifies the name of the resulting migration. (default: sg_telemetry_allowlist)

#### sg telemetry allowlist remove

Generate the SQL required to remove events from the allow list.

```sh
# Generate SQL to add events from the allow list
$ sg telemetry allowlist remove EVENT_ONE EVENT_TWO

# Automatically generate migration files associated with the allow list modification
$ sg telemetry allowlist remove --migration EVENT_ONE EVENT_TWO

# Provide a specific migration name for the migration files
$ sg telemetry allowlist remove --migration --name my_migration_name EVENT_ONE EVENT_TWO
```

Flags:

* `--migration`: Create migration files with the generated SQL.
* `--name="<value>"`: Specifies the name of the resulting migration. (default: sg_telemetry_allowlist)

## sg monitoring

Sourcegraph's monitoring generator (dashboards, alerts, etc).

Learn more about the Sourcegraph monitoring generator here: https://docs.sourcegraph.com/dev/background-information/observability/monitoring-generator

Also refer to the generated reference documentation available for site admins:

- https://docs.sourcegraph.com/admin/observability/dashboards
- https://docs.sourcegraph.com/admin/observability/alerts



### sg monitoring generate

Generate monitoring assets - dashboards, alerts, and more.

```sh
# Generate all monitoring with default configuration into a temporary directory
$ sg monitoring generate -all.dir /tmp/monitoring

# Generate and reload local instances of Grafana, Prometheus, etc.
$ sg monitoring generate -reload

# Render dashboards in a custom directory, and disable rendering of docs
$ sg monitoring generate -grafana.dir /tmp/my-dashboards -docs.dir ''
```

Flags:

* `--all.dir="<value>"`: Override all other '-*.dir' directories
* `--docs.dir="<value>"`: Output directory for generated documentation (default: $SG_ROOT/doc/admin/observability/)
* `--grafana.creds="<value>"`: Credentials for the Grafana instance to reload (default: admin:admin)
* `--grafana.dir="<value>"`: Output directory for generated Grafana assets (default: $SG_ROOT/docker-images/grafana/config/provisioning/dashboards/sourcegraph/)
* `--grafana.folder="<value>"`: Folder on Grafana instance to put generated dashboards in
* `--grafana.headers="<value>"`: Additional headers for HTTP requests to the Grafana instance
* `--grafana.url="<value>"`: Address for the Grafana instance to reload (default: http://127.0.0.1:3370)
* `--inject-label-matcher="<value>"`: Labels to inject into all selectors in Prometheus expressions: observable queries, dashboard template variables, etc.
* `--multi-instance-groupings="<value>"`: If non-empty, indicates whether or not to generate multi-instance assets with the provided labels to group on. The standard per-instance monitoring assets will NOT be generated.
* `--no-prune`: Toggles pruning of dangling generated assets through simple heuristic - should be disabled during builds.
* `--prometheus.dir="<value>"`: Output directory for generated Prometheus assets (default: $SG_ROOT/docker-images/prometheus/config/)
* `--prometheus.url="<value>"`: Address for the Prometheus instance to reload (default: http://127.0.0.1:9090)
* `--reload`: Trigger reload of active Prometheus or Grafana instance (requires respective output directories)

### sg monitoring dashboards

List and describe the default dashboards.

Arguments: `<dashboard...>`

Flags:

* `--groups`: Show row groups
* `--metrics`: Show metrics used in dashboards

### sg monitoring metrics

List metrics used in dashboards.

For per-dashboard summaries, use 'sg monitoring dashboards' instead.

Arguments: `<dashboard...>`

Flags:

* `--format, -f="<value>"`: Output format of list ('markdown', 'plain', 'regexp') (default: markdown)

## sg embeddings-qa

Calculate recall for embeddings.

Recall is the fraction of relevant documents that were successfully retrieved. Recall=1 if, for every query in the test data, all relevant documents were retrieved. The command requires a running embeddings service with embeddings of the Sourcegraph repository.


Flags:

* `--url, -u="<value>"`: Run the evaluation against this endpoint (default: http://localhost:9991/search)

## sg deploy

Generate a Kubernetes manifest for a Sourcegraph deployment.

Internal deployments live in the sourcegraph/infra repository.

```sh
$ sg deploy --values <path to values file>

$ Example of a values.yaml file:

$ name: my-app
$ image: gcr.io/sourcegraph-dev/my-app:latest
$ replicas: 1
$ envvars:
$ - name: ricky
$ value: foo
$ - name: julian
$ value: bar
$ containerPorts:
$ - name: frontend
$ port: 80
$ servicePorts:
$ - name: http
$ port: 80
$ targetPort: test # Set to the name or port number of the containerPort you want to expose
$ dns: dave-app.sgdev.org
```

Flags:

* `--dry-run`: Write the manifest to stdout instead of writing to a file
* `--infra-repo="<value>"`: The location of the sourcegraph/infrastructure repository. If undefined the currect directory will be used.
* `--values="<value>"`: The path to the values file

## sg wolfi

Automate Wolfi related tasks.

Build Wolfi packages and images locally, and update base image hashes

```sh
# Update base image hashes
$ sg wolfi update-hashes
$ sg wolfi update-hashes jaeger-agent

# Build a specific package using a manifest from wolfi-packages/
$ sg wolfi package jaeger
$ sg wolfi package jaeger.yaml

# Build a base image using a manifest from wolfi-images/
$ sg wolfi image gitserver
$ sg wolfi image gitserver.yaml
```

### sg wolfi package

Build a package locally using a manifest from sourcegraph/wolfi-packages/.

```sh
$ Build a Wolfi package locally by running Melange against a provided Melange manifest file, which can be found in sourcegraph/wolfi-packages.

$ This is convenient for testing package changes locally before pushing to the Wolfi registry.
$ Base images containing locally-built packages can then be built using 'sg wolfi image'.
```

### sg wolfi image

Build a base image locally using a manifest from sourcegraph/wolfi-images/.

```sh
$ Build a base image locally by running apko against a provided apko manifest file, which can be found in sourcegraph/wolfi-images.

$ Any packages built locally using 'sg wolfi package' can be included in the base image using the 'package@local' syntax in the base image manifest.
$ This is convenient for testing package changes locally before publishing them.

$ Once built, the base image is loaded into Docker and can be run locally.
$ It can also be used for local development by updating its path and hash in the 'dev/oci_deps.bzl' file.
```

### sg wolfi scan-images

Scan Wolfi base images for vulnerabilities.

```sh
$ Scans the Wolfi base images in the 'dev/oci_deps.bzl' file.
```

### sg wolfi update-hashes

Update Wolfi base images hashes to the latest versions.

```sh
$ Update the hash references for Wolfi base images in the 'dev/oci_deps.bzl' file.
$ By default all hashes will be updated; pass in a base image name to update a specific image.

$ Hash references are updated by fetching the ':latest' tag for each base image from the registry, and updating the corresponding hash in 'dev/oci_deps.bzl'.
```

## sg secret

Manipulate secrets stored in memory and in file.

```sh
# List all secrets stored in your local configuration.
$ sg secret list

# Remove the secrets associated with buildkite (sg ci build) - supports autocompletion for
# ease of use
$ sg secret reset buildkite
```

### sg secret reset

Remove a specific secret from secrets file.

Arguments: `<...key>`

### sg secret list

List all stored secrets.


Flags:

* `--view, -v`: Display configured secrets when listing

## sg setup

Validate and set up your local dev environment!.


Flags:

* `--check, -c`: Run checks and report setup state
* `--fix, -f`: Fix all checks
* `--skip-pre-commit`: Skip overwriting pre-commit.com installation

### sg setup disable-pre-commit

Disable pre-commit hooks.


## sg src

Run src-cli on a given instance defined with 'sg src-instance'.

```sh
$ sg src [src-cli args]
$ sg src help # get src-cli help
```

## sg src-instance

Interact with Sourcegraph instances that 'sg src' will use.

```sh
$ sg src-instance [command]
```

### sg src-instance register

Register (or edit an existing) Sourcegraph instance to target with src-cli.

```sh
$ sg src instance register [name] [endpoint]
```

### sg src-instance use

Set current src-cli instance to use with 'sg src'.


### sg src-instance list

List registered instances for src-cli.


## sg app

Manage releases and update manifests used to let Cody App clients know that a new update is available.


Various commands to handle management of releases, and processes around Cody App.



```sh
# Update the updater manifest
$ sg app update-manifest

# Update the updater manifest based on a particular github release
$ sg app update-manifest --release-tag app-v2023.07.07

# Do everything except upload the updated manifest
$ sg app update-manifest --no-upload

# Update the manifest but don't update the signatures from the release - useful if the release comes from the same build
$ sg app update-manifest --update-signatures

# Resets the dev app's db and web cache
$ sg app reset

# Prints the locations to be removed without deleting
$ sg app reset --dry-run
```

### sg app update-manifest

update the manifest used by the updater endpoint on dotCom.


Flags:

* `--bucket="<value>"`: Bucket where the updated manifest should be uploaded to once updated. (default: sourcegraph-app)
* `--build="<value>"`: Build number to retrieve the update-manifest from. If no build number is given, the latest build will be used (default: -1)
* `--no-upload`: do everything except upload the final manifest
* `--release-tag="<value>"`: GitHub release tag which should be used to update the manifest with. If no tag is given the latest GitHub release is used (default: latest)
* `--update-signatures`: update the signatures in the update manifest by retrieving the signature content from the GitHub release

### sg app reset

Resets the dev app's db and web cache.


Flags:

* `--dry-run`: write out paths to be removed

## sg teammate

Get information about Sourcegraph teammates.

For example, you can check a teammate's current time and find their handbook bio!

```sh
# Get the current time of a team mate based on their slack handle (case insensitive).
$ sg teammate time @dax
$ sg teammate time dax
# or their full name (case insensitive)
$ sg teammate time thorsten ball

# Open their handbook bio
$ sg teammate handbook asdine
```

### sg teammate time

Get the current time of a Sourcegraph teammate.

Arguments: `<nickname>`

### sg teammate handbook

Open the handbook page of a Sourcegraph teammate.

Arguments: `<nickname>`

## sg rfc

List, search, and open Sourcegraph RFCs.

Sourcegraph RFCs live in the following drives - see flags to configure which drive to query:

* Public: https://drive.google.com/drive/folders/1zP3FxdDlcSQGC1qvM9lHZRaHH4I9Jwwa
* Private: https://drive.google.com/drive/folders/1KCq4tMLnVlC0a1rwGuU5OSCw6mdDxLuv


```sh
# List all Public RFCs
$ sg rfc list

# List all Private RFCs
$ sg rfc --private list

# Search for a Public RFC
$ sg rfc search "search terms"

# Search for a Private RFC
$ sg rfc --private search "search terms"

# Open a specific Public RFC
$ sg rfc open 420

# Open a specific private RFC
$ sg rfc --private open 420

# Create a new public RFC
$ sg rfc create "title"

# Create a new private RFC. Possible types: [solution]
$ sg rfc --private create --type <type> "title"
```

Flags:

* `--private`: perform the RFC action on the private RFC drive

### sg rfc list

List Sourcegraph RFCs.


### sg rfc search

Search Sourcegraph RFCs.

Arguments: `[query]`

### sg rfc open

Open a Sourcegraph RFC - find and list RFC numbers with 'sg rfc list' or 'sg rfc search'.

Arguments: `[number]`

### sg rfc create

Create Sourcegraph RFCs.

Arguments: `--type <type> [title...]`

Flags:

* `--type="<value>"`: the type of the RFC to create (valid: solution) (default: solution)

## sg live

Reports which version of Sourcegraph is currently live in the given environment.

Prints the Sourcegraph version deployed to the given environment.

Available preset environments:

* s2
* dotcom
* k8s
* scaletesting

See more information about the deployments schedule here:
https://handbook.sourcegraph.com/departments/engineering/teams/dev-experience/#sourcegraph-instances-operated-by-us

```sh
# See which version is deployed on a preset environment
$ sg live s2
$ sg live dotcom
$ sg live k8s
$ sg live scaletesting

# See which version is deployed on a custom environment
$ sg live https://demo.sourcegraph.com

# List environments
$ sg live -help

# Check for commits further back in history
$ sg live -n 50 s2
```

Flags:

* `--commits, -c, -n="<value>"`: Number of commits to check for live version (default: 20)

## sg ops

Commands used by operations teams to perform common tasks.

Supports internal deploy-sourcegraph repos (non-customer facing)


### sg ops inspect-tag

Inspect main branch tag details from a image or tag.

```sh
# Inspect a full image
$ sg ops inspect-tag index.docker.io/sourcegraph/cadvisor:159625_2022-07-11_225c8ae162cc@sha256:foobar

# Inspect just the tag
$ sg ops inspect-tag 159625_2022-07-11_225c8ae162cc

# Get the build number
$ sg ops inspect-tag -p build 159625_2022-07-11_225c8ae162cc
```

Flags:

* `--property, -p="<value>"`: only output a specific `property` (one of: 'build', 'date', 'commit')

### sg ops update-images

Update images across a sourcegraph/deploy-sourcegraph/* manifests.

Arguments: `<dir>`

Flags:

* `--docker-password, --cr-password="<value>"`: dockerhub password
* `--docker-username, --cr-username="<value>"`: dockerhub username
* `--kind, -k="<value>"`: the `kind` of deployment (one of 'k8s', 'helm', 'compose') (default: k8s)
* `--pin-tag, -t="<value>"`: pin all images to a specific sourcegraph `tag` (e.g. '3.36.2', 'insiders') (default: latest main branch tag)
* `--registry="<value>"`: Sets the registry we want images to update to, public or internal. (default: public)
* `--skip, --skip-images="<value>"`: List of comma separated images to skip updating, ex: --skip 'gitserver,indexed-server'

## sg page

Page engineers at Sourcegraph - mostly used within scripts to automate paging alerts.

```sh
$ sg page --opsgenie.token [TOKEN] --message "something is broken" [my-schedule-on-ops-genie]
```

Flags:

* `--description, -d="<value>"`: Description for the paging alert (optional)
* `--escalation="<value>"`: Escalation team(s) to alert (if provided, target schedules can be omitted)
* `--message, -m="<value>"`: Message for the paging alert
* `--opsgenie.token="<value>"`: OpsGenie token
* `--priority, -p="<value>"`: Alert priority, importance decreases from P1 (critical) to P5 (lowest), defaults to P5 (default: P5)
* `--url="<value>"`: URL field for alert details (optional)

## sg cloud

Install and work with Sourcegraph Cloud tools.

Learn more about Sourcegraph Cloud:

- Product: https://docs.sourcegraph.com/cloud
- Handbook: https://handbook.sourcegraph.com/departments/cloud/



### sg cloud install

Install or upgrade local `mi2` CLI (for Cloud V2).

To learn more about Cloud V2, see https://handbook.sourcegraph.com/departments/cloud/technical-docs/v2.0/


## sg managed-services-platform

EXPERIMENTAL: Generate and manage services deployed on the Sourcegraph Managed Services Platform.

WARNING: This is currently still an experimental project.
To learm more, refer to go/rfc-msp and go/msp (https://handbook.sourcegraph.com/departments/engineering/teams/core-services/managed-services/platform)

MSP commands are currently build-flagged to avoid increasing 'sg' binary sizes. To install a build of 'sg' that includes 'sg msp', run:

	go build -tags=msp -o=./sg ./dev/sg && ./sg install -f -p=false

MSP commands should then be available under 'sg msp --help'.

```sh
# Create a service specification
$ sg msp init $SERVICE

# Provision Terraform Cloud workspaces
$ sg msp tfc sync $SERVICE $ENVIRONMENT

# Generate Terraform manifests
$ sg msp generate $SERVICE $ENVIRONMENT
```

## sg help

Get help and docs about sg.


Flags:

* `--full, -f`: generate full markdown sg reference
* `--help, -h`: show help
* `--output="<value>"`: write reference to `file`

## sg version

View details for this installation of sg.


### sg version changelog

See what's changed in or since this version of sg.


Flags:

* `--limit="<value>"`: Number of changelog entries to show. (default: 5)
* `--next`: Show changelog for changes you would get if you upgrade.

## sg update

Update local sg installation.

Update local sg installation with the latest changes. To see what's new, run:

    sg version changelog -next


## sg logo

Print the sg logo.

By default, prints the sg logo in different colors. When the 'classic' argument is passed it prints the classic logo.

Arguments: `[classic]`

## sg analytics

Manage analytics collected by sg.


### sg analytics submit

Make sg better by submitting all analytics stored locally!.

Requires HONEYCOMB_ENV_TOKEN or OTEL_EXPORTER_OTLP_ENDPOINT to be set.


### sg analytics reset

Delete all analytics stored locally.


### sg analytics view

View all analytics stored locally.


Flags:

* `--raw`: view raw data

## sg release

Sourcegraph release utilities.


### sg release cve-check

Check all CVEs found in a buildkite build against a set of preapproved CVEs for a release.

```sh
$ sg release cve-check -u https://handbook.sourcegraph.com/departments/security/tooling/trivy/4-2-0/ -b 184191
```

Flags:

* `--buildNumber, -b="<value>"`: The buildkite build number to check for CVEs
* `--uri, -u="<value>"`: A reference url that contains approved CVEs. Often a link to a handbook page eg: https://handbook.sourcegraph.com/departments/security/tooling/trivy/4-2-0/.
