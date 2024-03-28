import type { EmbeddingsSearchResults, SourcegraphGraphQLAPIClient } from '../sourcegraph-api/graphql'

import type { EmbeddingsSearch } from '.'

export class SourcegraphEmbeddingsSearchClient implements EmbeddingsSearch {
    constructor(private client: SourcegraphGraphQLAPIClient, private repoId: string, private web: boolean = false) {}

    public async search(
        query: string,
        codeResultsCount: number,
        textResultsCount: number
    ): Promise<EmbeddingsSearchResults | Error> {
        if (this.web) {
            return this.client.searchEmbeddings([this.repoId], query, codeResultsCount, textResultsCount)
        }

        return this.client.legacySearchEmbeddings(this.repoId, query, codeResultsCount, textResultsCount)
    }
}
