import type { SourcegraphGraphQLAPIClient } from '../sourcegraph-api/graphql'
import { isError } from '../utils'

import type { Attribution, Guardrails } from '.'

export class SourcegraphGuardrailsClient implements Guardrails {
    constructor(private client: SourcegraphGraphQLAPIClient) {}

    public async searchAttribution(snippet: string): Promise<Attribution | Error> {
        const result = await this.client.searchAttribution(snippet)

        if (isError(result)) {
            return result
        }

        return {
            limitHit: result.limitHit,
            repositories: result.nodes.map(repo => ({ name: repo.repositoryName })),
        }
    }
}
