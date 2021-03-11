import { CloseBatchChangeResult, CloseBatchChangeVariables } from '../../../graphql-operations'
import { gql, dataOrThrowErrors } from '../../../../../shared/src/graphql/graphql'
import { requestGraphQL } from '../../../backend/graphql'

export async function closeBatchChange({ batchChange, closeChangesets }: CloseBatchChangeVariables): Promise<void> {
    const result = await requestGraphQL<CloseBatchChangeResult, CloseBatchChangeVariables>(
        gql`
            mutation CloseBatchChange($batchChange: ID!, $closeChangesets: Boolean) {
                closeBatchChange(batchChange: $batchChange, closeChangesets: $closeChangesets) {
                    id
                }
            }
        `,
        { batchChange, closeChangesets }
    ).toPromise()
    dataOrThrowErrors(result)
}
