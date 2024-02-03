package adminanalytics

import (
	"context"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database"
)

const query = `WITH names AS (SELECT name, lower(name) FROM event_logs WHERE user_id != 0 GROUP BY name ORDER BY LOWER(name) ASC) SELECT name FROM names`

func AllEventNames(ctx context.Context, db database.DB, cache bool) ([]*string, error) {
	cacheKey := "allEventNames"

	if cache {
		if allEventNames, err := getArrayFromCache[string](cacheKey); err == nil {
			return allEventNames, nil
		}
	}

	rows, err := db.QueryContext(ctx, sqlf.Sprintf(query).Query(sqlf.PostgresBindVar))

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	names := make([]*string, 0)
	for rows.Next() {
		var name string

		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		names = append(names, &name)
	}

	if err := setArrayToCache(cacheKey, names); err != nil {
		return nil, err
	}

	return names, nil
}
