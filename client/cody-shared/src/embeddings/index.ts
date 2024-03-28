import type { EmbeddingsSearchResults } from '../sourcegraph-api/graphql'

export interface EmbeddingsSearch {
    search(query: string, codeResultsCount: number, textResultsCount: number): Promise<EmbeddingsSearchResults | Error>
}
