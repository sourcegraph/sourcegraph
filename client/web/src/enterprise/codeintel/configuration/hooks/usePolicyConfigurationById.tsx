import { ApolloError } from '@apollo/client'

import { gql, useQuery } from '@sourcegraph/http-client'

import {
    type CodeIntelligenceConfigurationPolicyFields,
    type CodeIntelligenceConfigurationPolicyResult,
    GitObjectType,
} from '../../../../graphql-operations'

import { defaultCodeIntelligenceConfigurationPolicyFieldsFragment } from './types'

export const POLICY_CONFIGURATION_BY_ID = gql`
    query CodeIntelligenceConfigurationPolicy($id: ID!) {
        node(id: $id) {
            ...CodeIntelligenceConfigurationPolicyFields
        }
    }

    ${defaultCodeIntelligenceConfigurationPolicyFieldsFragment}
`

interface UsePolicyConfigResult {
    policyConfig: CodeIntelligenceConfigurationPolicyFields | undefined
    loadingPolicyConfig: boolean
    policyConfigError: ApolloError | undefined
}

const emptyPolicy: CodeIntelligenceConfigurationPolicyFields = {
    __typename: 'CodeIntelligenceConfigurationPolicy',
    id: '',
    name: '',
    repository: null,
    repositoryPatterns: null,
    type: GitObjectType.GIT_UNKNOWN,
    pattern: '',
    protected: false,
    retentionEnabled: false,
    retentionDurationHours: null,
    retainIntermediateCommits: false,
    indexingEnabled: false,
    indexCommitMaxAgeHours: null,
    indexIntermediateCommits: false,
}

export const usePolicyConfigurationByID = (id: string): UsePolicyConfigResult => {
    const { data, loading, error } = useQuery<CodeIntelligenceConfigurationPolicyResult>(POLICY_CONFIGURATION_BY_ID, {
        variables: { id },
        skip: id === 'new',
    })

    const response = (data?.node?.__typename === 'CodeIntelligenceConfigurationPolicy' && data.node) || undefined
    const isNew = id === 'new'

    return {
        policyConfig: isNew ? emptyPolicy : response,
        loadingPolicyConfig: loading,
        policyConfigError:
            !response && !isNew
                ? new ApolloError({ errorMessage: 'No such CodeIntelligenceConfigurationPolicy' })
                : error,
    }
}
