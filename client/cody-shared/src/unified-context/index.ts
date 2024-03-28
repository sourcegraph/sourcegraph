export interface FileChunkContext {
    type: 'FileChunkContext'
    filePath: string
    content: string
    startLine: number
    endLine: number
    repoName: string
    revision: string
}

export type UnifiedContextFetcherResult = FileChunkContext | { type: 'UnknownContext' }

export interface UnifiedContextFetcher {
    getContext(
        query: string,
        codeResultsCount: number,
        textResultsCount: number
    ): Promise<UnifiedContextFetcherResult[] | Error>
}
