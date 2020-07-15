package db

import (
	"context"
	"database/sql"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/types"
	"github.com/sourcegraph/sourcegraph/internal/db/dbconn"
)

// This object provides access to the secrets table
type secrets struct{}

// Returned when we can't retrieve the specific crypt object that we need
type SecretNotFoundError struct {
}

// We always try to find *one* object.
func (err *SecretNotFoundError) Error() string {
	return "failed to find matching secret"
}

// Delete by object id
func (s *secrets) Delete(ctx context.Context, id int32) error {
	if Mocks.Secrets.Delete != nil {
		return Mocks.Secrets.Delete(ctx, id)
	}

	sqlQ := sqlf.Sprintf(
		`DELETE FROM
			secrets
		WHERE
			id=$1
		`, id)

	_, err := dbconn.Global.ExecContext(ctx, sqlQ.Query(sqlf.PostgresBindVar), sqlQ.Args()...)
	if err != nil {
		return err
	}

	return nil
}

// Delete by key name
func (s *secrets) DeleteByKeyName(ctx context.Context, keyName string) error {
	if Mocks.Secrets.DeleteByKeyName != nil {
		return Mocks.Secrets.DeleteByKeyName(ctx, keyName)
	}

	sqlQ := sqlf.Sprintf(
		`DELETE FROM
			secrets
		WHERE
			key_name=$1
		`, keyName)

	_, err := dbconn.Global.ExecContext(ctx, sqlQ.Query(sqlf.PostgresBindVar), sqlQ.Args()...)
	if err != nil {
		return err
	}

	return nil
}

// Delete the object by sourceType (i.e a repo style object) and the source id.
func (s *secrets) DeleteBySourceTypeAndID(ctx context.Context, sourceType string, sourceID int32) error {
	if Mocks.Secrets.DeleteBySourceTypeAndID != nil {
		return Mocks.Secrets.DeleteBySourceTypeAndID(ctx, sourceType, sourceID)
	}

	sqlQ :=
		`DELETE FROM
			secrets
		WHERE
			source_type=$1 AND source_id=$2
		`

	_, err := dbconn.Global.ExecContext(ctx, sqlQ, sourceType, sourceID)
	if err != nil {
		return err
	}

	return nil
}

func (s *secrets) getBySQL(ctx context.Context, query *sqlf.Query) (*types.Secret, error) {
	res, err := dbconn.Global.QueryContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	if err != nil {
		return nil, err
	}

	var results []*types.Secret
	defer res.Close()

	for res.Next() {
		var obj types.Secret
		if err := res.Scan(&obj.ID, &obj.SourceType, &obj.SourceID, &obj.Value); err != nil {
			return nil, err
		}
		results = append(results, &obj)
	}

	if len(results) != 1 {
		return nil, &SecretNotFoundError{}
	}

	return results[0], nil
}

// Get by object id
func (s *secrets) Get(ctx context.Context, id int32) (*types.Secret, error) {
	if Mocks.Secrets.Get != nil {
		return Mocks.Secrets.Get(ctx, id)
	}

	sqlQ := sqlf.Sprintf(
		`SELECT
			*
		FROM
			secrets
		WHERE
			id=$1
		`, id)

	return s.getBySQL(ctx, sqlQ)
}

// Get the secret by the key name - for key/value pair secrets
func (s *secrets) GetByKeyName(ctx context.Context, keyName string) (*types.Secret, error) {
	if Mocks.Secrets.GetByKeyName != nil {
		return Mocks.Secrets.GetByKeyName(ctx, keyName)
	}

	sqlQ := sqlf.Sprintf(
		`SELECT
			*
		FROM
			secrets
		WHERE
			key_name=$1
		`, keyName)

	return s.getBySQL(ctx, sqlQ)
}

// Get the secret by the sourceType and source id (i.e the specific repo entity)
func (s *secrets) GetBySourceTypeAndID(ctx context.Context, sourceType string, sourceID int32) (*types.Secret, error) {
	sqlQ := sqlf.Sprintf(
		`SELECT *
		FROM
			secrets
		WHERE
		`, sourceType, sourceID)

	return s.getBySQL(ctx, sqlQ)
}

func (s *secrets) insert(ctx context.Context, query *sqlf.Query) error {
	_, err := dbconn.Global.ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	return err
}

// Insert a new key-value secret
func (s *secrets) InsertKeyValue(ctx context.Context, keyName, value string) error {
	sqlQ := sqlf.Sprintf(
		`INSERT INTO
			secrets(key_name, value)
		VALUES($1, $2)
		`, keyName, value)
	return s.insert(ctx, sqlQ)
}

// Insert a new secret referenced by another table type
func (s *secrets) InsertSourceTypeValue(ctx context.Context, sourceType string, sourceID int32, value string) error {
	sqlQ := sqlf.Sprintf(
		`INSERT INTO
			secrets(source_type, source_id, value)
		VALUES($1, $2, $3)
		`, sourceType, sourceID, value)
	return s.insert(ctx, sqlQ)
}

// Update object id to value
func (s *secrets) Update(ctx context.Context, id int32, value string) error {
	sqlQ := sqlf.Sprintf(
		`UPDATE
			secrets
		SET
			value=$1
		WHERE
			id=$2
		`, value, id)

	res, err := dbconn.Global.ExecContext(ctx, sqlQ.Query(sqlf.PostgresBindVar), sqlQ.Args()...)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Update function for key-value pairs
func (s *secrets) UpdateByKeyName(ctx context.Context, keyName, value string) error {
	sqlQ := sqlf.Sprintf(
		`UPDATE
			secrets
		SET
			value=$1
		WHERE
			key_name=$1
		`, value, keyName)

	res, err := dbconn.Global.ExecContext(ctx, sqlQ.Query(sqlf.PostgresBindVar), sqlQ.Args()...)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *secrets) UpdateBySourceTypeAndID(ctx context.Context, sourceType string, sourceID int32, value string) error {
	sqlQ := sqlf.Sprintf(
		`UPDATE
			secrets
		SET
			value=$1
		WHERE
			source_type=$1 AND source_id=$2
		`, value, sourceType, sourceID)

	res, err := dbconn.Global.ExecContext(ctx, sqlQ.Query(sqlf.PostgresBindVar), sqlQ.Args()...)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
