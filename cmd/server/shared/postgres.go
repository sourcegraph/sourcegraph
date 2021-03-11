package shared

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var databases = map[string]string{
	"":           "sourcegraph",
	"CODEINTEL_": "sourcegraph-codeintel",
}

func maybePostgresProcFile() (string, error) {
	missingExternalConfig := false
	for prefix := range databases {
		if !isPostgresConfigured(prefix) {
			missingExternalConfig = true
		}
	}
	if !missingExternalConfig {
		// All target databases are configured to hit an external server.
		// Do not start the postgres instance inside the container as no
		// service will connect to it.
		return "", nil
	}

	// If we get here, _some_ service will use in the in-container postgres
	// instance. Ensure that everything is in place and generate a line for
	// the procfile to start it.
	procfile, err := postgresProcfile()
	if err != nil {
		return "", err
	}

	// Each un-configured service will point to the database instance that
	// we configured above.
	for prefix, database := range databases {
		if !isPostgresConfigured(prefix) {
			// Set *PGHOST to default to 127.0.0.1, NOT localhost, as localhost does not correctly resolve in some environments
			// (see https://github.com/sourcegraph/issues/issues/34 and https://github.com/sourcegraph/sourcegraph/issues/9129).
			SetDefaultEnv(prefix+"PGHOST", "127.0.0.1")
			SetDefaultEnv(prefix+"PGUSER", "postgres")
			SetDefaultEnv(prefix+"PGDATABASE", database)
			SetDefaultEnv(prefix+"PGSSLMODE", "disable")
		}
	}

	return procfile, nil
}

func postgresProcfile() (string, error) {
	// Postgres needs to be able to write to run
	var output bytes.Buffer
	e := execer{Out: &output}
	e.Command("mkdir", "-p", "/run/postgresql")
	e.Command("chown", "-R", "postgres", "/run/postgresql")
	if err := e.Error(); err != nil {
		pgPrintf("Setting up postgres failed:\n%s", output.String())
		return "", err
	}

	dataDir := os.Getenv("DATA_DIR")
	path := filepath.Join(dataDir, "postgresql")
	markersPath := filepath.Join(dataDir, "postgresql-markers")

	if ok, err := fileExists(markersPath); err != nil {
		return "", err
	} else if !ok {
		var output bytes.Buffer
		e := execer{Out: &output}
		e.Command("mkdir", "-p", markersPath)
		e.Command("touch", filepath.Join(markersPath, "sourcegraph"))

		if err := e.Error(); err != nil {
			pgPrintf("Failed to set up postgres database marker files:\n%s", output.String())
			os.RemoveAll(path)
			return "", err
		}
	}

	if ok, err := fileExists(path); err != nil {
		return "", err
	} else if !ok {
		if verbose {
			pgPrintf("Setting up PostgreSQL at %s", path)
		}
		pgPrintf("Sourcegraph is initializing the internal database... (may take 15-20 seconds)")

		var output bytes.Buffer
		e := execer{Out: &output}
		e.Command("mkdir", "-p", path)
		e.Command("chown", "postgres", path)
		// initdb --nosync saves ~3-15s on macOS during initial startup. By the time actual data lives in the
		// DB, the OS should have had time to fsync.
		e.Command("su-exec", "postgres", "initdb", "-D", path, "--nosync")
		e.Command("su-exec", "postgres", "pg_ctl", "-D", path, "-o -c listen_addresses=127.0.0.1", "-l", "/tmp/pgsql.log", "-w", "start")
		for _, database := range databases {
			e.Command("su-exec", "postgres", "createdb", database)
			e.Command("touch", filepath.Join(markersPath, database))
		}
		e.Command("su-exec", "postgres", "pg_ctl", "-D", path, "-m", "fast", "-l", "/tmp/pgsql.log", "-w", "stop")
		if err := e.Error(); err != nil {
			pgPrintf("Setting up postgres failed:\n%s", output.String())
			os.RemoveAll(path)
			return "", err
		}
	} else {
		// Between restarts the owner of the volume may have changed. Ensure
		// postgres can still read it.
		var output bytes.Buffer
		e := execer{Out: &output}
		e.Command("chown", "-R", "postgres", path)
		if err := e.Error(); err != nil {
			pgPrintf("Adjusting fs owners for postgres failed:\n%s", output.String())
			return "", err
		}

		var missingDatabases []string
		for _, database := range databases {
			ok, err := fileExists(filepath.Join(markersPath, database))
			if err != nil {
				return "", err
			} else if !ok {
				missingDatabases = append(missingDatabases, database)
			}
		}
		if len(missingDatabases) > 0 {
			pgPrintf("Sourcegraph is creating missing databases %s... (may take 15-20 seconds)", strings.Join(missingDatabases, ", "))

			e.Command("su-exec", "postgres", "pg_ctl", "-D", path, "-o -c listen_addresses=127.0.0.1", "-l", "/tmp/pgsql.log", "-w", "start")
			for _, database := range missingDatabases {
				alreadyExistsFilter := func(err error, out string) bool {
					return !strings.Contains(out, fmt.Sprintf(`ERROR:  database "%s" already exists`, database))
				}

				// Ignore errors about the databse already existing. This can happen on the
				// upgrade path from 3.21.0 -> 3.21.1 (or later), as both databases were created
				// for fresh installs of 3.21.0 but no files were created. This means that we can't
				// differentiate between a codeintel database being created on 3.21.0 and it not
				// existing at all. We need to at least try to create it here, and in the worst case
				// we start up postgres and shut it down without modification for one startup until
				// we touch the marker file.
				e.CommandWithFilter(alreadyExistsFilter, "su-exec", "postgres", "createdb", database)
				e.Command("touch", filepath.Join(markersPath, database))
			}
			e.Command("su-exec", "postgres", "pg_ctl", "-D", path, "-m", "fast", "-l", "/tmp/pgsql.log", "-w", "stop")
			if err := e.Error(); err != nil {
				pgPrintf("Setting up postgres failed:\n%s", output.String())
				return "", err
			}
		}
	}
	pgPrintf("Finished initializing the internal database.")
	return "postgres: su-exec postgres sh -c 'postgres -c listen_addresses=127.0.0.1 -D " + path + "' 2>&1 | grep -v 'database system was shut down' | grep -v 'MultiXact member wraparound' | grep -v 'database system is ready' | grep -v 'autovacuum launcher started' | grep -v 'the database system is starting up' | grep -v 'listening on IPv4 address'", nil
}

func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func isPostgresConfigured(prefix string) bool {
	return os.Getenv(prefix+"PGHOST") != "" || os.Getenv(prefix+"PGDATASOURCE") != ""
}

func pgPrintf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, "✱ "+format+"\n", args...)
}

var logLevelConverter = map[string]string{
	"dbug":  "debug",
	"info":  "info",
	"warn":  "warn",
	"error": "error",
	"crit":  "fatal",
}

// convertLogLevel converts a sourcegraph log level (dbug, info, warn, error, crit) into
// values postgres exporter accepts (debug, info, warn, error, fatal)
// If value cannot be converted returns "warn" which seems like a good middle-ground.
func convertLogLevel(level string) string {
	lvl, ok := logLevelConverter[level]
	if ok {
		return lvl
	}
	return "warn"
}
