package store

import (
	"context"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
)

type ListCodeHostsOpts struct {
	RepoIDs []api.RepoID
}

func (s *Store) ListCodeHosts(ctx context.Context, opts ListCodeHostsOpts) ([]*batches.CodeHost, error) {
	q := listCodeHostsQuery(opts)

	cs := make([]*batches.CodeHost, 0)
	err := s.query(ctx, q, func(sc scanner) error {
		var c batches.CodeHost
		if err := scanCodeHost(&c, sc); err != nil {
			return err
		}
		cs = append(cs, &c)
		return nil
	})

	return cs, err
}

var listCodeHostsQueryFmtstr = `
-- source: enterprise/internal/batches/store_codehost.go:ListCodeHosts
SELECT
	repo.external_service_type, repo.external_service_id, COUNT(esr.external_service_id) > 0 AS ssh_required
FROM repo
LEFT JOIN external_service_repos esr
	ON
		esr.repo_id = repo.id AND
		(
			-- Either start with ssh://
			esr.clone_url ILIKE %s OR
			-- OR have no scheme specified at all
			esr.clone_url !~* '[[:alpha:]]+://'
		)
WHERE %s
GROUP BY repo.external_service_type, repo.external_service_id
ORDER BY repo.external_service_type ASC, repo.external_service_id ASC
`

func listCodeHostsQuery(opts ListCodeHostsOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		// Only for those which have any enabled repositories.
		sqlf.Sprintf("repo.deleted_at IS NULL"),
	}

	// Only show supported hosts.
	supportedTypes := []*sqlf.Query{}
	for extSvcType := range batches.SupportedExternalServices {
		supportedTypes = append(supportedTypes, sqlf.Sprintf("%s", extSvcType))
	}
	preds = append(preds, sqlf.Sprintf("repo.external_service_type IN (%s)", sqlf.Join(supportedTypes, ", ")))

	if len(opts.RepoIDs) > 0 {
		repoIDs := make([]*sqlf.Query, len(opts.RepoIDs))
		for i, id := range opts.RepoIDs {
			repoIDs[i] = sqlf.Sprintf("%s", id)
		}
		preds = append(preds, sqlf.Sprintf("repo.id IN (%s)", sqlf.Join(repoIDs, ",")))
	}

	return sqlf.Sprintf(listCodeHostsQueryFmtstr, sqlf.Sprintf("%s", "ssh://%"), sqlf.Join(preds, "AND"))
}

func scanCodeHost(c *batches.CodeHost, sc scanner) error {
	return sc.Scan(
		&c.ExternalServiceType,
		&c.ExternalServiceID,
		&c.RequiresSSH,
	)
}

type GetExternalServiceIDOpts struct {
	ExternalServiceType string
	ExternalServiceID   string
}

func (s *Store) GetExternalServiceID(ctx context.Context, opts GetExternalServiceIDOpts) (int64, error) {
	q := getExternalServiceIDQuery(opts)

	// Returns the first external service to match.
	id, ok, err := basestore.ScanFirstInt(s.Query(ctx, q))
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoResults
	}
	return int64(id), nil
}

const getExternalServiceIDQueryFmtstr = `
-- source: enterprise/internal/batches/store_codehost.go:GetExternalServiceID
SELECT
	external_services.id
FROM external_services
JOIN external_service_repos esr ON esr.external_service_id = external_services.id
JOIN repo ON esr.repo_id = repo.id
WHERE %s
ORDER BY external_services.id ASC
LIMIT 1
`

func getExternalServiceIDQuery(opts GetExternalServiceIDOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("repo.external_service_type = %s", opts.ExternalServiceType),
		sqlf.Sprintf("repo.external_service_id = %s", opts.ExternalServiceID),
		sqlf.Sprintf("external_services.deleted_at IS NULL"),
		sqlf.Sprintf("repo.deleted_at IS NULL"),
	}
	return sqlf.Sprintf(getExternalServiceIDQueryFmtstr, sqlf.Join(preds, "AND"))
}
