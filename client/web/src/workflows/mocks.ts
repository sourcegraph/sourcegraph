import type { MockedResponse } from '@apollo/client/testing'

import { getDocumentNode } from '@sourcegraph/http-client'

import {
    CreateWorkflowResult,
    CreateWorkflowVariables,
    DeleteWorkflowResult,
    DeleteWorkflowVariables,
    UpdateWorkflowResult,
    UpdateWorkflowVariables,
    WorkflowFields,
    WorkflowResult,
    WorkflowVariables,
    WorkflowsResult,
    WorkflowsVariables,
} from '../graphql-operations'

import {
    createWorkflowMutation,
    deleteWorkflowMutation,
    updateWorkflowMutation,
    workflowQuery,
    workflowsQuery,
} from './backend'

const WORKFLOW_FIELDS: Pick<
    WorkflowFields,
    | '__typename'
    | 'description'
    | 'template'
    | 'draft'
    | 'owner'
    | 'createdBy'
    | 'createdAt'
    | 'updatedBy'
    | 'updatedAt'
    | 'viewerCanAdminister'
> = {
    __typename: 'Workflow',
    description: 'My description',
    template: { text: 'My template text' },
    draft: false,
    owner: {
        __typename: 'User',
        id: 'a',
        namespaceName: 'alice',
    },
    createdBy: {
        __typename: 'User',
        id: 'a',
        username: 'alice',
    },
    createdAt: '2024-04-12T15:00:00Z',
    updatedBy: {
        __typename: 'User',
        id: 'a',
        username: 'alice',
    },
    updatedAt: '2024-04-15T17:00:00Z',
    viewerCanAdminister: true,
}

const workflowsMock: MockedResponse<WorkflowsResult, WorkflowsVariables> = {
    request: {
        query: getDocumentNode(workflowsQuery),
        variables: {
            query: null,
            owner: '1',
            viewerIsAffiliated: null,
            includeDrafts: true,
            after: undefined,
            before: undefined,
            first: 100,
            last: undefined,
        },
    },
    result: {
        data: {
            workflows: {
                nodes: [
                    {
                        ...WORKFLOW_FIELDS,
                        id: '1',
                        name: 'my-workflow',
                        nameWithOwner: 'alice/my-workflow',
                    },
                    {
                        ...WORKFLOW_FIELDS,
                        id: '2',
                        name: 'another-workflow',
                        description: 'Another',
                        template: { text: 'Another template text' },
                        nameWithOwner: 'alice/another-workflow',
                    },
                ],
                totalCount: 2,
                pageInfo: {
                    hasNextPage: false,
                    hasPreviousPage: false,
                    endCursor: '',
                    startCursor: '',
                },
            },
        },
    },
}

const workflowMock: MockedResponse<WorkflowResult, WorkflowVariables> = {
    request: {
        query: getDocumentNode(workflowQuery),
        variables: { id: '1' },
    },
    result: {
        data: {
            node: {
                ...WORKFLOW_FIELDS,
                __typename: 'Workflow',
                id: '1',
                name: 'my-workflow',
                nameWithOwner: 'alice/my-workflow',
            },
        },
    },
}

const createWorkflowMock: MockedResponse<CreateWorkflowResult, CreateWorkflowVariables> = {
    request: {
        query: getDocumentNode(createWorkflowMutation),
        variables: {
            input: {
                owner: 'a',
                name: 'my-workflow',
                description: 'My description',
                templateText: 'My template text',
                draft: false,
            },
        },
    },
    delay: 500,
    result: {
        data: {
            createWorkflow: {
                ...WORKFLOW_FIELDS,
                id: '1',
                name: 'my-workflow',
                nameWithOwner: 'alice/my-workflow',
            },
        },
    },
}

const updateWorkflowMock: MockedResponse<UpdateWorkflowResult, UpdateWorkflowVariables> = {
    request: {
        query: getDocumentNode(updateWorkflowMutation),
        variables: {
            id: '1',
            input: {
                name: 'my-workflow',
                description: 'My description',
                templateText: 'My template text',
                draft: false,
            },
        },
    },
    delay: 500,
    result: {
        data: {
            updateWorkflow: {
                ...WORKFLOW_FIELDS,
                id: '1',
                name: 'my-workflow',
                nameWithOwner: 'alice/my-workflow',
            },
        },
    },
}

const deleteWorkflowMock: MockedResponse<DeleteWorkflowResult, DeleteWorkflowVariables> = {
    request: {
        query: getDocumentNode(deleteWorkflowMutation),
        variables: { id: '1' },
    },
    delay: 500,
    result: {
        data: {
            deleteWorkflow: {
                alwaysNil: null,
            },
        },
    },
}

export const MOCK_REQUESTS = [workflowsMock, workflowMock, createWorkflowMock, updateWorkflowMock, deleteWorkflowMock]
