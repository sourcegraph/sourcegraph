package shared

import (
	"bytes"
	"os"
	"path/filepath"
)

func maybePostgresProcFile() (string, error) {
	return maybePostgresProcFileWithPrefix("")
}

func maybePostgresProcFileWithPrefix(prefix string) (string, error) {
	// PG is already configured
	if os.Getenv(prefix+"PGHOST") != "" || os.Getenv(prefix+"PGDATASOURCE") != "" {
		return "", nil
	}

	// Postgres needs to be able to write to run
	var output bytes.Buffer
	e := execer{Out: &output}
	e.Command("mkdir", "-p", "/run/postgresql")
	e.Command("chown", "-R", "postgres", "/run/postgresql")
	if err := e.Error(); err != nil {
		l("Setting up postgres failed:\n%s", output.String())
		return "", err
	}

	path := filepath.Join(os.Getenv("DATA_DIR"), "postgresql")

	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}

		if verbose {
			l("Setting up PostgreSQL at %s", path)
		}
		l("Sourcegraph is initializing the internal database... (may take 15-20 seconds)")

		var output bytes.Buffer
		e := execer{Out: &output}
		e.Command("mkdir", "-p", path)
		e.Command("chown", "postgres", path)
		// initdb --nosync saves ~3-15s on macOS during initial startup. By the time actual data lives in the
		// DB, the OS should have had time to fsync.
		e.Command("su-exec", "postgres", "initdb", "-D", path, "--nosync")
		e.Command("su-exec", "postgres", "pg_ctl", "-D", path, "-o -c listen_addresses=127.0.0.1", "-l", "/tmp/pgsql.log", "-w", "start")
		e.Command("su-exec", "postgres", "createdb", "sourcegraph")
		e.Command("su-exec", "postgres", "pg_ctl", "-D", path, "-m", "fast", "-l", "/tmp/pgsql.log", "-w", "stop")
		if err := e.Error(); err != nil {
			l("Setting up postgres failed:\n%s", output.String())
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
			l("Adjusting fs owners for postgres failed:\n%s", output.String())
			return "", err
		}
	}

	// Set *PGHOST to default to 127.0.0.1, NOT localhost, as localhost does not correctly resolve in some environments
	// (see https://github.com/sourcegraph/issues/issues/34 and https://github.com/sourcegraph/sourcegraph/issues/9129).
	SetDefaultEnv(prefix+"PGHOST", "127.0.0.1")
	SetDefaultEnv(prefix+"PGUSER", "postgres")
	SetDefaultEnv(prefix+"PGDATABASE", "sourcegraph")
	SetDefaultEnv(prefix+"PGSSLMODE", "disable")

	return "postgres: su-exec postgres sh -c 'postgres -c listen_addresses=127.0.0.1 -D " + path + "' 2>&1 | grep -v 'database system was shut down' | grep -v 'MultiXact member wraparound' | grep -v 'database system is ready' | grep -v 'autovacuum launcher started' | grep -v 'the database system is starting up' | grep -v 'listening on IPv4 address'", nil
}
