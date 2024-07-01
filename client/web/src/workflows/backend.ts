import { gql } from '@sourcegraph/http-client'

const workflowFragment = gql`
    fragment WorkflowFields on Workflow {
        __typename
        id
        name
        description
        template {
            text
        }
        draft
        owner {
            __typename
            id
            namespaceName
        }
        nameWithOwner
        createdBy {
            id
            username
        }
        createdAt
        updatedBy {
            id
            username
        }
        updatedAt
        viewerCanAdminister
    }
`

export const workflowsQuery = gql`
    query Workflows(
        $query: String
        $owner: ID
        $viewerIsAffiliated: Boolean
        $includeDrafts: Boolean = true
        $first: Int
        $last: Int
        $after: String
        $before: String
    ) {
        workflows(
            query: $query
            owner: $owner
            viewerIsAffiliated: $viewerIsAffiliated
            includeDrafts: $includeDrafts
            first: $first
            last: $last
            after: $after
            before: $before
        ) {
            nodes {
                ...WorkflowFields
            }
            totalCount
            pageInfo {
                hasNextPage
                hasPreviousPage
                endCursor
                startCursor
            }
        }
    }
    ${workflowFragment}
`

export const workflowQuery = gql`
    query Workflow($id: ID!) {
        node(id: $id) {
            ... on Workflow {
                ...WorkflowFields
            }
        }
    }
    ${workflowFragment}
`

export const createWorkflowMutation = gql`
    mutation CreateWorkflow($input: WorkflowInput!) {
        createWorkflow(input: $input) {
            ...WorkflowFields
        }
    }
    ${workflowFragment}
`

export const updateWorkflowMutation = gql`
    mutation UpdateWorkflow($id: ID!, $input: WorkflowUpdateInput!) {
        updateWorkflow(id: $id, input: $input) {
            ...WorkflowFields
        }
    }
    ${workflowFragment}
`

export const deleteWorkflowMutation = gql`
    mutation DeleteWorkflow($id: ID!) {
        deleteWorkflow(id: $id) {
            alwaysNil
        }
    }
`
