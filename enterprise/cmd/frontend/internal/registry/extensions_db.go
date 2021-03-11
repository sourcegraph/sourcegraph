package registry

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/keegancsmith/sqlf"
	"github.com/pkg/errors"

	registry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
)

// dbExtension describes an extension in the extension registry.
//
// It is the internal form of github.com/sourcegraph/sourcegraph/internal/registry.Extension (which is
// the external API type). These types should generally be kept in sync, but registry.Extension
// updates require backcompat.
type dbExtension struct {
	ID        int32
	UUID      string
	Publisher dbPublisher
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time

	// NonCanonicalExtensionID is the denormalized fully qualified extension ID
	// ("[registry/]publisher/name" format), using the username/name of the extension's publisher
	// (joined from another table) as of when the query executed. Do not persist this, because the
	// (denormalized) registry and publisher names can change.
	//
	// If this value is obtained directly from a method on RegistryExtensions, this field will never
	// contain the registry name prefix (which is necessary to distinguish local extensions from
	// remote extensions). Call prefixLocalExtensionID to add it. The recommended way to apply this
	// automatically (when needed) is to use registry.GetExtensionByExtensionID instead of
	// (dbExtensions).GetByExtensionID.
	NonCanonicalExtensionID string

	// NonCanonicalRegistry is the denormalized registry name (as of when this field was set). This
	// field is only set by prefixLocalExtensionID and is always empty if this value is obtained
	// directly from a method on RegistryExtensions. Do not persist this value, because the
	// (denormalized) registry name can change.
	NonCanonicalRegistry string

	// NonCanonicalIsWorkInProgress is whether this extension was marked as a WIP extension when it
	// was fetched. This information comes from a separate table (registry_extension_releases, not
	// registry_extensions), so it is not canonical.
	NonCanonicalIsWorkInProgress bool
}

type dbExtensions struct{}

// extensionNotFoundError occurs when an extension is not found in the extension registry.
type extensionNotFoundError struct {
	args []interface{}
}

// NotFound implements errcode.NotFounder.
func (err extensionNotFoundError) NotFound() bool { return true }

func (err extensionNotFoundError) Error() string {
	return fmt.Sprintf("registry extension not found: %v", err.args)
}

// Create creates a new extension in the extension registry. Exactly 1 of publisherUserID and publisherOrgID must be nonzero.
func (s dbExtensions) Create(ctx context.Context, publisherUserID, publisherOrgID int32, name string) (id int32, err error) {
	if mocks.extensions.Create != nil {
		return mocks.extensions.Create(publisherUserID, publisherOrgID, name)
	}

	if publisherUserID != 0 && publisherOrgID != 0 {
		return 0, errors.New("at most 1 of the publisher user/org may be set")
	}

	uuid, err := uuid.NewRandom()
	if err != nil {
		return 0, err
	}

	if err := dbconn.Global.QueryRowContext(ctx,
		// Include users/orgs table query (with "FOR UPDATE") to ensure that the publisher user/org
		// not been deleted. If it was deleted, the query will return an error.
		`
INSERT INTO registry_extensions(uuid, publisher_user_id, publisher_org_id, name)
VALUES(
  $1,
  (SELECT id FROM users WHERE id=$2 AND deleted_at IS NULL FOR UPDATE),
  (SELECT id FROM orgs WHERE id=$3 AND deleted_at IS NULL FOR UPDATE),
  $4
)
RETURNING id
`,
		uuid, publisherUserID, publisherOrgID, name,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// GetByID retrieves the registry extension (if any) given its ID.
//
// 🚨 SECURITY: The caller must ensure that the actor is permitted to view this registry extension.
func (s dbExtensions) GetByID(ctx context.Context, id int32) (*dbExtension, error) {
	if mocks.extensions.GetByID != nil {
		return mocks.extensions.GetByID(id)
	}

	results, err := s.list(ctx, []*sqlf.Query{sqlf.Sprintf("x.id=%d", id)}, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, extensionNotFoundError{[]interface{}{id}}
	}
	return results[0], nil
}

// GetByUUID retrieves the registry extension (if any) given its UUID.
//
// 🚨 SECURITY: The caller must ensure that the actor is permitted to view this registry extension.
func (s dbExtensions) GetByUUID(ctx context.Context, uuid string) (*dbExtension, error) {
	if mocks.extensions.GetByUUID != nil {
		return mocks.extensions.GetByUUID(uuid)
	}

	results, err := s.list(ctx, []*sqlf.Query{sqlf.Sprintf("x.uuid=%d", uuid)}, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, extensionNotFoundError{[]interface{}{uuid}}
	}
	return results[0], nil
}

const (
	// extensionPublisherNameExpr is the SQL expression for the extension's publisher's name (using
	// the table aliases created by (dbExtensions).listCountSQL.
	extensionPublisherNameExpr = "COALESCE(users.username, orgs.name)"

	// extensionIDExpr is the SQL expression for the extension ID (using the table aliases created by
	// (dbExtensions).listCountSQL.
	extensionIDExpr = "CONCAT(" + extensionPublisherNameExpr + ", '/', x.name)"
)

// GetByExtensionID retrieves the registry extension (if any) given its extension ID, which is the
// concatenation of the publisher name, a slash ("/"), and the extension name.
//
// 🚨 SECURITY: The caller must ensure that the actor is permitted to view this registry extension.
func (s dbExtensions) GetByExtensionID(ctx context.Context, extensionID string) (*dbExtension, error) {
	if mocks.extensions.GetByExtensionID != nil {
		return mocks.extensions.GetByExtensionID(extensionID)
	}

	// TODO(sqs): prevent the creation of an org with the same name as a user so that there is no
	// ambiguity as to whether the publisher refers to a user or org by the given name
	// (https://github.com/sourcegraph/sourcegraph/issues/12068).
	parts := strings.SplitN(extensionID, "/", 2)
	if len(parts) < 2 {
		return nil, extensionNotFoundError{[]interface{}{fmt.Sprintf("extensionID %q", extensionID)}}
	}
	publisherName := parts[0]
	extensionName := parts[1]

	results, err := s.list(ctx, []*sqlf.Query{
		sqlf.Sprintf("x.name=%s", extensionName),
		sqlf.Sprintf("(users.username=%s OR orgs.name=%s)", publisherName, publisherName),
	}, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, extensionNotFoundError{[]interface{}{fmt.Sprintf("extensionID %q", extensionID)}}
	}
	return results[0], nil
}

// dbExtensionsListOptions contains options for listing registry extensions.
type dbExtensionsListOptions struct {
	Publisher              dbPublisher
	Query                  string // matches the extension ID and latest release's manifest's title
	Category               string // matches the latest release's manifest's categories array
	Tag                    string // matches the latest release's manifest's tags array
	PrioritizeExtensionIDs []string
	*database.LimitOffset
}

// extensionIsWIPExpr is the SQL expression for whether the extension is a WIP extension.
//
// BACKCOMPAT: It still reads the title property even though extensions no longer have titles.
var extensionIsWIPExpr = sqlf.Sprintf(`rer.manifest IS NULL OR COALESCE((rer.manifest->>'wip')::jsonb = 'true'::jsonb, rer.manifest->>'title' SIMILAR TO %s, false)`, registry.WorkInProgressExtensionTitlePostgreSQLPattern)

func (o dbExtensionsListOptions) sqlConditions() []*sqlf.Query {
	var conds []*sqlf.Query
	if o.Publisher.UserID != 0 {
		conds = append(conds, sqlf.Sprintf("x.publisher_user_id=%d", o.Publisher.UserID))
	}
	if o.Publisher.OrgID != 0 {
		conds = append(conds, sqlf.Sprintf("x.publisher_org_id=%d", o.Publisher.OrgID))
	}
	if o.Query != "" {
		likePattern := func(value string) string {
			return "%" + strings.Replace(strings.ToLower(value), " ", "%", -1) + "%"
		}
		queryConds := []*sqlf.Query{
			sqlf.Sprintf(extensionIDExpr+" ILIKE %s", likePattern(o.Query)),
			// BACKCOMPAT: This still reads the title property even though extensions no longer have titles.
			sqlf.Sprintf(`CASE WHEN rer.manifest IS NOT NULL THEN (rer.manifest->>'description' ILIKE %s OR rer.manifest->>'title' ILIKE %s) ELSE false END`, likePattern(o.Query), likePattern(o.Query)),
		}
		conds = append(conds, sqlf.Sprintf("(%s)", sqlf.Join(queryConds, ") OR (")))
	}
	if o.Category != "" {
		categoryConds := []*sqlf.Query{
			sqlf.Sprintf(`CASE WHEN rer.manifest IS NOT NULL THEN (rer.manifest->>'categories')::jsonb @> to_json(%s::text)::jsonb ELSE false END`, o.Category),
		}
		if o.Category == "Other" {
			// Special-case the "Other" category: it matches extensions explicitly categorized as
			// "Other" or extensions with a manifest with no category. (Extensions with no manifest
			// are omitted.) HACK: This ideally would be implemented at a different layer, but it is
			// so much simpler to just special-case it here.
			categoryConds = append(categoryConds, sqlf.Sprintf(`CASE WHEN rer.manifest IS NOT NULL THEN (rer.manifest->>'categories')::jsonb IS NULL ELSE false END`))
		}
		conds = append(conds, sqlf.Sprintf("(%s)", sqlf.Join(categoryConds, ") OR (")))
	}
	if o.Tag != "" {
		conds = append(conds, sqlf.Sprintf(`CASE WHEN rer.manifest IS NOT NULL THEN (rer.manifest->>'tags')::jsonb @> to_json(%s::text)::jsonb ELSE false END`, o.Tag))
	}
	if len(conds) == 0 {
		conds = append(conds, sqlf.Sprintf("TRUE"))
	}
	return conds
}

func (o dbExtensionsListOptions) sqlOrder() []*sqlf.Query {
	ids := make([]*sqlf.Query, len(o.PrioritizeExtensionIDs)+1)
	for i, id := range o.PrioritizeExtensionIDs {
		ids[i] = sqlf.Sprintf("%v", id)
	}
	ids[len(o.PrioritizeExtensionIDs)] = sqlf.Sprintf("NULL")
	return []*sqlf.Query{sqlf.Sprintf(extensionIDExpr+` IN (%v) ASC`, sqlf.Join(ids, ","))}
}

// List lists all registry extensions that satisfy the options.
//
// 🚨 SECURITY: The caller must ensure that the actor is permitted to list with the specified
// options.
func (s dbExtensions) List(ctx context.Context, opt dbExtensionsListOptions) ([]*dbExtension, error) {
	return s.list(ctx, opt.sqlConditions(), opt.sqlOrder(), opt.LimitOffset)
}

func (dbExtensions) listCountSQL(conds []*sqlf.Query) *sqlf.Query {
	return sqlf.Sprintf(`
FROM registry_extensions x
LEFT JOIN users ON users.id=publisher_user_id AND users.deleted_at IS NULL
LEFT JOIN orgs ON orgs.id=publisher_org_id AND orgs.deleted_at IS NULL
LEFT JOIN registry_extension_releases rer ON rer.registry_extension_id=x.id AND rer.deleted_at IS NULL
WHERE (%s)
  -- Join only to latest release from registry_extension_releases.
  AND NOT EXISTS (SELECT 1 FROM registry_extension_releases rer2
                  WHERE rer.registry_extension_id=rer2.registry_extension_id
                    AND rer2.deleted_at IS NULL
                    AND rer2.created_at > rer.created_at
  )
  AND x.deleted_at IS NULL`,
		sqlf.Join(conds, ") AND ("))
}

func (s dbExtensions) list(ctx context.Context, conds, order []*sqlf.Query, limitOffset *database.LimitOffset) ([]*dbExtension, error) {
	order = append(order, sqlf.Sprintf("TRUE"))
	q := sqlf.Sprintf(`
SELECT x.id, x.uuid, x.publisher_user_id, x.publisher_org_id, x.name, x.created_at, x.updated_at,
  `+extensionIDExpr+` AS non_canonical_extension_id, `+extensionPublisherNameExpr+` AS non_canonical_publisher_name,
  (%s) AS non_canonical_is_work_in_progress
%s
ORDER BY %s,
  -- Always sort WIP extensions last.
  (%s) ASC,
  x.id ASC
%s`,
		extensionIsWIPExpr,
		s.listCountSQL(conds),
		sqlf.Join(order, ","),
		extensionIsWIPExpr,
		limitOffset.SQL(),
	)

	rows, err := dbconn.Global.QueryContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*dbExtension
	for rows.Next() {
		var t dbExtension
		var publisherUserID, publisherOrgID sql.NullInt64
		if err := rows.Scan(&t.ID, &t.UUID, &publisherUserID, &publisherOrgID, &t.Name, &t.CreatedAt, &t.UpdatedAt, &t.NonCanonicalExtensionID, &t.Publisher.NonCanonicalName, &t.NonCanonicalIsWorkInProgress); err != nil {
			return nil, err
		}
		t.Publisher.UserID = int32(publisherUserID.Int64)
		t.Publisher.OrgID = int32(publisherOrgID.Int64)
		results = append(results, &t)
	}
	return results, nil
}

// Count counts all registry extensions that satisfy the options (ignoring limit and offset).
//
// 🚨 SECURITY: The caller must ensure that the actor is permitted to count the results.
func (s dbExtensions) Count(ctx context.Context, opt dbExtensionsListOptions) (int, error) {
	q := sqlf.Sprintf("SELECT COUNT(*) %s", s.listCountSQL(opt.sqlConditions()))
	var count int
	if err := dbconn.Global.QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Update updates information about the registry extension.
func (dbExtensions) Update(ctx context.Context, id int32, name *string) error {
	if mocks.extensions.Update != nil {
		return mocks.extensions.Update(id, name)
	}

	res, err := dbconn.Global.ExecContext(ctx,
		"UPDATE registry_extensions SET name=COALESCE($2, name), updated_at=now() WHERE id=$1 AND deleted_at IS NULL",
		id, name,
	)
	if err != nil {
		return err
	}
	nrows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if nrows == 0 {
		return extensionNotFoundError{[]interface{}{id}}
	}
	return nil
}

// Delete marks an registry extension as deleted.
func (dbExtensions) Delete(ctx context.Context, id int32) error {
	if mocks.extensions.Delete != nil {
		return mocks.extensions.Delete(id)
	}

	res, err := dbconn.Global.ExecContext(ctx, "UPDATE registry_extensions SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL", id)
	if err != nil {
		return err
	}
	nrows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if nrows == 0 {
		return extensionNotFoundError{[]interface{}{id}}
	}
	return nil
}

// mockExtensions mocks the registry extensions store.
type mockExtensions struct {
	Create           func(publisherUserID, publisherOrgID int32, name string) (int32, error)
	GetByID          func(id int32) (*dbExtension, error)
	GetByUUID        func(uuid string) (*dbExtension, error)
	GetByExtensionID func(extensionID string) (*dbExtension, error)
	Update           func(id int32, name *string) error
	Delete           func(id int32) error
}
