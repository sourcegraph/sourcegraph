// Package singleprogram contains runtime utilities for the single-program (Go static binary)
// distribution of Sourcegraph.
package singleprogram

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf/confdefaults"
	"github.com/sourcegraph/sourcegraph/internal/env"
)

func Init(logger log.Logger) {
	// TODO(sqs) TODO(single-binary): see the env.HackClearEnvironCache docstring, we should be able to remove this
	// eventually.
	env.HackClearEnvironCache()

	// INDEXED_SEARCH_SERVERS is empty (but defined) so that indexed search is disabled.
	setDefaultEnv(logger, "INDEXED_SEARCH_SERVERS", "")

	// GITSERVER_EXTERNAL_ADDR is used by gitserver to identify itself in the
	// list in SRC_GIT_SERVERS.
	setDefaultEnv(logger, "GITSERVER_ADDR", "127.0.0.1:3178")
	setDefaultEnv(logger, "GITSERVER_EXTERNAL_ADDR", "127.0.0.1:3178")
	setDefaultEnv(logger, "SRC_GIT_SERVERS", "127.0.0.1:3178")

	setDefaultEnv(logger, "SYMBOLS_URL", "http://127.0.0.1:3184")
	setDefaultEnv(logger, "SEARCHER_URL", "http://127.0.0.1:3181")
	setDefaultEnv(logger, "BLOBSTORE_URL", "http://127.0.0.1:9000")

	// The syntax-highlighter might not be running, but this is a better default than an internal
	// hostname.
	setDefaultEnv(logger, "SRC_SYNTECT_SERVER", "http://localhost:9238")

	// Jaeger might not be running, but this is a better default than an internal hostname.
	//
	// TODO(sqs) TODO(single-binary): this isnt taking effect
	//
	// setDefaultEnv(logger, "JAEGER_SERVER_URL", "http://localhost:16686")

	// The s3proxy blobstore does need to be running. TODO(sqs): TODO(single-binary): bundle this somehow?
	setDefaultEnv(logger, "PRECISE_CODE_INTEL_UPLOAD_AWS_ENDPOINT", "http://localhost:9000")
	setDefaultEnv(logger, "PRECISE_CODE_INTEL_UPLOAD_BACKEND", "blobstore")

	// Need to override this because without a host (eg ":3080") it listens only on localhost, which
	// is not accessible from the containers
	setDefaultEnv(logger, "SRC_HTTP_ADDR", "0.0.0.0:3080")

	// This defaults to an internal hostname.
	setDefaultEnv(logger, "SRC_FRONTEND_INTERNAL", "localhost:3090")

	cacheDir, err := os.UserCacheDir()
	if err == nil {
		cacheDir = filepath.Join(cacheDir, "sourcegraph-sp")
		err = os.MkdirAll(cacheDir, 0700)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to make user cache directory:", err)
		os.Exit(1)
	}

	setDefaultEnv(logger, "SRC_REPOS_DIR", filepath.Join(cacheDir, "repos"))
	setDefaultEnv(logger, "BLOBSTORE_DATA_DIR", filepath.Join(cacheDir, "blobstore"))
	setDefaultEnv(logger, "SYMBOLS_CACHE_DIR", filepath.Join(cacheDir, "symbols"))
	setDefaultEnv(logger, "SEARCHER_CACHE_DIR", filepath.Join(cacheDir, "searcher"))

	configDir, err := os.UserConfigDir()
	if err == nil {
		configDir = filepath.Join(configDir, "sourcegraph-sp")
		err = os.MkdirAll(configDir, 0700)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to make user config directory:", err)
		os.Exit(1)
	}

	embeddedPostgreSQLRootDir := filepath.Join(configDir, "postgresql")
	if err := initPostgreSQL(logger, embeddedPostgreSQLRootDir); err != nil {
		fmt.Fprintln(os.Stderr, "unable to set up PostgreSQL:", err)
		os.Exit(1)
	}

	writeFileIfNotExists := func(path string, data []byte) {
		var err error
		if _, err = os.Stat(path); os.IsNotExist(err) {
			err = os.WriteFile(path, data, 0600)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to write file %s: %s\n", path, err)
			os.Exit(1)
		}
	}

	siteConfigPath := filepath.Join(configDir, "site-config.json")
	setDefaultEnv(logger, "SITE_CONFIG_FILE", siteConfigPath)
	setDefaultEnv(logger, "SITE_CONFIG_ALLOW_EDITS", "true")
	writeFileIfNotExists(siteConfigPath, []byte(confdefaults.SingleProgram.Site))

	globalSettingsPath := filepath.Join(configDir, "global-settings.json")
	setDefaultEnv(logger, "GLOBAL_SETTINGS_FILE", globalSettingsPath)
	setDefaultEnv(logger, "GLOBAL_SETTINGS_ALLOW_EDITS", "true")
	writeFileIfNotExists(globalSettingsPath, []byte("{}\n"))

	// Escape hatch isn't needed in local dev since the site config can always just be a file on disk.
	setDefaultEnv(logger, "NO_SITE_CONFIG_ESCAPE_HATCH", "1")

	// We disable the use of executors passwords, because executors only listen on `localhost` this
	// is safe to do.
	setDefaultEnv(logger, "EXECUTOR_FRONTEND_URL", "http://localhost:3080")
	setDefaultEnv(logger, "EXECUTOR_FRONTEND_PASSWORD", confdefaults.SingleProgramInMemoryExecutorPassword)

	// TODO(single-binary): HACK: This is a hack to workaround the fact that the 2nd time you run `sourcegraph`
	// OOB migration validation fails:
	//
	// {"SeverityText":"FATAL","Timestamp":1675128552556359000,"InstrumentationScope":"sourcegraph","Caller":"svcmain/svcmain.go:143","Function":"github.com/sourcegraph/sourcegraph/internal/service/svcmain.run.func1","Body":"failed to start service","Resource":{"service.name":"sourcegraph","service.version":"0.0.196384-snapshot+20230131-6902ad","service.instance.id":"Stephens-MacBook-Pro.local"},"Attributes":{"service":"frontend","error":"failed to validate out of band migrations: Unfinished migrations. Please revert Sourcegraph to the previous version and wait for the following migrations to complete.\n  - migration 1 expected to be at 0.00% (at 100.00%)\n  - migration 13 expected to be at 0.00% (at 100.00%)\n  - migration 14 expected to be at 0.00% (at 100.00%)\n  - migration 15 expected to be at 0.00% (at 100.00%)\n  - migration 16 expected to be at 0.00% (at 100.00%)\n  - migration 17 expected to be at 0.00% (at 100.00%)\n  - migration 18 expected to be at 0.00% (at 100.00%)\n  - migration 19 expected to be at 0.00% (at 100.00%)\n  - migration 2 expected to be at 0.00% (at 100.00%)\n  - migration 20 expected to be at 0.00% (at 100.00%)\n  - migration 4 expected to be at 0.00% (at 100.00%)\n  - migration 5 expected to be at 0.00% (at 100.00%)\n  - migration 7 expected to be at 0.00% (at 100.00%)"}}
	//
	setDefaultEnv(logger, "SRC_DISABLE_OOBMIGRATION_VALIDATION", "1")

	setDefaultEnv(logger, "EXECUTOR_USE_FIRECRACKER", "false")
	// TODO(sqs): TODO(single-binary): Make it so we can run multiple executors in single-program mode. Right now, you
	// need to change this to "batches" to use batch changes executors.
	setDefaultEnv(logger, "EXECUTOR_QUEUE_NAME", "codeintel")

	writeFile := func(path string, data []byte, perm fs.FileMode) {
		if err := os.WriteFile(path, data, perm); err != nil {
			fmt.Fprintf(os.Stderr, "unable to write file %s: %s\n", path, err)
			os.Exit(1)
		}
	}

	setDefaultEnv(logger, "CTAGS_PROCESSES", "2")

	// generate a shell script to run a ctags Docker image
	// unless the environment is already set up to find ctags
	ctagsPath := os.Getenv("CTAGS_COMMAND")
	if stat, err := os.Stat(ctagsPath); err != nil || stat.IsDir() {
		// Write script that invokes universal-ctags via Docker.
		// TODO(sqs): TODO(single-binary): stop relying on a ctags Docker image
		ctagsPath = filepath.Join(cacheDir, "universal-ctags-dev")
		writeFile(ctagsPath, []byte(universalCtagsDevScript), 0700)
		setDefaultEnv(logger, "CTAGS_COMMAND", ctagsPath)
	}
}

// universalCtagsDevScript is copied from cmd/symbols/universal-ctags-dev.
const universalCtagsDevScript = `#!/usr/bin/env bash

# This script is a wrapper around universal-ctags.

exec docker run --rm -i \
    -a stdin -a stdout -a stderr \
    --user guest \
    --platform=linux/amd64 \
    --name=universal-ctags-$$ \
    --entrypoint /usr/local/bin/universal-ctags \
    slimsag/ctags:latest@sha256:dd21503a3ae51524ab96edd5c0d0b8326d4baaf99b4238dfe8ec0232050af3c7 "$@"
`

// setDefaultEnv will set the environment variable if it is not set.
func setDefaultEnv(logger log.Logger, k, v string) {
	if _, ok := os.LookupEnv(k); ok {
		return
	}
	err := os.Setenv(k, v)
	if err != nil {
		logger.Fatal("setting default env variable", log.Error(err))
	}
}
