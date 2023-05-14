package usagestats

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func GetAggregatedRepoMetadataStats(ctx context.Context, db database.DB) (*types.RepoMetadataAggregatedStats, error) {
	stats, err := db.EventLogs().AggregatedRepoMetadataStats(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	return stats, nil
}
