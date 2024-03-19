package repos

import (
	"context"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestGitoliteSource(t *testing.T) {
	gc := gitserver.NewMockClient()
	gc.ScopedFunc.SetDefaultReturn(gc)
	svc := typestest.MakeExternalService(t, extsvc.VariantGitolite, &schema.GitoliteConnection{})

	ctx := context.Background()
	s, err := NewGitoliteSource(ctx, svc, gc)
	if err != nil {
		t.Fatal(err)
	}

	res := make(chan SourceResult)
	go func() {
		s.ListRepos(ctx, res)
		close(res)
	}()

	for range res {
	}

	mockrequire.Called(t, gc.ListGitoliteReposFunc)
}
