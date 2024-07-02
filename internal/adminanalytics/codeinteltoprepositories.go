package adminanalytics

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type CodeIntelTopRepositories struct {
	LangId_     string  `json:"langId"`
	Name_       string  `json:"name"`
	Events_     float64 `json:"events"`
	Kind_       string  `json:"kind"`
	Precision_  string  `json:"precision"`
	HasPrecise_ bool    `json:"hasPrecise"`
}

func (s *CodeIntelTopRepositories) Name() string      { return s.Name_ }
func (s *CodeIntelTopRepositories) Language() string  { return s.LangId_ }
func (s *CodeIntelTopRepositories) Events() float64   { return s.Events_ }
func (s *CodeIntelTopRepositories) Kind() string      { return s.Kind_ }
func (s *CodeIntelTopRepositories) Precision() string { return s.Precision_ }
func (s *CodeIntelTopRepositories) HasPrecise() bool  { return s.HasPrecise_ }

func GetCodeIntelTopRepositories(ctx context.Context, db database.DB, cache bool, dateRange string) ([]*CodeIntelTopRepositories, error) {
	cacheKey := fmt.Sprintf(`CodeIntelTopRepositories:%s`, dateRange)

	if cache {
		if nodes, err := getArrayFromCache[CodeIntelTopRepositories](cacheKey); err == nil {
			return nodes, nil
		}
	}

	now := time.Now()
	from, err := getFromDate(dateRange, now)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		WITH events AS (
			SELECT *
			FROM (
				SELECT
					(public_argument->>'repositoryId')::int AS repo_id,
					(argument->>'languageId')::text AS lang,
					(
						CASE
						WHEN name = 'blob.codeintel.lsifDefinitions.xrepo'                              THEN 'crossRepo'
						WHEN name = 'blob.codeintel.lsifDefinitions'                                    THEN 'precise'
						WHEN name = 'blob.codeintel.lsifReferences.xrepo'                               THEN 'crossRepo'
						WHEN name = 'blob.codeintel.lsifReferences'                                     THEN 'precise'
						WHEN name = 'blob.codeintel.searchDefinitions.xrepo'                            THEN 'crossRepo'
						WHEN name = 'blob.codeintel.searchReferences.xrepo'                             THEN 'crossRepo'
						WHEN name = 'blob.findReferences.executed'           AND source != 'server.web' THEN 'codeHost'
						WHEN name = 'blob.findReferences.executed'           AND source = 'server.web'  THEN 'inApp'
						WHEN name = 'blob.goToDefinition.preloaded.executed' AND source != 'server.web' THEN 'codeHost'
						WHEN name = 'blob.goToDefinition.preloaded.executed' AND source = 'server.web'  THEN 'inApp'
						WHEN name = 'blob.goToDefinition.executed'           AND source != 'server.web' THEN 'codeHost'
						WHEN name = 'blob.goToDefinition.executed'           AND source = 'server.web'  THEN 'inApp'
						WHEN name = 'blob.codeintel.searchDefinitions'                                  THEN 'inApp'
						ELSE NULL
						END
					) AS kind,
					name
				FROM event_logs
				WHERE timestamp BETWEEN $1 AND $2
			) AS _
			WHERE kind IS NOT NULL
		), top_repos AS (
			SELECT repo_id
			FROM events
			GROUP BY repo_id
			ORDER BY COUNT(1) DESC
			LIMIT 5
		)
		SELECT
			(SELECT repo.name FROM repo WHERE repo.id = repo_id) AS repo_name,
			lang,
			kind,
			(
				CASE
				WHEN name = 'blob.codeintel.lsifDefinitions.xrepo' THEN 'precise'
				WHEN name = 'blob.codeintel.lsifDefinitions'       THEN 'precise'
				WHEN name = 'blob.codeintel.lsifHover'             THEN 'precise'
				WHEN name = 'blob.codeintel.lsifReferences.xrepo'  THEN 'precise'
				WHEN name = 'blob.codeintel.lsifReferences'        THEN 'precise'
				ELSE                                               'search-based'
				END
			) AS precision,
			COUNT(1) AS count_,
			EXISTS (SELECT 1 FROM lsif_uploads WHERE repository_id = repo_id AND state = 'completed') AS has_precise
		FROM top_repos JOIN events USING (repo_id)
		GROUP BY repo_id, lang, kind, precision;
	`, from.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, errors.Wrap(err, "GetCodeIntelTopRepositories SQL query")
	}
	defer rows.Close()

	items := []*CodeIntelTopRepositories{}
	for rows.Next() {
		var item CodeIntelTopRepositories

		if err := rows.Scan(&item.Name_, &item.LangId_, &item.Kind_, &item.Precision_, &item.Events_, &item.HasPrecise_); err != nil {
			return nil, err
		}

		items = append(items, &item)
	}

	if err := setArrayToCache(cacheKey, items); err != nil {
		return nil, err
	}

	return items, nil
}
