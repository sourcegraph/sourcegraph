package errors

import (
	"github.com/jackc/pgx/v5/pgconn"
)

// HasPostgresCode checks whether any of the errors in the chain
// signify a postgres error with the given error code.
func HasPostgresCode(err error, code string) bool {
	var pgerr *pgconn.PgError
	return As(err, &pgerr) && pgerr.Code == code
}
