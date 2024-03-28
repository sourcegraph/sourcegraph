import { isErrorLike } from '../common'
import type { SourcegraphGraphQLAPIClient } from '../sourcegraph-api/graphql'

import type { UnifiedContextFetcher, UnifiedContextFetcherResult } from '.'

export class UnifiedContextFetcherClient implements UnifiedContextFetcher {
    constructor(private client: SourcegraphGraphQLAPIClient, private repoIds: string[]) {}

    public async getContext(
        query: string,
        codeResultsCount: number,
        textResultsCount: number
    ): Promise<UnifiedContextFetcherResult[] | Error> {
        const response = await this.client.getCodyContext(this.repoIds, query, codeResultsCount, textResultsCount)

        if (isErrorLike(response)) {
            return response
        }

        return response.reduce((results, result) => {
            if (result?.__typename === 'FileChunkContext') {
                results.push({
                    type: 'FileChunkContext',
                    filePath: result.blob.path,
                    content: result.chunkContent,
                    startLine: result.startLine,
                    endLine: result.endLine,
                    repoName: result.blob.repository.name,
                    revision: result.blob.commit.oid,
                })
            } else {
                results.push({ type: 'UnknownContext' })
            }

            return results
        }, [] as UnifiedContextFetcherResult[])
    }
}
