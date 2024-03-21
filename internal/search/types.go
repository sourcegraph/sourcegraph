package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/regexp"
	"github.com/sourcegraph/zoekt"
	zoektquery "github.com/sourcegraph/zoekt/query"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/limits"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

// Inputs contains fields we set before kicking off search.
type Inputs struct {
	Plan                   query.Plan // the comprehensive query plan
	Query                  query.Q    // the current basic query being evaluated, one part of query.Plan
	OriginalQuery          string     // the raw string of the original search query
	SearchMode             Mode
	PatternType            query.SearchType
	UserSettings           *schema.Settings
	OnSourcegraphDotCom    bool
	Features               *Features
	Protocol               Protocol
	ContextLines           int32
	SanitizeSearchPatterns []*regexp.Regexp
}

// MaxResults computes the limit for the query.
func (inputs Inputs) MaxResults() int {
	return inputs.Query.MaxResults(inputs.DefaultLimit())
}

// DefaultLimit is the default limit to use if not specified in query.
func (inputs Inputs) DefaultLimit() int {
	switch inputs.Protocol {
	case Streaming:
		return limits.DefaultMaxSearchResultsStreaming
	case Batch:
		return limits.DefaultMaxSearchResults
	case Exhaustive:
		return limits.DefaultMaxSearchResultsExhaustive
	default:
		// Default to our normal interactive path
		return limits.DefaultMaxSearchResultsStreaming
	}
}

type Mode int

const (
	Precise     Mode = 0
	SmartSearch      = 1 << (iota - 1)
)

// Protocol encodes who the target client is and can be used to adjust default
// limits (or other behaviour changes) in the search code.
type Protocol int

const (
	// Streaming is our default interactive protocol. We use moderate default
	// limits to avoid doing unnecessary work.
	Streaming Protocol = iota
	// Batch needs to finish searching in an interactive time, so has limits
	// which are low.
	Batch
	// Exhaustive is run as a background job and as such has significantly
	// higher default limits.
	Exhaustive
)

func (p Protocol) String() string {
	switch p {
	case Streaming:
		return "Streaming"
	case Batch:
		return "Batch"
	case Exhaustive:
		return "Exhaustive"
	default:
		return fmt.Sprintf("unknown{%d}", p)
	}
}

type SymbolsParameters struct {
	// Repo is the name of the repository to search in.
	Repo api.RepoName `json:"repo"`

	// CommitID is the commit to search in.
	CommitID api.CommitID `json:"commitID"`

	// Query is the search query.
	Query string

	// IsRegExp if true will treat the Query as a regular expression.
	IsRegExp bool

	// IsCaseSensitive if false will ignore the case of query and file pattern
	// when finding matches.
	IsCaseSensitive bool

	// IncludePatterns is a list of regexes that symbol's file paths
	// need to match to get included in the result
	//
	// The patterns are ANDed together; a file's path must match all patterns
	// for it to be kept. That is also why it is a list (unlike the singular
	// ExcludePattern); it is not possible in general to construct a single
	// glob or Go regexp that represents multiple such patterns ANDed together.
	IncludePatterns []string

	// ExcludePattern is an optional regex that symbol's file paths
	// need to match to get included in the result
	ExcludePattern string

	// IncludeLangs and ExcludeLangs hold the language filters to apply.
	IncludeLangs []string
	ExcludeLangs []string

	// First indicates that only the first n symbols should be returned.
	First int

	// Timeout is the maximum amount of time the symbols search should take.
	//
	// If Timeout isn't specified, a default timeout of 60 seconds is used.
	Timeout time.Duration
}

type SymbolsResponse struct {
	Symbols result.Symbols `json:"symbols,omitempty"`
	Err     string         `json:"error,omitempty"`

	// LimitHit is true if the search results are incomplete due to limits
	// imposed by the service.
	LimitHit bool `json:"limitHit,omitempty"`
}

type IndexedRequestType string

const (
	TextRequest   IndexedRequestType = "text"
	SymbolRequest IndexedRequestType = "symbol"
)

// ZoektParameters contains all the inputs to run a Zoekt indexed search.
type ZoektParameters struct {
	Query           zoektquery.Q
	Typ             IndexedRequestType
	FileMatchLimit  int32
	Select          filter.SelectPath
	NumContextLines int

	// Features are feature flags that can affect behaviour of searcher.
	Features Features

	PatternType query.SearchType
}

// ToSearchOptions converts the parameters to options for the Zoekt search API.
func (o *ZoektParameters) ToSearchOptions(ctx context.Context) (searchOpts *zoekt.SearchOptions) {
	if o.Features.ZoektSearchOptionsOverride != "" {
		defer func() {
			old := *searchOpts
			err := json.Unmarshal([]byte(o.Features.ZoektSearchOptionsOverride), searchOpts)
			if err != nil {
				searchOpts = &old
			}
		}()
	}

	defaultTimeout := 20 * time.Second
	searchOpts = &zoekt.SearchOptions{
		Trace:             policy.ShouldTrace(ctx),
		MaxWallTime:       defaultTimeout,
		ChunkMatches:      true,
		UseKeywordScoring: o.PatternType == query.SearchTypeCodyContext,
		NumContextLines:   o.NumContextLines,
	}

	// These are reasonable default amounts of work to do per shard and
	// replica respectively.
	searchOpts.ShardMaxMatchCount = 10_000
	searchOpts.TotalMaxMatchCount = 100_000
	// KeywordScoring and Features.UseZoektParser represent different approaches we
	// are evaluating to deliver a better keyword-based search experience. For now
	// these are separate, but we might combine them in the future. Both profit from
	// higher defaults.
	if searchOpts.UseKeywordScoring || o.PatternType == query.SearchTypeKeyword {
		// Keyword searches tends to match much more broadly than code searches, so we need to
		// consider more candidates to ensure we don't miss highly-ranked documents
		searchOpts.ShardMaxMatchCount *= 10
		searchOpts.TotalMaxMatchCount *= 10
	}

	// Tell each zoekt replica to not send back more than limit results.
	limit := int(o.FileMatchLimit)
	searchOpts.MaxDocDisplayCount = limit

	// If we are searching for large limits, raise the amount of work we
	// are willing to do per shard and zoekt replica respectively.
	if limit > searchOpts.ShardMaxMatchCount {
		searchOpts.ShardMaxMatchCount = limit
	}
	if limit > searchOpts.TotalMaxMatchCount {
		searchOpts.TotalMaxMatchCount = limit
	}

	// If we're searching repos, ignore the other options and only check one file per repo
	if o.Select.Root() == filter.Repository {
		searchOpts.ShardRepoMaxMatchCount = 1
		return searchOpts
	}

	if o.Features.Debug {
		searchOpts.DebugScore = true
	}

	// This enables our stream based ranking, where we wait a certain amount
	// of time to collect results before ranking.
	searchOpts.FlushWallTime = conf.SearchFlushWallTime(searchOpts.UseKeywordScoring)

	// Only use document ranks if the jobs to calculate the ranks are enabled. This
	// is to make sure we don't use outdated ranks for scoring in Zoekt.
	searchOpts.UseDocumentRanks = conf.CodeIntelRankingDocumentReferenceCountsEnabled()
	searchOpts.DocumentRanksWeight = conf.SearchDocumentRanksWeight()

	return searchOpts
}

// SearcherParameters the inputs for a search fulfilled by the Searcher service
// (cmd/searcher). Searcher fulfills (1) unindexed literal and regexp searches
// and (2) structural search requests.
type SearcherParameters struct {
	PatternInfo *TextPatternInfo

	// UseFullDeadline indicates that the search should try do as much work as
	// it can within context.Deadline. If false the search should try and be
	// as fast as possible, even if a "slow" deadline is set.
	//
	// For example searcher will wait to full its archive cache for a
	// repository if this field is true. Another example is we set this field
	// to true if the user requests a specific timeout or maximum result size.
	UseFullDeadline bool

	// Features are feature flags that can affect behaviour of searcher.
	Features Features

	NumContextLines int
}

// TextPatternInfo defines the search request for unindexed and structural search
// (the 'searcher' service). Keep it in sync with pkg/searcher/protocol.PatternInfo.
type TextPatternInfo struct {
	// Query defines the search query
	Query protocol.QueryNode

	// Parameters for the search
	IsStructuralPat bool
	CombyRule       string
	IsCaseSensitive bool
	FileMatchLimit  int32
	Index           query.YesNoOnly
	Select          filter.SelectPath

	IncludePaths []string
	ExcludePaths string

	IncludeLangs []string
	ExcludeLangs []string

	PathPatternsAreCaseSensitive bool

	PatternMatchesContent bool
	PatternMatchesPath    bool

	// Languages is only used for structural search, and is separate from IncludeLangs above
	// TODO: remove this once the 'search-content-based-lang-detection' feature is enabled by default
	Languages []string
}

func (p *TextPatternInfo) Fields() []attribute.KeyValue {
	res := make([]attribute.KeyValue, 0, 4)
	add := func(fs ...attribute.KeyValue) {
		res = append(res, fs...)
	}

	add(attribute.Stringer("query", p.Query))

	if p.IsStructuralPat {
		add(attribute.Bool("isStructural", p.IsStructuralPat))
	}
	if p.CombyRule != "" {
		add(attribute.String("combyRule", p.CombyRule))
	}
	if p.IsCaseSensitive {
		add(attribute.Bool("isCaseSensitive", p.IsCaseSensitive))
	}
	add(attribute.Int("fileMatchLimit", int(p.FileMatchLimit)))

	if p.Index != query.Yes {
		add(attribute.String("index", string(p.Index)))
	}
	if len(p.Select) > 0 {
		add(attribute.StringSlice("select", p.Select))
	}
	if len(p.IncludePaths) > 0 {
		add(attribute.StringSlice("includePatterns", p.IncludePaths))
	}
	if p.ExcludePaths != "" {
		add(attribute.String("excludePattern", p.ExcludePaths))
	}
	if p.PathPatternsAreCaseSensitive {
		add(attribute.Bool("pathPatternsAreCaseSensitive", p.PathPatternsAreCaseSensitive))
	}
	if p.PatternMatchesPath {
		add(attribute.Bool("patternMatchesPath", p.PatternMatchesPath))
	}
	if len(p.Languages) > 0 {
		add(attribute.StringSlice("languages", p.Languages))
	}
	return res
}

func (p *TextPatternInfo) String() string {
	args := []string{p.Query.String()}

	if p.IsStructuralPat {
		if p.CombyRule != "" {
			args = append(args, fmt.Sprintf("comby:%s", p.CombyRule))
		} else {
			args = append(args, "comby")
		}
	}
	if p.IsCaseSensitive {
		args = append(args, "case")
	}
	if !p.PatternMatchesContent {
		args = append(args, "nocontent")
	}
	if !p.PatternMatchesPath {
		args = append(args, "nopath")
	}
	if p.FileMatchLimit > 0 {
		args = append(args, fmt.Sprintf("filematchlimit:%d", p.FileMatchLimit))
	}
	for _, lang := range p.Languages {
		args = append(args, fmt.Sprintf("lang:%s", lang))
	}

	path := "f"
	if p.PathPatternsAreCaseSensitive {
		path = "F"
	}
	if p.ExcludePaths != "" {
		args = append(args, fmt.Sprintf("-%s:%q", path, p.ExcludePaths))
	}
	for _, inc := range p.IncludePaths {
		args = append(args, fmt.Sprintf("%s:%q", path, inc))
	}

	return fmt.Sprintf("TextPatternInfo{%s}", strings.Join(args, ","))
}

// Features describe feature flags for a request. This is state that differs
// across users and time. It is created based on user feature flags and
// configuration.
//
// The Feature struct should be initialized once per search request early on.
//
// The default value for a Feature should be the go zero value, such that
// creating an empty Feature struct represents the usual search
// experience. This is to avoid needing to update a large number of tests when
// a new feature flag is introduced, and instead changes are localized to this
// struct and read sites of a flag.
type Features struct {
	// ContentBasedLangFilters when true will use the language detected from
	// the content of the file, rather than just file name patterns. This is
	// currently just supported by Zoekt.
	ContentBasedLangFilters bool `json:"search-content-based-lang-detection"`

	// Debug when true will set the Debug field on FileMatches. This may grow
	// from here. For now we treat this like a feature flag for convenience.
	Debug bool `json:"debug"`

	// ZoektSearchOptionsOverride is a JSON string that overrides the Zoekt search
	// options. This should be used for quick interactive experiments only. An
	// invalid JSON string or unknown fields will be ignored.
	ZoektSearchOptionsOverride string
}

func (f *Features) String() string {
	jsonObject, err := json.Marshal(f)
	if err != nil {
		return "error encoding features as string"
	}
	flagMap := featureflag.EvaluatedFlagSet{}
	if err := json.Unmarshal(jsonObject, &flagMap); err != nil {
		return "error decoding features"
	}
	return flagMap.String()
}

// RepoOptions is the source of truth for the options a user specified
// in their search query that affect which repos should be searched.
// When adding fields to this struct, be sure to update IsGlobal().
type RepoOptions struct {
	RepoFilters         []query.ParsedRepoFilter
	MinusRepoFilters    []string
	DescriptionPatterns []string

	CaseSensitiveRepoFilters bool
	SearchContextSpec        string

	CommitAfter *query.RepoHasCommitAfterArgs
	Visibility  query.RepoVisibility
	Limit       int
	Cursors     []*types.Cursor

	// Whether we should depend on Zoekt for resolving repositories
	UseIndex       query.YesNoOnly
	HasFileContent []query.RepoHasFileContentArgs
	HasKVPs        []query.RepoKVPFilter
	HasTopics      []query.RepoHasTopicPredicate

	// ForkSet indicates whether `fork:` was set explicitly in the query,
	// or whether the values were set from defaults.
	ForkSet   bool
	NoForks   bool
	OnlyForks bool

	OnlyCloned bool

	// ArchivedSet indicates whether `archived:` was set explicitly in the query,
	// or whether the values were set from defaults.
	ArchivedSet  bool
	NoArchived   bool
	OnlyArchived bool
}

func (op *RepoOptions) Attributes() []attribute.KeyValue {
	res := make([]attribute.KeyValue, 0, 8)
	add := func(f ...attribute.KeyValue) {
		res = append(res, f...)
	}

	if len(op.RepoFilters) > 0 {
		add(attribute.String("repoFilters", fmt.Sprintf("%v", op.RepoFilters)))
	}
	if len(op.MinusRepoFilters) > 0 {
		add(attribute.StringSlice("minusRepoFilters", op.MinusRepoFilters))
	}
	if len(op.DescriptionPatterns) > 0 {
		add(attribute.StringSlice("descriptionPatterns", op.DescriptionPatterns))
	}
	if op.CaseSensitiveRepoFilters {
		add(attribute.Bool("caseSensitiveRepoFilters", true))
	}
	if op.SearchContextSpec != "" {
		add(attribute.String("searchContextSpec", op.SearchContextSpec))
	}
	if op.CommitAfter != nil {
		add(attribute.String("commitAfter.time", op.CommitAfter.TimeRef))
		add(attribute.Bool("commitAfter.negated", op.CommitAfter.Negated))
	}
	if op.Visibility != query.Any {
		add(attribute.String("visibility", string(op.Visibility)))
	}
	if op.Limit > 0 {
		add(attribute.Int("limit", op.Limit))
	}
	if len(op.Cursors) > 0 {
		add(attribute.String("cursors", fmt.Sprintf("%+v", op.Cursors)))
	}
	if op.UseIndex != query.Yes {
		add(attribute.String("useIndex", string(op.UseIndex)))
	}
	if len(op.HasFileContent) > 0 {
		for i, arg := range op.HasFileContent {
			nondefault := []attribute.KeyValue{}
			if arg.Path != "" {
				nondefault = append(nondefault, attribute.String("path", arg.Path))
			}
			if arg.Content != "" {
				nondefault = append(nondefault, attribute.String("content", arg.Content))
			}
			if arg.Negated {
				nondefault = append(nondefault, attribute.Bool("negated", arg.Negated))
			}
			add(trace.Scoped(fmt.Sprintf("hasFileContent[%d]", i), nondefault...)...)
		}
	}
	if len(op.HasKVPs) > 0 {
		for i, arg := range op.HasKVPs {
			nondefault := []attribute.KeyValue{}
			if arg.Key != "" {
				nondefault = append(nondefault, attribute.String("key", arg.Key))
			}
			if arg.Value != nil {
				nondefault = append(nondefault, attribute.String("value", *arg.Value))
			}
			if arg.Negated {
				nondefault = append(nondefault, attribute.Bool("negated", arg.Negated))
			}
			add(trace.Scoped(fmt.Sprintf("hasKVPs[%d]", i), nondefault...)...)
		}
	}
	if len(op.HasTopics) > 0 {
		for i, arg := range op.HasTopics {
			nondefault := []attribute.KeyValue{}
			if arg.Topic != "" {
				nondefault = append(nondefault, attribute.String("topic", arg.Topic))
			}
			if arg.Negated {
				nondefault = append(nondefault, attribute.Bool("negated", arg.Negated))
			}
			add(trace.Scoped(fmt.Sprintf("hasTopics[%d]", i), nondefault...)...)
		}
	}
	if op.ForkSet {
		add(attribute.Bool("forkSet", op.ForkSet))
	}
	if !op.NoForks { // default value is true
		add(attribute.Bool("noForks", op.NoForks))
	}
	if op.OnlyForks {
		add(attribute.Bool("onlyForks", op.OnlyForks))
	}
	if op.OnlyCloned {
		add(attribute.Bool("onlyCloned", op.OnlyCloned))
	}
	if op.ArchivedSet {
		add(attribute.Bool("archivedSet", op.ArchivedSet))
	}
	if !op.NoArchived { // default value is true
		add(attribute.Bool("noArchived", op.NoArchived))
	}
	if op.OnlyArchived {
		add(attribute.Bool("onlyArchived", op.OnlyArchived))
	}
	return res
}

func (op *RepoOptions) String() string {
	var b strings.Builder

	if len(op.RepoFilters) > 0 {
		fmt.Fprintf(&b, "RepoFilters: %q\n", op.RepoFilters)
	} else {
		b.WriteString("RepoFilters: []\n")
	}
	if len(op.MinusRepoFilters) > 0 {
		fmt.Fprintf(&b, "MinusRepoFilters: %q\n", op.MinusRepoFilters)
	} else {
		b.WriteString("MinusRepoFilters: []\n")
	}

	if len(op.DescriptionPatterns) > 0 {
		fmt.Fprintf(&b, "DescriptionPatterns: %q\n", op.DescriptionPatterns)
	}

	if op.CommitAfter != nil {
		fmt.Fprintf(&b, "CommitAfter: %s\n", op.CommitAfter.TimeRef)
	}
	fmt.Fprintf(&b, "Visibility: %s\n", string(op.Visibility))

	if op.UseIndex != query.Yes {
		fmt.Fprintf(&b, "UseIndex: %s\n", string(op.UseIndex))
	}
	if len(op.HasFileContent) > 0 {
		for i, arg := range op.HasFileContent {
			if arg.Path != "" {
				fmt.Fprintf(&b, "HasFileContent[%d].path: %s\n", i, arg.Path)
			}
			if arg.Content != "" {
				fmt.Fprintf(&b, "HasFileContent[%d].content: %s\n", i, arg.Content)
			}
			if arg.Negated {
				fmt.Fprintf(&b, "HasFileContent[%d].negated: %t\n", i, arg.Negated)
			}
		}
	}
	if len(op.HasKVPs) > 0 {
		for i, arg := range op.HasKVPs {
			if arg.Key != "" {
				fmt.Fprintf(&b, "HasKVPs[%d].key: %s\n", i, arg.Key)
			}
			if arg.Value != nil {
				fmt.Fprintf(&b, "HasKVPs[%d].value: %s\n", i, *arg.Value)
			}
			if arg.Negated {
				fmt.Fprintf(&b, "HasKVPs[%d].negated: %t\n", i, arg.Negated)
			}
		}
	}
	if len(op.HasTopics) > 0 {
		for i, arg := range op.HasTopics {
			if arg.Topic != "" {
				fmt.Fprintf(&b, "HasTopics[%d].topic: %s\n", i, arg.Topic)
			}
			if arg.Negated {
				fmt.Fprintf(&b, "HasTopics[%d].negated: %t\n", i, arg.Negated)
			}
		}
	}

	if op.CaseSensitiveRepoFilters {
		fmt.Fprintf(&b, "CaseSensitiveRepoFilters: %t\n", op.CaseSensitiveRepoFilters)
	}
	if op.ForkSet {
		fmt.Fprintf(&b, "ForkSet: %t\n", op.ForkSet)
	}
	if op.NoForks {
		fmt.Fprintf(&b, "NoForks: %t\n", op.NoForks)
	}
	if op.OnlyForks {
		fmt.Fprintf(&b, "OnlyForks: %t\n", op.OnlyForks)
	}
	if op.OnlyCloned {
		fmt.Fprintf(&b, "OnlyCloned: %t\n", op.OnlyCloned)
	}
	if op.ArchivedSet {
		fmt.Fprintf(&b, "ArchivedSet: %t\n", op.ArchivedSet)
	}
	if op.NoArchived {
		fmt.Fprintf(&b, "NoArchived: %t\n", op.NoArchived)
	}
	if op.OnlyArchived {
		fmt.Fprintf(&b, "OnlyArchived: %t\n", op.OnlyArchived)
	}

	return b.String()
}
