// Package protocol contains structures used by the searcher API.
package protocol

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/sourcegraph/sourcegraph/internal/api"
	proto "github.com/sourcegraph/sourcegraph/internal/searcher/v1"
)

// Request represents a request to searcher
type Request struct {
	// Repo is the name of the repository to search. eg "github.com/gorilla/mux"
	Repo api.RepoName

	// RepoID is the Sourcegraph repository id of the repo to search.
	RepoID api.RepoID

	// Commit is which commit to search. It is required to be resolved,
	// not a ref like HEAD or master. eg
	// "599cba5e7b6137d46ddf58fb1765f5d928e69604"
	Commit api.CommitID

	// Branch is used for structural search as an alternative to Commit
	// because Zoekt only takes branch names
	Branch string

	PatternInfo

	// The amount of time to wait for a repo archive to fetch.
	// It is parsed with time.ParseDuration.
	//
	// This timeout should be low when searching across many repos
	// so that unfetched repos don't delay the search, and because we are likely
	// to get results from the repos that have already been fetched.
	//
	// This timeout should be high when searching across a single repo
	// because returning results slowly is better than returning no results at all.
	//
	// This only times out how long we wait for the fetch request;
	// the fetch will still happen in the background so future requests don't have to wait.
	FetchTimeout time.Duration

	// Whether the revision to be searched is indexed or unindexed. This matters for
	// structural search because it will query Zoekt for indexed structural search.
	Indexed bool

	NumContextLines int32
}

// PatternInfo describes a search request on a repo.
type PatternInfo struct {
	// Query defines the search query. It supports regexp patterns optionally
	// combined through boolean operators.
	Query QueryNode

	// IsStructuralPat if true will treat the pattern as a Comby structural search pattern.
	IsStructuralPat bool

	// IsCaseSensitive if false will ignore the case of text and pattern
	// when finding matches.
	IsCaseSensitive bool

	// ExcludePaths is a pattern that may not match the returned files' paths.
	// eg '**/node_modules'
	ExcludePaths string

	// IncludePaths is a list of patterns that must *all* match the returned
	// files' paths.
	// eg '**/node_modules'
	//
	// The patterns are ANDed together; a file's path must match all patterns
	// for it to be kept. That is also why it is a list (unlike the singular
	// ExcludePaths); it is not possible in general to construct a single
	// glob or Go regexp that represents multiple such patterns ANDed together.
	IncludePaths []string

	// IncludeLangs and ExcludeLangs are the languages passed via the lang filters (e.g., "lang:c")
	IncludeLangs []string
	ExcludeLangs []string

	// IncludeExcludePatternAreCaseSensitive indicates that ExcludePaths, IncludePattern,
	// and IncludePaths are case sensitive.
	PathPatternsAreCaseSensitive bool

	// Limit is the cap on the total number of matches returned.
	// A match is either a path match, or a fragment of a line matched by the query.
	Limit int

	// PatternMatchesPath is whether the pattern should be matched against the content
	// of files.
	PatternMatchesContent bool

	// PatternMatchesPath is whether a file whose path matches Query (but whose contents don't) should be
	// considered a match.
	PatternMatchesPath bool

	// CombyRule is a rule that constrains matching for structural search.
	// It only applies when IsStructuralPat is true.
	// As a temporary measure, the expression `where "backcompat" == "backcompat"` acts as
	// a flag to activate the old structural search path, which queries zoekt for the
	// file list in the frontend and passes it to searcher.
	CombyRule string

	// Select is the value of the the select field in the query. It is not necessary to
	// use it since selection is done after the query completes, but exposing it can enable
	// optimizations.
	Select string

	// Languages represents the set of languages requested in the query. It is only used for
	// structural search and is separate from IncludeLangs, which represents language filters.
	Languages []string
}

func (p *PatternInfo) String() string {
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
	if p.Limit > 0 {
		args = append(args, fmt.Sprintf("limit:%d", p.Limit))
	}
	for _, lang := range p.IncludeLangs {
		args = append(args, fmt.Sprintf("lang:%s", lang))
	}
	for _, lang := range p.ExcludeLangs {
		args = append(args, fmt.Sprintf("-lang:%s", lang))
	}
	if p.Select != "" {
		args = append(args, fmt.Sprintf("select:%s", p.Select))
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

	return fmt.Sprintf("PatternInfo{%s}", strings.Join(args, ","))
}

func (r *Request) ToProto() *proto.SearchRequest {
	return &proto.SearchRequest{
		Repo:      string(r.Repo),
		RepoId:    uint32(r.RepoID),
		CommitOid: string(r.Commit),
		Branch:    r.Branch,
		Indexed:   r.Indexed,
		PatternInfo: &proto.PatternInfo{
			Query:                        r.PatternInfo.Query.ToProto(),
			IsStructural:                 r.PatternInfo.IsStructuralPat,
			IsCaseSensitive:              r.PatternInfo.IsCaseSensitive,
			ExcludePattern:               r.PatternInfo.ExcludePaths,
			IncludePatterns:              r.PatternInfo.IncludePaths,
			PathPatternsAreCaseSensitive: r.PatternInfo.PathPatternsAreCaseSensitive,
			Limit:                        int64(r.PatternInfo.Limit),
			PatternMatchesContent:        r.PatternInfo.PatternMatchesContent,
			PatternMatchesPath:           r.PatternInfo.PatternMatchesPath,
			CombyRule:                    r.PatternInfo.CombyRule,
			IncludeLangs:                 r.PatternInfo.IncludeLangs,
			ExcludeLangs:                 r.PatternInfo.ExcludeLangs,
			Select:                       r.PatternInfo.Select,
			Languages:                    r.PatternInfo.Languages,
		},
		FetchTimeout:    durationpb.New(r.FetchTimeout),
		NumContextLines: r.NumContextLines,
	}
}

func (r *Request) FromProto(req *proto.SearchRequest) {
	*r = Request{
		Repo:   api.RepoName(req.Repo),
		RepoID: api.RepoID(req.RepoId),
		Commit: api.CommitID(req.CommitOid),
		Branch: req.Branch,
		PatternInfo: PatternInfo{
			Query:                        NodeFromProto(req.PatternInfo.Query),
			IsStructuralPat:              req.PatternInfo.IsStructural,
			IsCaseSensitive:              req.PatternInfo.IsCaseSensitive,
			ExcludePaths:                 req.PatternInfo.ExcludePattern,
			IncludePaths:                 req.PatternInfo.IncludePatterns,
			PathPatternsAreCaseSensitive: req.PatternInfo.PathPatternsAreCaseSensitive,
			Limit:                        int(req.PatternInfo.Limit),
			PatternMatchesContent:        req.PatternInfo.PatternMatchesContent,
			PatternMatchesPath:           req.PatternInfo.PatternMatchesPath,
			IncludeLangs:                 req.PatternInfo.IncludeLangs,
			ExcludeLangs:                 req.PatternInfo.ExcludeLangs,
			CombyRule:                    req.PatternInfo.CombyRule,
			Select:                       req.PatternInfo.Select,
		},
		FetchTimeout:    req.FetchTimeout.AsDuration(),
		Indexed:         req.Indexed,
		NumContextLines: req.NumContextLines,
	}
}

func NodeFromProto(p *proto.QueryNode) QueryNode {
	switch v := p.GetValue().(type) {
	case *proto.QueryNode_Pattern:
		return &PatternNode{
			Value:     v.Pattern.Value,
			IsRegExp:  v.Pattern.IsRegexp,
			IsNegated: v.Pattern.IsNegated,
			Boost:     v.Pattern.Boost,
		}
	case *proto.QueryNode_And:
		children := make([]QueryNode, 0, len(v.And.Children))
		for _, child := range v.And.Children {
			children = append(children, NodeFromProto(child))
		}
		return &AndNode{Children: children}
	case *proto.QueryNode_Or:
		children := make([]QueryNode, 0, len(v.Or.Children))
		for _, child := range v.Or.Children {
			children = append(children, NodeFromProto(child))
		}
		return &OrNode{Children: children}
	default:
		// Use a panic since this is used in a struct initializer, and there's not
		// a nice way to handle an error
		panic(fmt.Sprintf("unknown query node type %T", p.GetValue()))
	}
}

// Response represents the response from a Search request.
type Response struct {
	Matches []FileMatch

	// LimitHit is true if Matches may not include all FileMatches because a match limit was hit.
	LimitHit bool

	// DeadlineHit is true if Matches may not include all FileMatches because a deadline was hit.
	DeadlineHit bool
}

// FileMatch is the struct used to represent search results
type FileMatch struct {
	Path     string
	Language string

	ChunkMatches []ChunkMatch

	// LimitHit is true if LineMatches may not include all LineMatches.
	LimitHit bool
}

func (fm *FileMatch) ToProto() *proto.FileMatch {
	chunkMatches := make([]*proto.ChunkMatch, len(fm.ChunkMatches))
	for i, cm := range fm.ChunkMatches {
		chunkMatches[i] = cm.ToProto()
	}
	return &proto.FileMatch{
		Path:         []byte(fm.Path),
		Language:     []byte(fm.Language),
		ChunkMatches: chunkMatches,
		LimitHit:     fm.LimitHit,
	}
}

func (fm *FileMatch) FromProto(pm *proto.FileMatch) {
	chunkMatches := make([]ChunkMatch, len(pm.GetChunkMatches()))
	for i, cm := range pm.GetChunkMatches() {
		chunkMatches[i].FromProto(cm)
	}
	*fm = FileMatch{
		Path:         string(pm.GetPath()), // WARNING: It is not safe to assume that Path is utf-8 encoded.
		Language:     string(pm.GetLanguage()),
		ChunkMatches: chunkMatches,
		LimitHit:     pm.GetLimitHit(),
	}
}

func (fm FileMatch) MatchCount() int {
	if len(fm.ChunkMatches) == 0 {
		return 1 // path match is still one match
	}
	count := 0
	for _, cm := range fm.ChunkMatches {
		count += len(cm.Ranges)
	}
	return count
}

func (fm *FileMatch) Limit(limit int) {
	for i := range fm.ChunkMatches {
		l := len(fm.ChunkMatches[i].Ranges)
		if l <= limit {
			limit -= l
			continue
		}

		// invariant: limit < l
		fm.ChunkMatches[i].Ranges = fm.ChunkMatches[i].Ranges[:limit]
		if limit > 0 {
			fm.ChunkMatches = fm.ChunkMatches[:i+1]
		} else {
			fm.ChunkMatches = fm.ChunkMatches[:i]
		}
		fm.LimitHit = true
		return
	}
}

type ChunkMatch struct {
	Content      string // Warning: It is not safe to assume that Content is utf-8 encoded.
	ContentStart Location
	Ranges       []Range
}

func (cm ChunkMatch) MatchedContent() []string {
	res := make([]string, 0, len(cm.Ranges))
	for _, rr := range cm.Ranges {
		res = append(res, cm.Content[rr.Start.Offset-cm.ContentStart.Offset:rr.End.Offset-cm.ContentStart.Offset])
	}
	return res
}

func (cm *ChunkMatch) ToProto() *proto.ChunkMatch {
	ranges := make([]*proto.Range, len(cm.Ranges))
	for i, r := range cm.Ranges {
		ranges[i] = r.ToProto()
	}
	return &proto.ChunkMatch{
		Content:      []byte(cm.Content),
		ContentStart: cm.ContentStart.ToProto(),
		Ranges:       ranges,
	}
}

func (cm *ChunkMatch) FromProto(pm *proto.ChunkMatch) {
	var contentStart Location
	contentStart.FromProto(pm.GetContentStart())

	ranges := make([]Range, len(pm.GetRanges()))
	for i, r := range pm.GetRanges() {
		ranges[i].FromProto(r)
	}

	*cm = ChunkMatch{
		Content:      string(pm.GetContent()), // WARNING: It is not safe to assume that the chunk match content is utf-8 encoded.
		ContentStart: contentStart,
		Ranges:       ranges,
	}
}

type Range struct {
	Start Location
	End   Location
}

func (r *Range) ToProto() *proto.Range {
	return &proto.Range{
		Start: r.Start.ToProto(),
		End:   r.End.ToProto(),
	}
}

func (r *Range) FromProto(pr *proto.Range) {
	r.Start.FromProto(pr.GetStart())
	r.End.FromProto(pr.GetEnd())
}

type Location struct {
	// The byte offset from the beginning of the file.
	Offset int32

	// Line is the count of newlines before the offset in the file.
	// Line is 0-based.
	Line int32

	// Column is the rune offset from the beginning of the last line.
	Column int32
}

func (l *Location) ToProto() *proto.Location {
	return &proto.Location{
		Offset: l.Offset,
		Line:   l.Line,
		Column: l.Column,
	}
}

func (l *Location) FromProto(pl *proto.Location) {
	*l = Location{
		Offset: pl.GetOffset(),
		Line:   pl.GetLine(),
		Column: pl.GetColumn(),
	}
}
