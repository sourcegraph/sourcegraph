import { storiesOf } from '@storybook/react'
import { MATCH_ANY_PARAMETERS, MockedResponses, WildcardMockLink } from 'wildcard-mock-link'

import { getDocumentNode } from '@sourcegraph/http-client'
import {
    EMPTY_SETTINGS_CASCADE,
    SettingsOrgSubject,
    SettingsUserSubject,
} from '@sourcegraph/shared/src/settings/settings'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { WebStory } from '../../../../components/WebStory'
import { BatchSpecWorkspaceResolutionState, BatchSpecWorkspaceState } from '../../../../graphql-operations'
import { mockAuthenticatedUser } from '../../../code-monitoring/testing/util'
import { GET_BATCH_CHANGE_TO_EDIT, WORKSPACE_RESOLUTION_STATUS } from '../../create/backend'
import {
    COMPLETED_BATCH_SPEC,
    EXECUTING_BATCH_SPEC,
    FAILED_BATCH_SPEC,
    mockBatchChange,
    mockWorkspaceResolutionStatus,
    mockWorkspaces,
} from '../batch-spec.mock'

import { BATCH_SPEC_WORKSPACES, FETCH_BATCH_SPEC_EXECUTION } from './backend'
import { ExecuteBatchSpecPage } from './ExecuteBatchSpecPage'

const { add } = storiesOf('web/batches/batch-spec/execute/ExecuteBatchSpecPage', module)
    .addDecorator(story => (
        <div className="p-3" style={{ height: '95vh', width: '100%' }}>
            {story()}
        </div>
    ))
    .addParameters({
        chromatic: {
            disableSnapshot: false,
        },
    })

const FIXTURE_ORG: SettingsOrgSubject = {
    __typename: 'Org',
    name: 'sourcegraph',
    displayName: 'Sourcegraph',
    id: 'a',
    viewerCanAdminister: true,
}

const FIXTURE_USER: SettingsUserSubject = {
    __typename: 'User',
    username: 'alice',
    displayName: 'alice',
    id: 'b',
    viewerCanAdminister: true,
}

const SETTINGS_CASCADE = {
    ...EMPTY_SETTINGS_CASCADE,
    subjects: [
        { subject: FIXTURE_ORG, settings: { a: 1 }, lastID: 1 },
        { subject: FIXTURE_USER, settings: { b: 2 }, lastID: 2 },
    ],
}

const COMMON_MOCKS: MockedResponses = [
    {
        request: {
            query: getDocumentNode(GET_BATCH_CHANGE_TO_EDIT),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: { data: { batchChange: mockBatchChange() } },
        nMatches: Number.POSITIVE_INFINITY,
    },
    {
        request: {
            query: getDocumentNode(WORKSPACE_RESOLUTION_STATUS),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: { data: mockWorkspaceResolutionStatus(BatchSpecWorkspaceResolutionState.COMPLETED) },
        nMatches: Number.POSITIVE_INFINITY,
    },
]

const SUCCESSFUL_MOCKS: MockedResponses = [
    {
        request: {
            query: getDocumentNode(BATCH_SPEC_WORKSPACES),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: { data: mockWorkspaces(50) },
        nMatches: Number.POSITIVE_INFINITY,
    },
    {
        request: {
            query: getDocumentNode(FETCH_BATCH_SPEC_EXECUTION),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: { data: { node: EXECUTING_BATCH_SPEC } },
        nMatches: Number.POSITIVE_INFINITY,
    },
]

add('executing', () => (
    <WebStory>
        {props => (
            <MockedTestProvider link={new WildcardMockLink([...COMMON_MOCKS, ...SUCCESSFUL_MOCKS])}>
                <ExecuteBatchSpecPage
                    {...props}
                    batchSpecID="spec1234"
                    batchChange={{ name: 'my-batch-change', namespace: 'user1234' }}
                    authenticatedUser={mockAuthenticatedUser}
                    settingsCascade={SETTINGS_CASCADE}
                />
            </MockedTestProvider>
        )}
    </WebStory>
))

const FAILED_MOCKS: MockedResponses = [
    {
        request: {
            query: getDocumentNode(BATCH_SPEC_WORKSPACES),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: { data: mockWorkspaces(50, { state: BatchSpecWorkspaceState.FAILED, failureMessage: 'Uh oh!' }) },
        nMatches: Number.POSITIVE_INFINITY,
    },
    {
        request: {
            query: getDocumentNode(FETCH_BATCH_SPEC_EXECUTION),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: { data: { node: FAILED_BATCH_SPEC } },
        nMatches: Number.POSITIVE_INFINITY,
    },
]

add('failed', () => {
    console.log(mockWorkspaces(50, { state: BatchSpecWorkspaceState.FAILED, failureMessage: 'Uh oh!' }))
    return (
        <WebStory>
            {props => (
                <MockedTestProvider link={new WildcardMockLink([...COMMON_MOCKS, ...FAILED_MOCKS])}>
                    <ExecuteBatchSpecPage
                        {...props}
                        batchSpecID="spec1234"
                        batchChange={{ name: 'my-batch-change', namespace: 'user1234' }}
                        testContextState={{
                            errors: {
                                execute:
                                    "Oh no something went wrong. This is a longer error message to demonstrate how this might take up a decent portion of screen real estate but hopefully it's still helpful information so it's worth the cost. Here's a long error message with some bullets:\n  * This is a bullet\n  * This is another bullet\n  * This is a third bullet and it's also the most important one so it's longer than all the others wow look at that.",
                            },
                        }}
                        authenticatedUser={mockAuthenticatedUser}
                        settingsCascade={SETTINGS_CASCADE}
                    />
                </MockedTestProvider>
            )}
        </WebStory>
    )
})

const COMPLETED_MOCKS: MockedResponses = [
    {
        request: {
            query: getDocumentNode(BATCH_SPEC_WORKSPACES),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: { data: mockWorkspaces(50, { state: BatchSpecWorkspaceState.COMPLETED }) },
        nMatches: Number.POSITIVE_INFINITY,
    },
    {
        request: {
            query: getDocumentNode(FETCH_BATCH_SPEC_EXECUTION),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: { data: { node: COMPLETED_BATCH_SPEC } },
        nMatches: Number.POSITIVE_INFINITY,
    },
]

add('completed', () => {
    console.log(mockWorkspaces(50, { state: BatchSpecWorkspaceState.FAILED, failureMessage: 'Uh oh!' }))
    return (
        <WebStory>
            {props => (
                <MockedTestProvider link={new WildcardMockLink([...COMMON_MOCKS, ...COMPLETED_MOCKS])}>
                    <ExecuteBatchSpecPage
                        {...props}
                        batchSpecID="spec1234"
                        batchChange={{ name: 'my-batch-change', namespace: 'user1234' }}
                        authenticatedUser={mockAuthenticatedUser}
                        settingsCascade={SETTINGS_CASCADE}
                    />
                </MockedTestProvider>
            )}
        </WebStory>
    )
})
