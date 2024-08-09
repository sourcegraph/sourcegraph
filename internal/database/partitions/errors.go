package partitions

import (
	"errors"

	"github.com/jackc/pgconn"
)

func IsMissingPartition(err error) bool {
	var e *pgconn.PgError
	// ERROR: no partition of relation "tablename" found for row (SQLSTATE 23514)
	return errors.As(err, &e) && e.Code == "23514"
}
