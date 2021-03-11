import { storiesOf } from '@storybook/react'
import { boolean } from '@storybook/addon-knobs'
import React from 'react'
import { ExternalChangesetNode } from './ExternalChangesetNode'
import { addHours } from 'date-fns'
import {
    ChangesetCheckState,
    ChangesetReviewState,
    ChangesetSpecType,
    ChangesetState,
} from '../../../../graphql-operations'
import { of } from 'rxjs'
import { EnterpriseWebStory } from '../../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/ExternalChangesetNode', module).addDecorator(story => (
    <div className="p-3 container web-content batch-change-changesets__grid">{story()}</div>
))

add('All states', () => {
    const now = new Date()
    return (
        <EnterpriseWebStory>
            {props => (
                <>
                    {Object.values(ChangesetState)
                        .filter(state => state !== ChangesetState.UNPUBLISHED)
                        .map((state, index) => (
                            <ExternalChangesetNode
                                key={index}
                                {...props}
                                node={{
                                    id: 'somechangeset',
                                    updatedAt: now.toISOString(),
                                    nextSyncAt: addHours(now, 1).toISOString(),
                                    state,
                                    __typename: 'ExternalChangeset',
                                    title: 'Changeset title on code host',
                                    error: null,
                                    syncerError: null,
                                    body: 'This changeset does the following things:\nIs awesome\nIs useful',
                                    checkState: ChangesetCheckState.PENDING,
                                    createdAt: now.toISOString(),
                                    externalID: '123',
                                    externalURL: {
                                        url: 'http://test.test/pr/123',
                                    },
                                    diffStat: {
                                        added: 10,
                                        changed: 20,
                                        deleted: 8,
                                    },
                                    labels: [],
                                    repository: {
                                        id: 'repoid',
                                        name: 'github.com/sourcegraph/sourcegraph',
                                        url: 'http://test.test/sourcegraph/sourcegraph',
                                    },
                                    reviewState: ChangesetReviewState.COMMENTED,
                                    currentSpec: {
                                        id: 'spec-rand-id-1',
                                        type: ChangesetSpecType.BRANCH,
                                        description: {
                                            __typename: 'GitBranchChangesetDescription',
                                            headRef: 'my-branch',
                                        },
                                    },
                                }}
                                viewerCanAdminister={boolean('viewerCanAdminister', true)}
                                queryExternalChangesetWithFileDiffs={() =>
                                    of({
                                        diff: {
                                            __typename: 'PreviewRepositoryComparison',
                                            fileDiffs: {
                                                nodes: [],
                                                totalCount: 0,
                                                pageInfo: {
                                                    endCursor: null,
                                                    hasNextPage: false,
                                                },
                                            },
                                        },
                                    })
                                }
                            />
                        ))}
                </>
            )}
        </EnterpriseWebStory>
    )
})

add('Unpublished', () => {
    const now = new Date()
    return (
        <EnterpriseWebStory>
            {props => (
                <ExternalChangesetNode
                    {...props}
                    node={{
                        __typename: 'ExternalChangeset',
                        id: 'somechangeset',
                        updatedAt: now.toISOString(),
                        nextSyncAt: null,
                        state: ChangesetState.UNPUBLISHED,
                        title: 'Changeset title on code host',
                        error: null,
                        syncerError: null,
                        body: 'This changeset does the following things:\nIs awesome\nIs useful',
                        checkState: null,
                        createdAt: now.toISOString(),
                        externalID: null,
                        externalURL: null,
                        diffStat: {
                            added: 10,
                            changed: 20,
                            deleted: 8,
                        },
                        labels: [],
                        repository: {
                            id: 'repoid',
                            name: 'github.com/sourcegraph/sourcegraph',
                            url: 'http://test.test/sourcegraph/sourcegraph',
                        },
                        reviewState: null,
                        currentSpec: {
                            id: 'spec-rand-id-1',
                            type: ChangesetSpecType.BRANCH,
                            description: {
                                __typename: 'GitBranchChangesetDescription',
                                headRef: 'my-branch',
                            },
                        },
                    }}
                    viewerCanAdminister={boolean('viewerCanAdminister', true)}
                    queryExternalChangesetWithFileDiffs={() =>
                        of({
                            diff: {
                                __typename: 'PreviewRepositoryComparison',
                                fileDiffs: {
                                    nodes: [],
                                    totalCount: 0,
                                    pageInfo: {
                                        endCursor: null,
                                        hasNextPage: false,
                                    },
                                },
                            },
                        })
                    }
                />
            )}
        </EnterpriseWebStory>
    )
})

add('Importing', () => {
    const now = new Date()
    return (
        <EnterpriseWebStory>
            {props => (
                <ExternalChangesetNode
                    {...props}
                    node={{
                        __typename: 'ExternalChangeset',
                        id: 'somechangeset',
                        updatedAt: now.toISOString(),
                        nextSyncAt: null,
                        state: ChangesetState.PROCESSING,
                        // No title yet, still importing.
                        title: null,
                        error: null,
                        syncerError: null,
                        body: null,
                        checkState: null,
                        createdAt: now.toISOString(),
                        externalID: '12345',
                        externalURL: null,
                        diffStat: null,
                        labels: [],
                        repository: {
                            id: 'repoid',
                            name: 'github.com/sourcegraph/sourcegraph',
                            url: 'http://test.test/sourcegraph/sourcegraph',
                        },
                        reviewState: null,
                        currentSpec: null,
                    }}
                    viewerCanAdminister={boolean('viewerCanAdminister', true)}
                    queryExternalChangesetWithFileDiffs={() =>
                        of({
                            diff: {
                                __typename: 'PreviewRepositoryComparison',
                                fileDiffs: {
                                    nodes: [],
                                    totalCount: 0,
                                    pageInfo: {
                                        endCursor: null,
                                        hasNextPage: false,
                                    },
                                },
                            },
                        })
                    }
                />
            )}
        </EnterpriseWebStory>
    )
})

add('Importing failed', () => {
    const now = new Date()
    return (
        <EnterpriseWebStory>
            {props => (
                <ExternalChangesetNode
                    {...props}
                    node={{
                        __typename: 'ExternalChangeset',
                        id: 'somechangeset-2',
                        updatedAt: now.toISOString(),
                        nextSyncAt: null,
                        state: ChangesetState.FAILED,
                        // No title, because it wasn't found.
                        title: null,
                        error: 'Changeset with external ID 99999 not found',
                        syncerError: null,
                        body: null,
                        checkState: null,
                        createdAt: now.toISOString(),
                        externalID: '99999',
                        externalURL: null,
                        diffStat: null,
                        labels: [],
                        repository: {
                            id: 'repoid',
                            name: 'github.com/sourcegraph/sourcegraph',
                            url: 'http://test.test/sourcegraph/sourcegraph',
                        },
                        reviewState: null,
                        currentSpec: null,
                    }}
                    viewerCanAdminister={boolean('viewerCanAdminister', true)}
                    queryExternalChangesetWithFileDiffs={() =>
                        of({
                            diff: null,
                        })
                    }
                />
            )}
        </EnterpriseWebStory>
    )
})

add('Sync failed', () => {
    const now = new Date()
    return (
        <EnterpriseWebStory>
            {props => (
                <ExternalChangesetNode
                    {...props}
                    node={{
                        __typename: 'ExternalChangeset',
                        id: 'somechangeset-2',
                        updatedAt: now.toISOString(),
                        nextSyncAt: null,
                        state: ChangesetState.FAILED,
                        // No title, because it wasn't found.
                        title: null,
                        error: null,
                        syncerError: 'Invalid token, cannot load PR.',
                        body: null,
                        checkState: null,
                        createdAt: now.toISOString(),
                        externalID: '99999',
                        externalURL: null,
                        diffStat: null,
                        labels: [],
                        repository: {
                            id: 'repoid',
                            name: 'github.com/sourcegraph/sourcegraph',
                            url: 'http://test.test/sourcegraph/sourcegraph',
                        },
                        reviewState: null,
                        currentSpec: null,
                    }}
                    viewerCanAdminister={boolean('viewerCanAdminister', true)}
                    queryExternalChangesetWithFileDiffs={() =>
                        of({
                            diff: null,
                        })
                    }
                />
            )}
        </EnterpriseWebStory>
    )
})
