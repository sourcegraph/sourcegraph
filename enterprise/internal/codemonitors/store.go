package codemonitors

import (
	"context"
	"database/sql"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

// Store exposes methods to read and write codemonitors domain models
// from persistent storage.
type Store struct {
	*basestore.Store
	now func() time.Time
}

// NewStore returns a new Store backed by the given database.
func NewStore(db dbutil.DB) *Store {
	return NewStoreWithClock(db, timeutil.Now)
}

// NewStoreWithClock returns a new Store backed by the given database and
// clock for timestamps.
func NewStoreWithClock(db dbutil.DB, clock func() time.Time) *Store {
	return &Store{Store: basestore.NewWithDB(db, sql.TxOptions{}), now: clock}
}

// With creates a new store with the underlying database handle from the given store.
func (s *Store) With(other basestore.ShareableStore) *Store {
	return &Store{
		Store: s.Store.With(other),
		now:   timeutil.Now,
	}
}

// Clock returns the clock of the underlying store.
func (s *Store) Clock() func() time.Time {
	return s.now
}

func (s *Store) Now() time.Time {
	return s.now()
}

// Transact creates a new transaction.
// It's required to implement this method and wrap the Transact method of the
// underlying basestore.Store.
func (s *Store) Transact(ctx context.Context) (*Store, error) {
	txBase, err := s.Store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	return &Store{Store: txBase, now: s.now}, nil
}
