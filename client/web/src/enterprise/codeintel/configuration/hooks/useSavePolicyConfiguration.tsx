import type { ApolloError, FetchResult, MutationFunctionOptions, OperationVariables } from '@apollo/client'

import { gql, useMutation } from '@sourcegraph/http-client'

import type { UpdateCodeIntelligenceConfigurationPolicyResult } from '../../../../graphql-operations'

const CREATE_POLICY_CONFIGURATION = gql`
    mutation CreateCodeIntelligenceConfigurationPolicy(
        $repositoryId: ID
        $repositoryPatterns: [String!]
        $name: String!
        $type: GitObjectType!
        $pattern: String!
        $retentionEnabled: Boolean!
        $retentionDurationHours: Int
        $retainIntermediateCommits: Boolean!
        $indexingEnabled: Boolean!
        $indexCommitMaxAgeHours: Int
        $indexIntermediateCommits: Boolean!
    ) {
        createCodeIntelligenceConfigurationPolicy(
            repository: $repositoryId
            repositoryPatterns: $repositoryPatterns
            name: $name
            type: $type
            pattern: $pattern
            retentionEnabled: $retentionEnabled
            retentionDurationHours: $retentionDurationHours
            retainIntermediateCommits: $retainIntermediateCommits
            indexingEnabled: $indexingEnabled
            indexCommitMaxAgeHours: $indexCommitMaxAgeHours
            indexIntermediateCommits: $indexIntermediateCommits
        ) {
            id
        }
    }
`

const UPDATE_POLICY_CONFIGURATION = gql`
    mutation UpdateCodeIntelligenceConfigurationPolicy(
        $id: ID!
        $name: String!
        $repositoryPatterns: [String!]
        $type: GitObjectType!
        $pattern: String!
        $retentionEnabled: Boolean!
        $retentionDurationHours: Int
        $retainIntermediateCommits: Boolean!
        $indexingEnabled: Boolean!
        $indexCommitMaxAgeHours: Int
        $indexIntermediateCommits: Boolean!
    ) {
        updateCodeIntelligenceConfigurationPolicy(
            id: $id
            name: $name
            repositoryPatterns: $repositoryPatterns
            type: $type
            pattern: $pattern
            retentionEnabled: $retentionEnabled
            retentionDurationHours: $retentionDurationHours
            retainIntermediateCommits: $retainIntermediateCommits
            indexingEnabled: $indexingEnabled
            indexCommitMaxAgeHours: $indexCommitMaxAgeHours
            indexIntermediateCommits: $indexIntermediateCommits
        ) {
            alwaysNil
        }
    }
`

type SavePolicyConfigResult = Promise<
    FetchResult<UpdateCodeIntelligenceConfigurationPolicyResult, Record<string, string>, Record<string, string>>
>
interface SavePolicyConfigurationResult {
    savePolicyConfiguration: (
        options?:
            | MutationFunctionOptions<UpdateCodeIntelligenceConfigurationPolicyResult, OperationVariables>
            | undefined
    ) => SavePolicyConfigResult
    isSaving: boolean
    savingError: ApolloError | undefined
}

export const useSavePolicyConfiguration = (isNew: boolean): SavePolicyConfigurationResult => {
    const mutation = isNew ? CREATE_POLICY_CONFIGURATION : UPDATE_POLICY_CONFIGURATION
    const [savePolicyConfiguration, { loading, error }] =
        useMutation<UpdateCodeIntelligenceConfigurationPolicyResult>(mutation)

    return {
        savePolicyConfiguration,
        isSaving: loading,
        savingError: error,
    }
}
