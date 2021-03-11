package testing

import (
	"context"
	"testing"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

func InsertTestOrg(t *testing.T, db dbutil.DB, name string) (orgID int32) {
	t.Helper()

	q := sqlf.Sprintf("INSERT INTO orgs (name) VALUES (%s) RETURNING id", name)
	err := db.QueryRowContext(context.Background(), q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&orgID)
	if err != nil {
		t.Fatal(err)
	}

	return orgID
}
