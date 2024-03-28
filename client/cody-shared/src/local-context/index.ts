export interface ContextResult {
    repoName?: string
    revision?: string
    fileName: string
    content: string
}

export interface KeywordContextFetcher {
    getContext(query: string, numResults: number): Promise<ContextResult[]>

    getSearchContext(query: string, numResults: number): Promise<ContextResult[]>
}

export interface FilenameContextFetcher {
    getContext(query: string, numResults: number): Promise<ContextResult[]>
}

export interface Point {
    row: number
    col: number
}

export interface Range {
    startByte: number
    endByte: number
    startPoint: Point
    endPoint: Point
}

export interface Result {
    fqname: string
    name: string
    type: string
    doc: string
    exported: boolean
    lang: string
    file: string
    range: Range
    summary: string
}

export interface IndexedKeywordContextFetcher {
    getIndexReady(scopeDir: string, whenReadyFn: () => void): Promise<boolean>

    getResults(query: string, scopeDir: string): Promise<Result[]>
}
