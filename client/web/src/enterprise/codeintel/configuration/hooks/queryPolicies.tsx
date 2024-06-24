import type { ApolloClient } from '@apollo/client'
import { from, type Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { getDocumentNode, gql } from '@sourcegraph/http-client'

import type {
    CodeIntelligenceConfigurationPoliciesResult,
    CodeIntelligenceConfigurationPoliciesVariables,
    CodeIntelligenceConfigurationPolicyFields,
} from '../../../../graphql-operations'

import { defaultCodeIntelligenceConfigurationPolicyFieldsFragment } from './types'

interface PolicyConnection {
    nodes: CodeIntelligenceConfigurationPolicyFields[]
    totalCount: number | null
    pageInfo: { endCursor: string | null; hasNextPage: boolean }
}

export const POLICIES_CONFIGURATION = gql`
    query CodeIntelligenceConfigurationPolicies(
        $repository: ID
        $query: String
        $forDataRetention: Boolean
        $forIndexing: Boolean
        $first: Int
        $after: String
        $protected: Boolean
    ) {
        codeIntelligenceConfigurationPolicies(
            repository: $repository
            query: $query
            forDataRetention: $forDataRetention
            forIndexing: $forIndexing
            first: $first
            after: $after
            protected: $protected
        ) {
            nodes {
                ...CodeIntelligenceConfigurationPolicyFields
            }
            totalCount
            pageInfo {
                endCursor
                hasNextPage
            }
        }
    }

    ${defaultCodeIntelligenceConfigurationPolicyFieldsFragment}
`

export const queryPolicies = (
    {
        repository,
        first,
        query,
        forDataRetention,
        forIndexing,
        after,
        protected: varProtected,
    }: Partial<CodeIntelligenceConfigurationPoliciesVariables>,
    client: ApolloClient<object>
): Observable<PolicyConnection> => {
    const variables: CodeIntelligenceConfigurationPoliciesVariables = {
        repository: repository ?? null,
        query: query ?? null,
        forDataRetention: forDataRetention ?? null,
        forIndexing: forIndexing ?? null,
        first: first ?? null,
        after: after ?? null,
        protected: varProtected ?? null,
    }

    return from(
        client.query<CodeIntelligenceConfigurationPoliciesResult>({
            query: getDocumentNode(POLICIES_CONFIGURATION),
            variables,
        })
    ).pipe(
        map(({ data }) => data),
        map(({ codeIntelligenceConfigurationPolicies }) => codeIntelligenceConfigurationPolicies)
    )
}
