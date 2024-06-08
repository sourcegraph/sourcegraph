package store

import (
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type NotFoundError struct{}

func (e *NotFoundError) Error() string {
	return "not found"
}

func (e *NotFoundError) NotFound() bool {
	return true
}

// New returns a new Store. The encryption key passed is used for interacting with
// the database to read and write client secrets.
func New(db database.DB, observationCtx *observation.Context, key encryption.Key) *Store {
	return &Store{
		Store: basestore.NewWithHandle(db.Handle()),
		key:   key,
		// operations:     newOperations(observationCtx),
	}
}
