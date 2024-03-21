package v1

import (
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func (x *SearchRequest) FromInternal(p *search.SymbolsParameters) {
	*x = SearchRequest{
		Repo:     string(p.Repo),
		CommitId: string(p.CommitID),

		Query:           p.Query,
		IsRegExp:        p.IsRegExp,
		IsCaseSensitive: p.IsCaseSensitive,
		IncludePatterns: p.IncludePatterns,
		ExcludePattern:  p.ExcludePattern,
		IncludeLangs:    p.IncludeLangs,
		ExcludeLangs:    p.ExcludeLangs,

		First:   int32(p.First),
		Timeout: durationpb.New(p.Timeout),
	}
}

func (x *SearchRequest) ToInternal() search.SymbolsParameters {
	return search.SymbolsParameters{
		Repo:            api.RepoName(x.GetRepo()),
		CommitID:        api.CommitID(x.GetCommitId()),
		Query:           x.GetQuery(),
		IsRegExp:        x.GetIsRegExp(),
		IsCaseSensitive: x.GetIsCaseSensitive(),
		IncludePatterns: x.GetIncludePatterns(),
		ExcludePattern:  x.GetExcludePattern(),
		IncludeLangs:    x.GetIncludeLangs(),
		ExcludeLangs:    x.GetExcludeLangs(),
		First:           int(x.GetFirst()),
		Timeout:         x.GetTimeout().AsDuration(),
	}
}

func (x *SearchResponse) FromInternal(r *search.SymbolsResponse) {
	symbols := make([]*SearchResponse_Symbol, 0, len(r.Symbols))

	for _, s := range r.Symbols {
		var ps SearchResponse_Symbol
		ps.FromInternal(&s)

		symbols = append(symbols, &ps)
	}

	var err *string
	if r.Err != "" {
		err = &r.Err
	}

	*x = SearchResponse{
		Symbols:  symbols,
		Error:    err,
		LimitHit: r.LimitHit,
	}
}

func (x *SearchResponse) ToInternal() search.SymbolsResponse {
	symbols := make([]result.Symbol, 0, len(x.GetSymbols()))

	for _, s := range x.GetSymbols() {
		symbols = append(symbols, s.ToInternal())
	}

	return search.SymbolsResponse{
		Symbols:  symbols,
		Err:      x.GetError(),
		LimitHit: x.GetLimitHit(),
	}
}

func (x *SearchResponse_Symbol) FromInternal(s *result.Symbol) {
	*x = SearchResponse_Symbol{
		Name: s.Name,
		Path: s.Path,

		Line:      int32(s.Line),
		Character: int32(s.Character),

		Kind:     s.Kind,
		Language: s.Language,

		Parent:     s.Parent,
		ParentKind: s.ParentKind,

		Signature:   s.Signature,
		FileLimited: s.FileLimited,
	}
}

func (x *SearchResponse_Symbol) ToInternal() result.Symbol {
	return result.Symbol{
		Name: x.GetName(),
		Path: x.GetPath(),

		Line:      int(x.GetLine()),
		Character: int(x.GetCharacter()),

		Kind:     x.GetKind(),
		Language: x.GetLanguage(),

		Parent:     x.GetParent(),
		ParentKind: x.GetParentKind(),

		Signature:   x.GetSignature(),
		FileLimited: x.GetFileLimited(),
	}
}

func (x *SymbolInfoResponse) FromInternal(s *types.SymbolInfo) {
	if s == nil {
		*x = SymbolInfoResponse{}
		return
	}

	var rcp RepoCommitPath
	rcp.FromInternal(&s.Definition.RepoCommitPath)

	var maybeRange *Range
	if s.Definition.Range != nil {
		maybeRange = &Range{}
		maybeRange.FromInternal(s.Definition.Range)
	}

	*x = SymbolInfoResponse{
		Result: &SymbolInfoResponse_DefinitionResult{
			Definition: &SymbolInfoResponse_Definition{
				RepoCommitPath: &rcp,
				Range:          maybeRange,
			},
			Hover: s.Hover,
		},
	}
}

func (x *SymbolInfoResponse) ToInternal() *types.SymbolInfo {
	maybeResult := x.GetResult()
	if maybeResult == nil {
		return nil
	}

	var definition types.RepoCommitPathMaybeRange

	protoDefinition := maybeResult.GetDefinition()

	definition.RepoCommitPath = protoDefinition.GetRepoCommitPath().ToInternal()
	if protoDefinition.GetRange() != nil {
		defRange := protoDefinition.GetRange().ToInternal()
		definition.Range = &defRange
	}

	return &types.SymbolInfo{
		Definition: definition,
		Hover:      maybeResult.Hover, // don't use GetHover() because it returns a string, not a pointer to a string
	}
}

func (x *LocalCodeIntelResponse) FromInternal(p *types.LocalCodeIntelPayload) {
	symbols := make([]*LocalCodeIntelResponse_Symbol, 0, len(p.Symbols))

	for _, s := range p.Symbols {
		var symbol LocalCodeIntelResponse_Symbol
		symbol.FromInternal(&s)

		symbols = append(symbols, &symbol)
	}

	*x = LocalCodeIntelResponse{
		Symbols: symbols,
	}
}

func (x *LocalCodeIntelResponse) ToInternal() *types.LocalCodeIntelPayload {
	if x == nil {
		return nil
	}

	symbols := make([]types.Symbol, 0, len(x.GetSymbols()))

	for _, s := range x.GetSymbols() {
		symbols = append(symbols, s.ToInternal())
	}

	return &types.LocalCodeIntelPayload{
		Symbols: symbols,
	}
}

func (x *LocalCodeIntelResponse_Symbol) FromInternal(s *types.Symbol) {
	refs := make([]*Range, 0, len(s.Refs))

	for _, r := range s.Refs {
		protoRef := &Range{}
		protoRef.FromInternal(&r)

		refs = append(refs, protoRef)
	}

	var def Range
	def.FromInternal(&s.Def)

	*x = LocalCodeIntelResponse_Symbol{
		Name:  s.Name,
		Hover: s.Hover,
		Def:   &def,
		Refs:  refs,
	}
}

func (x *LocalCodeIntelResponse_Symbol) ToInternal() types.Symbol {
	def := x.GetDef().ToInternal()

	refs := make([]types.Range, 0, len(x.GetRefs()))

	for _, ref := range x.GetRefs() {
		refs = append(refs, ref.ToInternal())
	}

	return types.Symbol{
		Name:  x.GetName(),
		Hover: x.GetHover(),
		Def:   def,
		Refs:  refs,
	}
}

func (x *RepoCommitPath) FromInternal(r *types.RepoCommitPath) {
	*x = RepoCommitPath{
		Repo:   r.Repo,
		Commit: r.Commit,
		Path:   r.Path,
	}
}

func (x *RepoCommitPath) ToInternal() types.RepoCommitPath {
	return types.RepoCommitPath{
		Repo:   x.GetRepo(),
		Commit: x.GetCommit(),
		Path:   x.GetPath(),
	}
}

func (x *Range) FromInternal(r *types.Range) {
	*x = Range{
		Row:    int32(r.Row),
		Column: int32(r.Column),
		Length: int32(r.Length),
	}
}

func (x *Range) ToInternal() types.Range {
	return types.Range{
		Row:    int(x.GetRow()),
		Column: int(x.GetColumn()),
		Length: int(x.GetLength()),
	}
}

func (x *Point) FromInternal(p *types.Point) {
	*x = Point{
		Row:    int32(p.Row),
		Column: int32(p.Column),
	}
}

func (x *Point) ToInternal() types.Point {
	return types.Point{
		Row:    int(x.GetRow()),
		Column: int(x.GetColumn()),
	}
}
