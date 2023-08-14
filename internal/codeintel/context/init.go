package context

import (
	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gosyntect"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func NewService(
	observationCtx *observation.Context,
	codeIntelDB codeintelshared.CodeIntelDB,
	repostore database.RepoStore,
	codenavSvc CodeNavService,
	syntectClient *gosyntect.Client,
	gitserverClient gitserver.Client,
) *Service {
	return newService(
		observationCtx,
		repostore,
		codenavSvc,
		syntectClient,
		gitserverClient,
	)
}

func scopedContext(component string, parent *observation.Context) *observation.Context {
	return observation.ScopedContext("codeintel", "context", component, parent)
}
