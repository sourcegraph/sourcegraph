package dbconn

import (
	"os"

	"github.com/jackc/pgx/v5"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	defaultDataSource      = env.Get("PGDATASOURCE", "", "Default dataSource to pass to Postgres. See https://pkg.go.dev/github.com/jackc/pgx for more information.")
	defaultApplicationName = env.Get("PGAPPLICATIONNAME", "sourcegraph", "The value of application_name appended to dataSource")
	// Ensure all time instances have their timezones set to UTC.
	// https://github.com/golang/go/blob/7eb31d999cf2769deb0e7bdcafc30e18f52ceb48/src/time/zoneinfo_unix.go#L29-L34
	_ = env.Ensure("TZ", "UTC", "timezone used by time instances")
)

// buildConfig takes either a Postgres connection string or connection URI,
// parses it, and returns a config with additional parameters.
func buildConfig(logger log.Logger, dataSource, app string) (*pgx.ConnConfig, error) {
	if dataSource == "" {
		dataSource = defaultDataSource
	}

	if app == "" {
		app = defaultApplicationName
	}

	cfg, err := pgx.ParseConfig(dataSource)
	if err != nil {
		return nil, err
	}

	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = make(map[string]string)
	}

	// pgx doesn't support dbname so we emulate it
	if dbname, ok := cfg.RuntimeParams["dbname"]; ok {
		cfg.Database = dbname
		delete(cfg.RuntimeParams, "dbname")
	}

	// pgx doesn't support fallback_application_name so we emulate it
	// by checking if application_name is set and setting a default
	// value if not.
	if _, ok := cfg.RuntimeParams["application_name"]; !ok {
		cfg.RuntimeParams["application_name"] = app
	}

	// Force PostgreSQL session timezone to UTC.
	// pgx doesn't support the PGTZ environment variable, we need to pass
	// that information in the configuration instead.
	tz := "UTC"
	if v, ok := os.LookupEnv("PGTZ"); ok && v != "UTC" && v != "utc" {
		logger.Warn("Ignoring PGTZ environment variable; using PGTZ=UTC.", log.String("ignoredPGTZ", v))
	}
	// We set the environment variable to PGTZ to avoid bad surprises if and when
	// it will be supported by the driver.
	if err := os.Setenv("PGTZ", "UTC"); err != nil {
		return nil, errors.Wrap(err, "Error setting PGTZ=UTC")
	}
	cfg.RuntimeParams["timezone"] = tz

	// Ensure the TZ environment variable is set so that times are parsed correctly.
	if _, ok := os.LookupEnv("TZ"); !ok {
		logger.Warn("TZ environment variable not defined; using TZ=''.")
		if err := os.Setenv("TZ", ""); err != nil {
			return nil, errors.Wrap(err, "Error setting TZ=''")
		}
	}

	return cfg, nil
}
