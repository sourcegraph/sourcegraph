import { lastValueFrom } from 'rxjs'

import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'

import { requestGraphQL } from '../../../backend/graphql'
import type { CloseBatchChangeResult, CloseBatchChangeVariables } from '../../../graphql-operations'

export async function closeBatchChange({ batchChange, closeChangesets }: CloseBatchChangeVariables): Promise<void> {
    const result = await lastValueFrom(
        requestGraphQL<CloseBatchChangeResult, CloseBatchChangeVariables>(
            gql`
                mutation CloseBatchChange($batchChange: ID!, $closeChangesets: Boolean) {
                    closeBatchChange(batchChange: $batchChange, closeChangesets: $closeChangesets) {
                        id
                    }
                }
            `,
            { batchChange, closeChangesets }
        )
    )
    dataOrThrowErrors(result)
}
