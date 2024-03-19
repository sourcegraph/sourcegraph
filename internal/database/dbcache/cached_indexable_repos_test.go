package dbcache

import (
	"context"
	"testing"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
)

func BenchmarkIndexableRepos_List_Empty(b *testing.B) {
	logger := logtest.Scoped(b)
	db := database.NewDB(logger, dbtest.NewDB(b))

	ctx := context.Background()
	select {
	case <-ctx.Done():
		b.Fatal("context already canceled")
	default:
	}
	b.ResetTimer()
	for range b.N {
		_, err := NewIndexableReposLister(logger, db.Repos()).List(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
