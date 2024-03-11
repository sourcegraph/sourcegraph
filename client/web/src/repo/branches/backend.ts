import { gql, useQuery } from '@sourcegraph/http-client'

import {
    GitRefFields,
    RepositoryGitBranchesOverviewResult,
    RepositoryGitBranchesOverviewVariables,
    Scalars,
} from '../../graphql-operations'
import { gitReferenceFragments } from '../GitReference'

interface Branches {
    loading: boolean
    error?: Error

    defaultBranch: GitRefFields | null
    activeBranches: GitRefFields[]
    hasMoreActiveBranches: boolean
}

const REPOSITORY_GIT_BRANCHES_OVERVIEW = gql`
    query RepositoryGitBranchesOverview($repo: ID!, $first: Int!, $withBehindAhead: Boolean!) {
        node(id: $repo) {
            ...RepositoryGitBranchesOverviewRepository
        }
    }

    fragment RepositoryGitBranchesOverviewRepository on Repository {
        defaultBranch {
            ...GitRefFields
        }
        gitRefs(first: $first, type: GIT_BRANCH) {
            nodes {
                ...GitRefFields
            }
            pageInfo {
                hasNextPage
            }
        }
    }

    ${gitReferenceFragments}
`

export function useBranches(repoID: Scalars['ID'], first: number): Branches {
    const { data, loading, error } = useQuery<
        RepositoryGitBranchesOverviewResult,
        RepositoryGitBranchesOverviewVariables
    >(REPOSITORY_GIT_BRANCHES_OVERVIEW, {
        variables: { repo: repoID, first, withBehindAhead: true },
    })

    if (!data || !data.node || data.node?.__typename !== 'Repository') {
        return {
            loading,
            error,
            activeBranches: [],
            defaultBranch: null,
            hasMoreActiveBranches: false,
        }
    }

    const repo = data.node

    return {
        loading,
        error,
        defaultBranch: repo.defaultBranch,
        activeBranches: repo.gitRefs.nodes.filter(
            // Filter out default branch from activeBranches.
            ({ id }) => !repo.defaultBranch || repo!.defaultBranch.id !== id
        ),
        hasMoreActiveBranches: repo.gitRefs.pageInfo.hasNextPage,
    }
}
