package codenav

import (
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
)

var (
	testRange1 = shared.Range{Start: shared.Position{Line: 11, Character: 21}, End: shared.Position{Line: 31, Character: 41}}
	testRange2 = shared.Range{Start: shared.Position{Line: 12, Character: 22}, End: shared.Position{Line: 32, Character: 42}}
	testRange3 = shared.Range{Start: shared.Position{Line: 13, Character: 23}, End: shared.Position{Line: 33, Character: 43}}
	testRange4 = shared.Range{Start: shared.Position{Line: 14, Character: 24}, End: shared.Position{Line: 34, Character: 44}}
	testRange5 = shared.Range{Start: shared.Position{Line: 15, Character: 25}, End: shared.Position{Line: 35, Character: 45}}
	testRange6 = shared.Range{Start: shared.Position{Line: 16, Character: 26}, End: shared.Position{Line: 36, Character: 46}}

	mockPath   = repoRelPath("s1/main.go")
	mockCommit = api.CommitID("deadbeef")
)
