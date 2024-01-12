package dbconn

import (
	"github.com/jackc/pgx/v5"

	"github.com/sourcegraph/sourcegraph/internal/database/dbconn/rds"
	"github.com/sourcegraph/sourcegraph/internal/env"
)

var (
	pgConnectionUpdater = env.Get("PG_CONNECTION_UPDATER", "", "The postgres connection updater use for connecting to the database.")
)

// ConnectionUpdater is an interface to allow updating pgx connection config
// before opening a new connection.
//
// Use cases:
// - Implement RDS IAM auth that requires refreshing the auth token regularly.
type ConnectionUpdater interface {
	// Update applies update to the pgx connection config
	//
	// It is concurrency safe.
	Update(*pgx.ConnConfig) (*pgx.ConnConfig, error)

	// ShouldUpdate indicates whether the connection config should be updated
	ShouldUpdate(*pgx.ConnConfig) bool
}

var connectionUpdaters = map[string]ConnectionUpdater{
	"EC2_ROLE_CREDENTIALS": rds.NewUpdater(),
}
