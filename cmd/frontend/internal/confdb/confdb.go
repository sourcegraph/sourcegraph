package confdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/jsonx"

	"github.com/sourcegraph/sourcegraph/internal/conf/confdefaults"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
)

// SiteConfig contains the contents of a site config along with associated metadata.
type SiteConfig struct {
	ID        int32     // the unique ID of this config
	Contents  string    // the raw JSON content (with comments and trailing commas allowed)
	CreatedAt time.Time // the date when this config was created
	UpdatedAt time.Time // the date when this config was updated
}

// ErrNewerEdit is returned by SiteCreateIfUpToDate when a newer edit has already been applied and
// the edit has been rejected.
var ErrNewerEdit = errors.New("someone else has already applied a newer edit")

// SiteCreateIfUpToDate saves the given site config "contents" to the database iff the
// supplied "lastID" is equal to the one that was most recently saved to the database.
//
// The site config that was most recently saved to the database is returned.
// An error is returned if "contents" is invalid JSON.
//
// 🚨 SECURITY: This method does NOT verify the user is an admin. The caller is
// responsible for ensuring this or that the response never makes it to a user.
func SiteCreateIfUpToDate(ctx context.Context, lastID *int32, contents string) (latest *SiteConfig, err error) {
	tx, done, err := newTransaction(ctx)
	if err != nil {
		return nil, err
	}
	defer done()

	newLastID, err := addDefault(ctx, tx, confdefaults.Default.Site)
	if err != nil {
		return nil, err
	}
	if newLastID != nil {
		lastID = newLastID
	}

	return createIfUpToDate(ctx, tx, lastID, contents)
}

// SiteGetLatest returns the site config that was most recently saved to the database.
// This returns nil, nil if there is not yet a site config in the database.
//
// 🚨 SECURITY: This method does NOT verify the user is an admin. The caller is
// responsible for ensuring this or that the response never makes it to a user.
func SiteGetLatest(ctx context.Context) (latest *SiteConfig, err error) {
	tx, done, err := newTransaction(ctx)
	if err != nil {
		return nil, err
	}
	defer done()

	_, err = addDefault(ctx, tx, confdefaults.Default.Site)
	if err != nil {
		return nil, err
	}

	return getLatest(ctx, tx)
}

func newTransaction(ctx context.Context) (tx queryable, done func(), err error) {
	rtx, err := dbconn.Global.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	return rtx, func() {
		if err != nil {
			rollErr := rtx.Rollback()
			if rollErr != nil {
				err = multierror.Append(err, rollErr)
			}
			return
		}
		err = rtx.Commit()
	}, nil
}

func addDefault(ctx context.Context, tx queryable, contents string) (newLastID *int32, err error) {
	latest, err := getLatest(ctx, tx)
	if err != nil {
		return nil, err
	}
	if latest != nil {
		// We have an existing config!
		return nil, nil
	}

	// Create the default.
	latest, err = createIfUpToDate(ctx, tx, nil, contents)
	if err != nil {
		return nil, err
	}
	return &latest.ID, nil
}

func createIfUpToDate(ctx context.Context, tx queryable, lastID *int32, contents string) (latest *SiteConfig, err error) {
	// Validate JSON syntax before saving.
	if _, errs := jsonx.Parse(contents, jsonx.ParseOptions{Comments: true, TrailingCommas: true}); len(errs) > 0 {
		return nil, fmt.Errorf("invalid settings JSON: %v", errs)
	}

	new := SiteConfig{Contents: contents}

	latest, err = getLatest(ctx, tx)
	if err != nil {
		return nil, err
	}
	if latest != nil && lastID != nil && latest.ID != *lastID {
		return nil, ErrNewerEdit
	}

	err = tx.QueryRowContext(
		ctx,
		"INSERT INTO critical_and_site_config(type, contents) VALUES($1, $2) RETURNING id, created_at, updated_at",
		"site", new.Contents,
	).Scan(&new.ID, &new.CreatedAt, &new.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &new, nil
}

func getLatest(ctx context.Context, tx queryable) (*SiteConfig, error) {
	q := sqlf.Sprintf("SELECT s.id, s.contents, s.created_at, s.updated_at FROM critical_and_site_config s WHERE type=%s ORDER BY id DESC LIMIT 1", "site")
	rows, err := tx.QueryContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		return nil, err
	}
	versions, err := parseQueryRows(ctx, rows)
	if err != nil {
		return nil, err
	}
	if len(versions) != 1 {
		// No config has been written yet.
		return nil, nil
	}
	return versions[0], nil
}

func parseQueryRows(ctx context.Context, rows *sql.Rows) ([]*SiteConfig, error) {
	versions := []*SiteConfig{}
	defer rows.Close()
	for rows.Next() {
		f := SiteConfig{}
		err := rows.Scan(&f.ID, &f.Contents, &f.CreatedAt, &f.UpdatedAt)
		if err != nil {
			return nil, err
		}
		versions = append(versions, &f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

// queryable allows us to reuse the same logic for certain operations both
// inside and outside an explicit transaction.
type queryable interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}
