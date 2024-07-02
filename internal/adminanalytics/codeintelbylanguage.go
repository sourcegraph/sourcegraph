package adminanalytics

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/database"
)

type CodeIntelByLanguage struct {
	Language_  string  `json:"language"`
	Precision_ string  `json:"precision"`
	Count_     float64 `json:"count"`
}

func (s *CodeIntelByLanguage) Language() string  { return s.Language_ }
func (s *CodeIntelByLanguage) Precision() string { return s.Precision_ }
func (s *CodeIntelByLanguage) Count() float64    { return s.Count_ }

func GetCodeIntelByLanguage(ctx context.Context, db database.DB, cache bool, dateRange string) ([]*CodeIntelByLanguage, error) {
	cacheKey := fmt.Sprintf(`CodeIntelByLanguage:%s`, dateRange)

	if cache {
		if nodes, err := getArrayFromCache[CodeIntelByLanguage](cacheKey); err == nil {
			return nodes, nil
		}
	}

	now := time.Now()
	from, err := getFromDate(dateRange, now)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT language, precision, COUNT(*) AS count
		FROM (
			SELECT argument->>'languageId' AS language, CASE WHEN name LIKE '%search%' THEN 'search-based' ELSE 'precise' END AS precision
			FROM event_logs
			WHERE
				timestamp BETWEEN $1 AND $2 AND
				name IN (
					'blob.codeintel.searchDefinitions',
					'blob.codeintel.searchDefinitions.xrepo',
					'blob.codeintel.searchReferences',
					'blob.codeintel.searchReferences.xrepo',
					'blob.codeintel.lsifDefinitions',
					'blob.codeintel.lsifDefinitions.xrepo',
					'blob.codeintel.lsifReferences',
					'blob.codeintel.lsifReferences.xrepo'
				)
		) sub
		GROUP BY language, precision;
	`, from.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*CodeIntelByLanguage{}
	for rows.Next() {
		var item CodeIntelByLanguage

		if err := rows.Scan(&item.Language_, &item.Precision_, &item.Count_); err != nil {
			return nil, err
		}

		items = append(items, &item)
	}

	if err := setArrayToCache(cacheKey, items); err != nil {
		return nil, err
	}

	return items, nil
}
