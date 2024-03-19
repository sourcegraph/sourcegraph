package graphql

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers/gitresolvers"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	sgtypes "github.com/sourcegraph/sourcegraph/internal/types"
)

func TestRanges(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()

	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	args := &resolverstubs.LSIFRangesArgs{StartLine: 10, EndLine: 20}
	if _, err := resolver.Ranges(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(mockCodeNavService.GetRangesFunc.History()) != 1 {
		t.Fatalf("unexpected call count. want=%d have=%d", 1, len(mockCodeNavService.GetRangesFunc.History()))
	}
	if val := mockCodeNavService.GetRangesFunc.History()[0].Arg3; val != 10 {
		t.Fatalf("unexpected start line. want=%d have=%d", 10, val)
	}
	if val := mockCodeNavService.GetRangesFunc.History()[0].Arg4; val != 20 {
		t.Fatalf("unexpected end line. want=%d have=%d", 20, val)
	}
}

func TestDefinitions(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()
	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	args := &resolverstubs.LSIFQueryPositionArgs{Line: 10, Character: 15}
	if _, err := resolver.Definitions(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(mockCodeNavService.GetDefinitionsFunc.History()) != 1 {
		t.Fatalf("unexpected call count. want=%d have=%d", 1, len(mockCodeNavService.GetDefinitionsFunc.History()))
	}
	if val := mockCodeNavService.GetDefinitionsFunc.History()[0].Arg1; val.Line != 10 {
		t.Fatalf("unexpected line. want=%v have=%v", 10, val)
	}
	if val := mockCodeNavService.GetDefinitionsFunc.History()[0].Arg1; val.Character != 15 {
		t.Fatalf("unexpected character. want=%d have=%v", 15, val)
	}
}

func TestReferences(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()
	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	offset := int32(25)
	mockRefCursor := codenav.Cursor{Phase: "local"}
	encodedCursor := encodeTraversalCursor(mockRefCursor)
	mockCursor := base64.StdEncoding.EncodeToString([]byte(encodedCursor))

	args := &resolverstubs.LSIFPagedQueryPositionArgs{
		LSIFQueryPositionArgs: resolverstubs.LSIFQueryPositionArgs{
			Line:      10,
			Character: 15,
		},
		PagedConnectionArgs: resolverstubs.PagedConnectionArgs{ConnectionArgs: resolverstubs.ConnectionArgs{First: &offset}, After: &mockCursor},
	}

	if _, err := resolver.References(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(mockCodeNavService.GetReferencesFunc.History()) != 1 {
		t.Fatalf("unexpected call count. want=%d have=%d", 1, len(mockCodeNavService.GetReferencesFunc.History()))
	}
	if val := mockCodeNavService.GetReferencesFunc.History()[0].Arg1; val.Line != 10 {
		t.Fatalf("unexpected line. want=%v have=%v", 10, val)
	}
	if val := mockCodeNavService.GetReferencesFunc.History()[0].Arg1; val.Character != 15 {
		t.Fatalf("unexpected character. want=%v have=%v", 15, val)
	}
	if val := mockCodeNavService.GetReferencesFunc.History()[0].Arg1; val.Limit != 25 {
		t.Fatalf("unexpected character. want=%v have=%v", 25, val)
	}
	if val := mockCodeNavService.GetReferencesFunc.History()[0].Arg1; val.RawCursor != encodedCursor {
		t.Fatalf("unexpected character. want=%v have=%v", "test-cursor", val)
	}
}

func TestReferencesDefaultLimit(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()
	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	args := &resolverstubs.LSIFPagedQueryPositionArgs{
		LSIFQueryPositionArgs: resolverstubs.LSIFQueryPositionArgs{
			Line:      10,
			Character: 15,
		},
		PagedConnectionArgs: resolverstubs.PagedConnectionArgs{},
	}

	if _, err := resolver.References(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(mockCodeNavService.GetReferencesFunc.History()) != 1 {
		t.Fatalf("unexpected call count. want=%d have=%d", 1, len(mockCodeNavService.GetDiagnosticsFunc.History()))
	}
	if val := mockCodeNavService.GetReferencesFunc.History()[0].Arg1; val.Limit != DefaultReferencesPageSize {
		t.Fatalf("unexpected limit. want=%v have=%v", DefaultReferencesPageSize, val)
	}
}

func TestReferencesDefaultIllegalLimit(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()
	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	offset := int32(-1)
	args := &resolverstubs.LSIFPagedQueryPositionArgs{
		LSIFQueryPositionArgs: resolverstubs.LSIFQueryPositionArgs{
			Line:      10,
			Character: 15,
		},
		PagedConnectionArgs: resolverstubs.PagedConnectionArgs{ConnectionArgs: resolverstubs.ConnectionArgs{First: &offset}},
	}

	if _, err := resolver.References(context.Background(), args); err != ErrIllegalLimit {
		t.Fatalf("unexpected error. want=%q have=%q", ErrIllegalLimit, err)
	}
}

func TestHover(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()
	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	mockCodeNavService.GetHoverFunc.SetDefaultReturn("text", shared.Range{}, true, nil)
	args := &resolverstubs.LSIFQueryPositionArgs{Line: 10, Character: 15}
	if _, err := resolver.Hover(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(mockCodeNavService.GetHoverFunc.History()) != 1 {
		t.Fatalf("unexpected call count. want=%d have=%d", 1, len(mockCodeNavService.GetHoverFunc.History()))
	}
	if val := mockCodeNavService.GetHoverFunc.History()[0].Arg1; val.Line != 10 {
		t.Fatalf("unexpected line. want=%v have=%v", 10, val)
	}
	if val := mockCodeNavService.GetHoverFunc.History()[0].Arg1; val.Character != 15 {
		t.Fatalf("unexpected character. want=%v have=%v", 15, val)
	}
}

func TestDiagnostics(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()
	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	offset := int32(25)
	args := &resolverstubs.LSIFDiagnosticsArgs{
		First: &offset,
	}

	if _, err := resolver.Diagnostics(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(mockCodeNavService.GetDiagnosticsFunc.History()) != 1 {
		t.Fatalf("unexpected call count. want=%d have=%d", 1, len(mockCodeNavService.GetDiagnosticsFunc.History()))
	}
	if val := mockCodeNavService.GetDiagnosticsFunc.History()[0].Arg1; val.Limit != 25 {
		t.Fatalf("unexpected limit. want=%v have=%v", 25, val)
	}
}

func TestDiagnosticsDefaultLimit(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()
	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	args := &resolverstubs.LSIFDiagnosticsArgs{}

	if _, err := resolver.Diagnostics(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(mockCodeNavService.GetDiagnosticsFunc.History()) != 1 {
		t.Fatalf("unexpected call count. want=%d have=%d", 1, len(mockCodeNavService.GetDiagnosticsFunc.History()))
	}
	if val := mockCodeNavService.GetDiagnosticsFunc.History()[0].Arg1; val.Limit != DefaultDiagnosticsPageSize {
		t.Fatalf("unexpected limit. want=%v have=%v", DefaultDiagnosticsPageSize, val)
	}
}

func TestDiagnosticsDefaultIllegalLimit(t *testing.T) {
	mockCodeNavService := NewMockCodeNavService()
	mockRequestState := codenav.RequestState{
		RepositoryID: 1,
		Commit:       "deadbeef1",
		Path:         "/src/main",
	}
	mockOperations := newOperations(&observation.TestContext)

	resolver := newGitBlobLSIFDataResolver(
		mockCodeNavService,
		nil,
		mockRequestState,
		nil,
		nil,
		nil,
		mockOperations,
	)

	offset := int32(-1)
	args := &resolverstubs.LSIFDiagnosticsArgs{
		First: &offset,
	}

	if _, err := resolver.Diagnostics(context.Background(), args); err != ErrIllegalLimit {
		t.Fatalf("unexpected error. want=%q have=%q", ErrIllegalLimit, err)
	}
}

func TestResolveLocations(t *testing.T) {
	repos := dbmocks.NewStrictMockRepoStore()
	repos.GetFunc.SetDefaultHook(func(_ context.Context, id api.RepoID) (*sgtypes.Repo, error) {
		return &sgtypes.Repo{ID: id, Name: api.RepoName(fmt.Sprintf("repo%d", id))}, nil
	})

	gsClient := gitserver.NewMockClient()
	gsClient.ResolveRevisionFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, spec string, _ gitserver.ResolveRevisionOptions) (api.CommitID, error) {
		if spec == "deadbeef3" {
			return "", &gitdomain.RevisionNotFoundError{}
		}
		return api.CommitID(spec), nil
	})

	factory := gitresolvers.NewCachedLocationResolverFactory(repos, gsClient)
	locationResolver := factory.Create()

	r1 := shared.Range{Start: shared.Position{Line: 11, Character: 12}, End: shared.Position{Line: 13, Character: 14}}
	r2 := shared.Range{Start: shared.Position{Line: 21, Character: 22}, End: shared.Position{Line: 23, Character: 24}}
	r3 := shared.Range{Start: shared.Position{Line: 31, Character: 32}, End: shared.Position{Line: 33, Character: 34}}
	r4 := shared.Range{Start: shared.Position{Line: 41, Character: 42}, End: shared.Position{Line: 43, Character: 44}}

	locations, err := resolveLocations(context.Background(), locationResolver, []shared.UploadLocation{
		{Upload: uploadsshared.CompletedUpload{RepositoryID: 50}, TargetCommit: "deadbeef1", TargetRange: r1, Path: "p1"},
		{Upload: uploadsshared.CompletedUpload{RepositoryID: 51}, TargetCommit: "deadbeef2", TargetRange: r2, Path: "p2"},
		{Upload: uploadsshared.CompletedUpload{RepositoryID: 52}, TargetCommit: "deadbeef3", TargetRange: r3, Path: "p3"},
		{Upload: uploadsshared.CompletedUpload{RepositoryID: 53}, TargetCommit: "deadbeef4", TargetRange: r4, Path: "p4"},
	})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	mockrequire.Called(t, repos.GetFunc)

	if len(locations) != 3 {
		t.Fatalf("unexpected length. want=%d have=%d", 3, len(locations))
	}
	if url := locations[0].CanonicalURL(); url != "/repo50@deadbeef1/-/blob/p1?L12:13-14:15" {
		t.Errorf("unexpected canonical url. want=%s have=%s", "/repo50@deadbeef1/-/blob/p1?L12:13-14:15", url)
	}
	if url := locations[1].CanonicalURL(); url != "/repo51@deadbeef2/-/blob/p2?L22:23-24:25" {
		t.Errorf("unexpected canonical url. want=%s have=%s", "/repo51@deadbeef2/-/blob/p2?L22:23-24:25", url)
	}
	if url := locations[2].CanonicalURL(); url != "/repo53@deadbeef4/-/blob/p4?L42:43-44:45" {
		t.Errorf("unexpected canonical url. want=%s have=%s", "/repo53@deadbeef4/-/blob/p4?L42:43-44:45", url)
	}
}
