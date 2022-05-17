import { AggregableBadge, Badge as ExtensionBadgeType } from 'sourcegraph'

/**
 * Interface for different ranking algorithms that determine how to display search results in the client.
 *
 * Determines only ranking of results for a local file.
 */
export interface PerFileResultRanking {
    /**
     * Returns the hunks that should be displayed by default before the user expands them
     */
    collapsedResults(matches: MatchItem[], context: number): RankingResult
    /**
     * Returns the hunks that should be displayed after the user has explicitly requested to see all results.
     */
    expandedResults(matches: MatchItem[], context: number): RankingResult
}

export interface RankingResult {
    matches: MatchItem[]
    grouped: MatchGroup[]
}

export interface MatchItem extends ExtensionBadgeType {
    highlightRanges: {
        start: number
        highlightLength: number
    }[]
    /**
     * The 0-based line number of this match.
     */
    line: number
    aggregableBadges?: AggregableBadge[]
}

/**
 * Describes a single group of matches.
 */
export interface MatchGroup {
    blobLines?: string[]

    // The matches in this group to display.
    matches: MatchGroupMatch[]

    // The 1-based position of where the first match in the group.
    position: {
        line: number
        character: number
    }

    // The 0-based start line of the group (inclusive.)
    startLine: number

    // The 0-based end line of the group (exclusive.)
    endLine: number
}

export interface MatchGroupMatch {
    line: number
    character: number
    highlightLength: number
    isInContext: boolean
}
