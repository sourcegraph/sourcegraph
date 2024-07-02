export enum FilterKind {
    SymbolKind = 'symbol type',
    Language = 'lang',
    Author = 'author',
    Repository = 'repo',
    CommitDate = 'commit date',
    File = 'file',
    Utility = 'utility',

    // Synthetic filters, lives only on the client
    Count = 'count',
    Type = 'type',
    Snippet = 'snippet',
}

export const DYNAMIC_FILTER_KINDS = [
    FilterKind.SymbolKind,
    FilterKind.Language,
    FilterKind.Author,
    FilterKind.Repository,
    FilterKind.CommitDate,
    FilterKind.File,
]

export enum SearchTypeFilter {
    Code = 'Code',
    Repositories = 'Repositories',
    Paths = 'Paths',
    Symbols = 'Symbols',
    Commits = 'Commits',
    Diffs = 'Diffs',
}

export const SEARCH_TYPES_TO_FILTER_TYPES: Record<`${SearchTypeFilter}`, `${FilterKind}`[]> = {
    [SearchTypeFilter.Code]: [
        FilterKind.Language,
        FilterKind.Repository,
        FilterKind.File,
        FilterKind.Utility,
        FilterKind.Count,
    ],
    [SearchTypeFilter.Repositories]: [FilterKind.Utility, FilterKind.Count],
    [SearchTypeFilter.Paths]: [
        FilterKind.Language,
        FilterKind.Repository,
        FilterKind.File,
        FilterKind.Utility,
        FilterKind.Count,
    ],
    [SearchTypeFilter.Symbols]: [
        FilterKind.SymbolKind,
        FilterKind.Language,
        FilterKind.Repository,
        FilterKind.File,
        FilterKind.Utility,
        FilterKind.Count,
    ],
    [SearchTypeFilter.Commits]: [
        FilterKind.Author,
        FilterKind.Repository,
        FilterKind.CommitDate,
        FilterKind.Utility,
        FilterKind.Count,
    ],
    [SearchTypeFilter.Diffs]: [
        FilterKind.Author,
        FilterKind.Repository,
        FilterKind.CommitDate,
        FilterKind.Utility,
        FilterKind.Count,
    ],
}
