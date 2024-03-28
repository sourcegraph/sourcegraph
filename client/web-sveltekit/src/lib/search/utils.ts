import type { ContentMatch, MatchGroup, ChunkMatch, LineMatch } from '$lib/shared'

/**
 * A context object provided on pages with the main search input to interact
 * with the main input.
 */
export interface SearchPageContext {
    setQuery(query: string | ((query: string) => string)): void
}

export function resultToMatchGroups(result: ContentMatch): MatchGroup[] {
    return result.chunkMatches?.map(chunkToMatchGroup) || result.lineMatches?.map(lineToMatchGroup) || []
}

export function chunkToMatchGroup(chunk: ChunkMatch): MatchGroup {
    const matches = chunk.ranges.map(range => ({
        startLine: range.start.line,
        startCharacter: range.start.column,
        endLine: range.end.line,
        endCharacter: range.end.column,
    }))
    const plaintextLines = chunk.content.replace(/\r?\n$/, '').split(/\r?\n/)
    return {
        plaintextLines,
        highlightedHTMLRows: undefined, // populated lazily
        matches,
        startLine: chunk.contentStart.line,
        endLine: chunk.contentStart.line + plaintextLines.length,
    }
}

export function lineToMatchGroup(match: LineMatch): MatchGroup {
    const matches = match.offsetAndLengths.map(offsetAndLength => ({
        startLine: match.lineNumber,
        startCharacter: offsetAndLength[0],
        endLine: match.lineNumber,
        endCharacter: offsetAndLength[0] + offsetAndLength[1],
    }))
    return {
        plaintextLines: [match.line],
        highlightedHTMLRows: undefined, // populated lazily
        matches,
        startLine: match.lineNumber,
        endLine: match.lineNumber,
    }
}
