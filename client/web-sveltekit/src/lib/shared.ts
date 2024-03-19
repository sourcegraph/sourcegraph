// We want to limit the number of imported modules as much as possible

export type { AbsoluteRepoFile } from '@sourcegraph/shared/src/util/url'

export {
    parseRepoRevision,
    parseQueryAndHash,
    buildSearchURLQuery,
    makeRepoURI,
} from '@sourcegraph/shared/src/util/url'

export {
    isCloneInProgressErrorLike,
    isRepoSeeOtherErrorLike,
    isRepoNotFoundErrorLike,
    isRevisionNotFoundErrorLike,
    CloneInProgressError,
    RepoNotFoundError,
    RepoSeeOtherError,
    RevisionNotFoundError,
} from '@sourcegraph/shared/src/backend/errors'
export { viewerSettingsQuery } from '@sourcegraph/shared/src/backend/settings'
export { SectionID as SearchSidebarSectionID } from '@sourcegraph/shared/src/settings/temporary/searchSidebar'
export { TemporarySettingsStorage } from '@sourcegraph/shared/src/settings/temporary/TemporarySettingsStorage'
export {
    type Skipped,
    getFileMatchUrl,
    getRepositoryUrl,
    aggregateStreamingSearch,
    emptyAggregateResults,
    LATEST_VERSION,
    type AggregateStreamingSearchResults,
    type StreamSearchOptions,
    getRepoMatchLabel,
    getRepoMatchUrl,
    getMatchUrl,
    type RepositoryMatch,
    type SymbolMatch,
    type PathMatch,
    type ContentMatch,
    type ChunkMatch,
    type LineMatch,
    type SearchMatch,
    type OwnerMatch,
    type TeamMatch,
    type PersonMatch,
    type CommitMatch,
    type Progress,
    type Range,
    type Filter,
    type SearchEvent,
} from '@sourcegraph/shared/src/search/stream'
export {
    type MatchItem,
    type MatchGroupMatch,
    type MatchGroup,
    rankPassthrough,
    rankByLine,
    truncateGroups,
} from '@sourcegraph/shared/src/components/ranking/PerFileResultRanking'
export { filterExists } from '@sourcegraph/shared/src/search/query/validate'
export { scanSearchQuery } from '@sourcegraph/shared/src/search/query/scanner'
export { KeywordKind } from '@sourcegraph/shared/src/search/query/token'
export { FilterType } from '@sourcegraph/shared/src/search/query/filters'
export { getGlobalSearchContextFilter, findFilter, FilterKind } from '@sourcegraph/shared/src/search/query/query'
export { omitFilter, appendFilter, updateFilter } from '@sourcegraph/shared/src/search/query/transformer'
export { type Settings, SettingsProvider } from '@sourcegraph/shared/src/settings/settings'
export { fetchStreamSuggestions } from '@sourcegraph/shared/src/search/suggestions'
export { QueryChangeSource, type QueryState } from '@sourcegraph/shared/src/search/helpers'
export { migrateLocalStorageToTemporarySettings } from '@sourcegraph/shared/src/settings/temporary/migrateLocalStorageToTemporarySettings'
export type { TemporarySettings } from '@sourcegraph/shared/src/settings/temporary/TemporarySettings'
export { SyntaxKind } from '@sourcegraph/shared/src/codeintel/scip'
export { shortcutDisplayName } from '@sourcegraph/shared/src/keyboardShortcuts'
export { createCodeIntelAPI, type CodeIntelAPI } from '@sourcegraph/shared/src/codeintel/api'
export { getModeFromPath } from '@sourcegraph/shared/src/languages'
export type { ActionItemAction } from '@sourcegraph/shared/src/actions/ActionItem'
export { repositoryInsertText } from '@sourcegraph/shared/src/search/query/completion-utils'

// Copies of non-reusable code

// Currently defined in client/shared/src/components/RepoLink.tsx

/**
 * Returns the friendly display form of the repository name (e.g., removing "github.com/").
 */
export function displayRepoName(repoName: string): string {
    let parts = repoName.split('/')
    if (parts.length > 0 && parts[0].includes('.')) {
        parts = parts.slice(1) // remove hostname from repo name (reduce visual noise)
    }
    return parts.join('/')
}

/**
 * Splits the repository name into the dir and base components.
 */
export function splitPath(path: string): [string, string] {
    const components = path.split('/')
    return [components.slice(0, -1).join('/'), components.at(-1) ?? '']
}
