import { gql } from '@sourcegraph/http-client'

import { GitObjectType } from '../../../../graphql-operations'

export const nullPolicy = {
    __typename: 'CodeIntelligenceConfigurationPolicy' as const,
    id: '',
    name: '',
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
    repository: null,
}

export const defaultCodeIntelligenceConfigurationPolicyFieldsFragment = gql`
    fragment CodeIntelligenceConfigurationPolicyFields on CodeIntelligenceConfigurationPolicy {
        __typename
        id
        name
        repository {
            id
            name
        }
        repositoryPatterns
        type
        pattern
        protected
        retentionEnabled
        retentionDurationHours
        retainIntermediateCommits
        indexingEnabled
        indexCommitMaxAgeHours
        indexIntermediateCommits
    }
`
