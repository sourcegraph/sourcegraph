package resolvers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/graph-gophers/graphql-go"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/scip/bindings/go/scip"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/core"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/markdown"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type CodeNavServiceResolver interface {
	GitBlobLSIFData(ctx context.Context, args *GitBlobLSIFDataArgs) (GitBlobLSIFDataResolver, error)
	// CodeGraphData is a newer API that is more SCIP-oriented.
	// The second parameter is called 'opts' and not 'args' to reflect
	// that it is not what is exactly provided as input from the GraphQL
	// client.
	CodeGraphData(ctx context.Context, opts *CodeGraphDataOpts) (*[]CodeGraphDataResolver, error)
	// CodeGraphDataByID materializes a CodeGraphDataResolver purely from a graphql.ID.
	CodeGraphDataByID(ctx context.Context, id graphql.ID) (CodeGraphDataResolver, error)
	UsagesForSymbol(ctx context.Context, args *UsagesForSymbolArgs) (UsageConnectionResolver, error)
}

const CodeGraphDataIDKind = "CodeGraphData"

type GitBlobLSIFDataArgs struct {
	Repo      *types.Repo
	Commit    api.CommitID
	Path      string
	ExactPath bool
	ToolName  string
}

func (a *GitBlobLSIFDataArgs) Options() shared.UploadMatchingOptions {
	matching := shared.RootMustEnclosePath
	if !a.ExactPath {
		matching = shared.RootEnclosesPathOrPathEnclosesRoot
	}
	return shared.UploadMatchingOptions{
		RepositoryID: a.Repo.ID,
		Commit:       a.Commit,
		// OK to use Unchecked method since we expect a repo-root relative
		// path from the GraphQL API arguments; upload root relative paths
		// are largely an implementation detail.
		Path:               core.NewRepoRelPathUnchecked(a.Path),
		RootToPathMatching: matching,
		Indexer:            a.ToolName,
	}
}

type GitBlobLSIFDataResolver interface {
	GitTreeLSIFDataResolver
	ToGitTreeLSIFData() (GitTreeLSIFDataResolver, bool)
	ToGitBlobLSIFData() (GitBlobLSIFDataResolver, bool)

	Stencil(ctx context.Context) ([]RangeResolver, error)
	Ranges(ctx context.Context, args *LSIFRangesArgs) (CodeIntelligenceRangeConnectionResolver, error)
	Definitions(ctx context.Context, args *LSIFQueryPositionArgs) (LocationConnectionResolver, error)
	References(ctx context.Context, args *LSIFPagedQueryPositionArgs) (LocationConnectionResolver, error)
	Implementations(ctx context.Context, args *LSIFPagedQueryPositionArgs) (LocationConnectionResolver, error)
	Prototypes(ctx context.Context, args *LSIFPagedQueryPositionArgs) (LocationConnectionResolver, error)
	Hover(ctx context.Context, args *LSIFQueryPositionArgs) (HoverResolver, error)
	VisibleIndexes(ctx context.Context) (_ *[]PreciseIndexResolver, err error)
	Snapshot(ctx context.Context, args *struct{ IndexID graphql.ID }) (_ *[]SnapshotDataResolver, err error)
}

type SnapshotDataResolver interface {
	Offset() int32
	Data() string
	Additional() *[]string
}

type LSIFRangesArgs struct {
	StartLine int32
	EndLine   int32
}

type LSIFQueryPositionArgs struct {
	Line      int32
	Character int32
	Filter    *string
}

type LSIFPagedQueryPositionArgs struct {
	LSIFQueryPositionArgs
	PagedConnectionArgs
	Filter *string
}

type (
	CodeIntelligenceRangeConnectionResolver = ConnectionResolver[CodeIntelligenceRangeResolver]
)

type CodeIntelligenceRangeResolver interface {
	Range(ctx context.Context) (RangeResolver, error)
	Definitions(ctx context.Context) (LocationConnectionResolver, error)
	References(ctx context.Context) (LocationConnectionResolver, error)
	Implementations(ctx context.Context) (LocationConnectionResolver, error)
	Hover(ctx context.Context) (HoverResolver, error)
}

type RangeResolver interface {
	Start() PositionResolver
	End() PositionResolver
}

type PositionResolver interface {
	Line() int32
	Character() int32
}

type (
	LocationConnectionResolver = PagedConnectionResolver[LocationResolver]
)

type LocationResolver interface {
	Resource() GitTreeEntryResolver
	Range() RangeResolver
	URL(ctx context.Context) (string, error)
	CanonicalURL() string
}

type HoverResolver interface {
	Markdown() Markdown
	Range() RangeResolver
}

type Markdown string

func (m Markdown) Text() string {
	return string(m)
}

func (m Markdown) HTML() (string, error) {
	return markdown.Render(string(m))
}

type GitTreeLSIFDataResolver interface {
	Diagnostics(ctx context.Context, args *LSIFDiagnosticsArgs) (DiagnosticConnectionResolver, error)
}

type (
	LSIFDiagnosticsArgs          = ConnectionArgs
	DiagnosticConnectionResolver = PagedConnectionWithTotalCountResolver[DiagnosticResolver]
)

type DiagnosticResolver interface {
	Severity() (*string, error)
	Code() (*string, error)
	Source() (*string, error)
	Message() (*string, error)
	Location(ctx context.Context) (LocationResolver, error)
}

type CodeGraphDataResolver interface {
	// ID satisfies the Node interface.
	ID() graphql.ID
	Provenance(ctx context.Context) (CodeGraphDataProvenance, error)
	Commit(ctx context.Context) (string, error)
	ToolInfo(ctx context.Context) (*CodeGraphToolInfo, error)
	// Pre-condition: args are Normalized.
	Occurrences(ctx context.Context, args *OccurrencesArgs) (SCIPOccurrenceConnectionResolver, error)
}

// CodeGraphDataProvenance corresponds to the matching type in the GraphQL API.
//
// Make sure this type maintains its marshaling/unmarshaling behavior in
// case the type definition is changed.
type CodeGraphDataProvenance string

const (
	ProvenancePrecise     CodeGraphDataProvenance = "PRECISE"
	ProvenanceSyntactic   CodeGraphDataProvenance = "SYNTACTIC"
	ProvenanceSearchBased CodeGraphDataProvenance = "SEARCH_BASED"
)

type CodeGraphDataProvenanceComparator struct {
	Equals *CodeGraphDataProvenance
}

type CodeGraphDataFilter struct {
	Provenance *CodeGraphDataProvenanceComparator
}

// String is meant as a debugging-only representation without round-trippability
func (f *CodeGraphDataFilter) String() string {
	if f != nil && f.Provenance != nil && f.Provenance.Equals != nil {
		return fmt.Sprintf("provenance == %s", string(*f.Provenance.Equals))
	}
	return ""
}

// CodeGraphDataArgs represents the arguments to the codeGraphData(...)
// field on GitBlob in the GraphQL API.
//
// All fields are left public for JSON marshaling/unmarshaling.
type CodeGraphDataArgs struct {
	Filter *CodeGraphDataFilter
}

func (args *CodeGraphDataArgs) Attrs() []attribute.KeyValue {
	if args == nil {
		return nil
	}
	return []attribute.KeyValue{attribute.String("args.filter", args.Filter.String())}
}

type ForEachProvenance[T any] struct {
	SearchBased T
	Syntactic   T
	Precise     T
}

func (a *CodeGraphDataArgs) ProvenancesForSCIPData() ForEachProvenance[bool] {
	var out ForEachProvenance[bool]
	if a == nil || a.Filter == nil || a.Filter.Provenance == nil || a.Filter.Provenance.Equals == nil {
		out.Syntactic = true
		out.Precise = true
	} else {
		p := *a.Filter.Provenance.Equals
		switch p {
		case ProvenancePrecise:
			out.Precise = true
		case ProvenanceSyntactic:
			out.Syntactic = true
		case ProvenanceSearchBased:
		}
	}
	return out
}

type CodeGraphDataOpts struct {
	Args   *CodeGraphDataArgs
	Repo   *types.Repo
	Commit api.CommitID
	Path   core.RepoRelPath
}

func (opts *CodeGraphDataOpts) Attrs() []attribute.KeyValue {
	return append([]attribute.KeyValue{attribute.String("repo", opts.Repo.String()),
		opts.Commit.Attr(),
		attribute.String("path", opts.Path.RawValue())}, opts.Args.Attrs()...)
}

type CodeGraphToolInfo struct {
	Name_    *string
	Version_ *string
}

func (ti *CodeGraphToolInfo) Name() *string {
	return ti.Name_
}

func (ti *CodeGraphToolInfo) Version() *string {
	return ti.Version_
}

type OccurrencesArgs struct {
	First *int32
	After *string
}

// Normalize returns args for convenience of chaining
func (args *OccurrencesArgs) Normalize(maxPageSize int32) *OccurrencesArgs {
	if args == nil {
		*args = OccurrencesArgs{}
	}
	if args.First == nil || *args.First > maxPageSize {
		args.First = &maxPageSize
	}
	return args
}

type SCIPOccurrenceConnectionResolver interface {
	ConnectionResolver[SCIPOccurrenceResolver]
	PageInfo(ctx context.Context) (*graphqlutil.ConnectionPageInfo[SCIPOccurrenceResolver], error)
}

type SCIPOccurrenceResolver interface {
	Symbol() (*string, error)
	Range() (RangeResolver, error)
	Roles() (*[]SymbolRole, error)
}

type SymbolRole string

// ⚠️ CAUTION: These constants are part of the public GraphQL API
const (
	SymbolRoleDefinition        SymbolRole = "DEFINITION"
	SymbolRoleReference         SymbolRole = "REFERENCE"
	SymbolRoleForwardDefinition SymbolRole = "FORWARD_DEFINITION"
)

type UsagesForSymbolArgs struct {
	Symbol *SymbolComparator
	Range  RangeInput
	Filter *UsagesFilter
	First  *int32
	After  *string
}

// Resolve checks the well-formedness of args, and records the common information
// that will be needed by precise, syntactic and search-based code nav.
func (args *UsagesForSymbolArgs) Resolve(
	ctx context.Context,
	repoStore database.RepoStore,
	client gitserver.Client,
	maxPageSize int32,
) (out UsagesForSymbolResolvedArgs, err error) {
	// Resolve filtering/matching arguments.
	resolvedSymbol, err := args.Symbol.Resolve()
	if err != nil {
		return out, err
	}
	resolvedFilter, err := args.Filter.Resolve(ctx, repoStore)
	if err != nil {
		return out, err
	}

	// Resolve range related arguments.
	repo, err := repoStore.GetByName(ctx, api.RepoName(args.Range.Repository))
	if err != nil {
		return out, err
	}
	commitish := "HEAD"
	if rev := args.Range.Revision; rev != nil && *rev != "" {
		if _, err = api.NewCommitID(*rev); err != nil {
			return out, err
		}
		commitish = *rev
	}
	commitID, err := client.ResolveRevision(ctx, repo.Name, commitish, gitserver.ResolveRevisionOptions{})
	if err != nil {
		return out, err
	}
	if args.Range.Path == "" || path.IsAbs(args.Range.Path) {
		return out, errors.New("relative path to document containing one reference is required")
	}

	// Resolve pagination related arguments.
	remainingCount := maxPageSize
	if args.First != nil {
		remainingCount = min(max(*args.First, 0), maxPageSize)
	}
	var cursor UsagesCursor
	if args.After != nil {
		bytes, err := base64.StdEncoding.DecodeString(*args.After)
		if err != nil {
			return out, errors.Wrap(err, "invalid after: cursor")
		}
		if err = json.Unmarshal(bytes, &cursor); err != nil {
			return out, errors.Wrap(err, "invalid after: cursor")
		}
	} else {
		cursor.PreciseCursorType = DefinitionsCursor
	}

	scipRange, err := scip.NewRange([]int32{
		args.Range.Start.Line,
		args.Range.Start.Character,
		args.Range.End.Line,
		args.Range.End.Character,
	})
	if err != nil {
		return out, errors.Newf("invalid symbol range: %s", err)
	}

	return UsagesForSymbolResolvedArgs{
		resolvedSymbol,
		*repo,
		commitID,
		// OK to use Unchecked function as input path is expected to be relative
		// to repo root
		core.NewRepoRelPathUnchecked(args.Range.Path),
		scipRange,
		resolvedFilter,
		remainingCount,
		cursor,
	}, nil
}

type UsagesForSymbolResolvedArgs struct {
	// Symbol is either nil or all the fields are populated for the equality check.
	Symbol   *ResolvedSymbolComparator
	Repo     types.Repo
	CommitID api.CommitID
	Path     core.RepoRelPath
	Range    scip.Range
	Filter   *ResolvedUsagesFilter

	RemainingCount int32
	Cursor         UsagesCursor
}

type UsagesCursor struct {
	PreciseCursorType `json:"ty"`
	PreciseCursor     codenav.Cursor `json:"pc"`
}

type PreciseCursorType string

const (
	DefinitionsCursor     PreciseCursorType = "definitions"
	ImplementationsCursor PreciseCursorType = "implementations"
	PrototypesCursor      PreciseCursorType = "prototypes"
	ReferencesCursor      PreciseCursorType = "references"
)

func (args *UsagesForSymbolArgs) Attrs() (out []attribute.KeyValue) {
	out = append(append(args.Symbol.Attrs(), args.Range.Attrs()...), attribute.String("filter", args.Filter.DebugString()))
	if args.First != nil {
		out = append(out, attribute.Int("first", int(*args.First)))
	}
	out = append(out, attribute.Bool("hasAfter", args.After != nil))
	return out
}

type SymbolComparator struct {
	Name       SymbolNameComparator
	Provenance CodeGraphDataProvenanceComparator
}

func (c *SymbolComparator) Attrs() (out []attribute.KeyValue) {
	if c == nil {
		return nil
	}
	if c.Name.Equals != nil {
		out = append(out, attribute.String("symbol.name.equals", *c.Name.Equals))
	}
	if c.Provenance.Equals != nil {
		out = append(out, attribute.String("symbol.provenance.equals", string(*c.Provenance.Equals)))
	}
	return out
}

func (c *SymbolComparator) Resolve() (*ResolvedSymbolComparator, error) {
	if c == nil {
		return nil, nil
	}
	sym := c.Name.Equals
	prov := c.Provenance.Equals
	if sym == nil || prov == nil {
		return nil, errors.New("name.equals and provenance.equals must be specified if SymbolComparator is provided")
	}
	switch *prov {
	case ProvenancePrecise:
	case ProvenanceSyntactic:
	case ProvenanceSearchBased:
	default:
		return nil, errors.New("invalid provenance.equals")
	}
	if *sym == "" {
		return nil, errors.New("symbol name should be non-empty")
	}
	parsedSym, err := scip.ParseSymbol(*sym)
	if err != nil {
		return nil, errors.Wrap(err, "invalid symbol name")
	}
	return &ResolvedSymbolComparator{
		EqualsName:       *sym,
		EqualsProvenance: *prov,
		EqualsSymbol:     parsedSym,
	}, nil
}

type ResolvedSymbolComparator struct {
	EqualsName       string
	EqualsProvenance CodeGraphDataProvenance
	EqualsSymbol     *scip.Symbol
}

type ResolvedSymbolNameComparator struct {
	Equals       string
	EqualsSymbol scip.SymbolInformation
}

func (s *ResolvedSymbolComparator) ProvenancesForSCIPData() ForEachProvenance[bool] {
	var out ForEachProvenance[bool]
	if s == nil {
		out.Precise = true
		out.Syntactic = true
		out.SearchBased = true
	} else {
		switch s.EqualsProvenance {
		case ProvenancePrecise:
			out.Precise = true
		case ProvenanceSyntactic:
			out.Syntactic = true
		case ProvenanceSearchBased:
			out.SearchBased = true
		}
	}
	return out
}

type SymbolNameComparator struct {
	Equals *string
}

type RangeInput struct {
	Repository string
	Revision   *string
	Path       string
	Start      PositionInput
	End        PositionInput
}

func (r *RangeInput) Attrs() (out []attribute.KeyValue) {
	out = append(out, attribute.String("range.repository", r.Repository))
	if r.Revision != nil {
		out = append(out, attribute.String("range.revision", *r.Revision))
	}
	out = append(out, attribute.String("range.path", r.Path))
	out = append(out, attribute.Int("range.start.line", int(r.Start.Line)))
	out = append(out, attribute.Int("range.start.character", int(r.Start.Character)))
	out = append(out, attribute.Int("range.end.line", int(r.End.Line)))
	out = append(out, attribute.Int("range.end.character", int(r.End.Character)))
	return out
}

type PositionInput struct {
	// Zero-based line number
	Line int32
	// Zero-based UTF-16 code unit offset
	Character int32
}

type UsagesFilter struct {
	Not        *UsagesFilter
	Repository *RepositoryFilter
}

func (f *UsagesFilter) DebugString() string {
	if f == nil {
		return ""
	}
	result := []string{}
	if f.Not != nil {
		result = append(result, fmt.Sprintf("(not %s)", f.Not.DebugString()))
	}
	if f.Repository != nil && f.Repository.Name.Equals != nil {
		result = append(result, fmt.Sprintf("(repo == %s)", *f.Repository.Name.Equals))
	}
	return strings.Join(result, " and ")
}

func (f *UsagesFilter) Resolve(ctx context.Context, repoStore database.RepoStore) (*ResolvedUsagesFilter, error) {
	return resolveWithCache(ctx, repoStore, f, map[string]*types.Repo{})
}

func resolveWithCache(ctx context.Context, repoStore database.RepoStore, f *UsagesFilter, cache map[string]*types.Repo) (*ResolvedUsagesFilter, error) {
	if f == nil {
		return nil, nil
	}
	var repoFilter *ResolvedRepositoryFilter
	if f.Repository != nil && f.Repository.Name.Equals != nil {
		repoFilter = &ResolvedRepositoryFilter{}
		repoName := *f.Repository.Name.Equals
		if repoName == "" {
			return nil, errors.New("repository.name.equals should be non-empty; for no filtering, remove the repository field")
		}
		if cached, ok := cache[repoName]; ok {
			repoFilter.RepoEquals = *cached
		} else {
			uncached, err := repoStore.GetByName(ctx, api.RepoName(repoName))
			if err != nil {
				return nil, errors.Wrap(err, "unknown repo in filter")
			}
			repoFilter.RepoEquals = *uncached
			cache[repoName] = uncached
		}
		repoFilter.NameEquals = repoName
	}
	notFilter, err := resolveWithCache(ctx, repoStore, f.Not, cache) // recurse
	if err != nil {
		return nil, err
	}
	return &ResolvedUsagesFilter{notFilter, repoFilter}, nil
}

type ResolvedUsagesFilter struct {
	Not        *ResolvedUsagesFilter
	Repository *ResolvedRepositoryFilter
}

type ResolvedRepositoryFilter struct {
	NameEquals string
	// Resolved from above name
	RepoEquals types.Repo
}

type RepositoryFilter struct {
	Name StringComparator
}

type StringComparator struct {
	Equals *string
}

type UsageConnectionResolver = PagedConnectionResolver[UsageResolver]

type UsageResolver interface {
	Symbol(context.Context) (SymbolInformationResolver, error)
	Provenance(context.Context) (CodeGraphDataProvenance, error)
	DataSource() *string
	UsageRange(context.Context) (UsageRangeResolver, error)
	SurroundingContent(_ context.Context, args *struct {
		*SurroundingLines `json:"surroundingLines"`
	}) (*string, error)
	UsageKind() SymbolUsageKind
}

type SymbolInformationResolver interface {
	Name() (string, error)
	Documentation() (*[]string, error)
}

type UsageRangeResolver interface {
	Repository() string
	Revision() string
	Path() string
	Range() RangeResolver
}

type SurroundingLines struct {
	LinesBefore *int32 `json:"linesBefore"`
	LinesAfter  *int32 `json:"linesAfter"`
}

// SymbolUsageKind corresponds to the matching type in the GraphQL API.
//
// Make sure this type maintains its marshaling/unmarshaling behavior in
// case the type definition is changed.
type SymbolUsageKind string

const (
	UsageKindDefinition     SymbolUsageKind = "DEFINITION"
	UsageKindReference      SymbolUsageKind = "REFERENCE"
	UsageKindImplementation SymbolUsageKind = "IMPLEMENTATION"
	UsageKindSuper          SymbolUsageKind = "SUPER"
)
