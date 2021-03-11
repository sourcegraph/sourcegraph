import { storiesOf } from '@storybook/react'
import React from 'react'
import { VisibleChangesetApplyPreviewNode } from './VisibleChangesetApplyPreviewNode'
import {
    VisibleChangesetSpecFields,
    ChangesetSpecType,
    Scalars,
    VisibleChangesetApplyPreviewFields,
} from '../../../../graphql-operations'
import { of } from 'rxjs'
import { EnterpriseWebStory } from '../../../components/EnterpriseWebStory'
import { ChangesetSpecOperation, ChangesetState } from '../../../../../../shared/src/graphql-operations'

const { add } = storiesOf('web/batches/preview/VisibleChangesetApplyPreviewNode', module).addDecorator(story => (
    <div className="p-3 container web-content preview-list__grid">{story()}</div>
))

const testRepo = { name: 'github.com/sourcegraph/testrepo', url: 'https://test.test/repo' }

function baseChangesetSpec(
    published: Scalars['PublishedValue'],
    overrides: Partial<VisibleChangesetSpecFields> = {}
): VisibleChangesetSpecFields {
    return {
        __typename: 'VisibleChangesetSpec',
        id: 'someidv2' + String(published) + JSON.stringify(overrides),
        type: ChangesetSpecType.EXISTING,
        description: {
            __typename: 'GitBranchChangesetDescription',
            baseRepository: testRepo,
            baseRef: 'master',
            headRef: 'cool-branch',
            body: 'Body text',
            commits: [
                {
                    subject: 'This is the first line of the commit message',
                    body: `And the more explanatory body. And the more explanatory body.
And the more explanatory body. And the more explanatory body.
And the more explanatory body. And the more explanatory body.
And the more explanatory body. And the more explanatory body. And the more explanatory body.
And the more explanatory body. And the more explanatory body. And the more explanatory body.`,
                    author: {
                        avatarURL: null,
                        displayName: 'john',
                        email: 'john@test.not',
                        user: { displayName: 'lejohn', url: '/users/lejohn', username: 'john' },
                    },
                },
            ],
            diffStat: {
                added: 10,
                changed: 8,
                deleted: 2,
            },
            title: 'Add prettier to repository',
            published,
        },
        ...overrides,
    }
}

export const visibleChangesetApplyPreviewNodeStories: Record<string, VisibleChangesetApplyPreviewFields> = {
    'Import changeset': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.IMPORT],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsAttach',
            changesetSpec: {
                __typename: 'VisibleChangesetSpec',
                id: 'someidv1',
                type: ChangesetSpecType.EXISTING,
                description: {
                    __typename: 'ExistingChangesetReference',
                    baseRepository: { name: 'github.com/sourcegraph/testrepo', url: 'https://test.test/repo' },
                    externalID: '123',
                },
            },
        },
    },
    'Create changeset published': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.PUSH, ChangesetSpecOperation.PUBLISH],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsAttach',
            changesetSpec: baseChangesetSpec(true),
        },
    },
    'Create changeset draft': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.PUSH, ChangesetSpecOperation.PUBLISH_DRAFT],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsAttach',
            changesetSpec: baseChangesetSpec('draft'),
        },
    },
    'Create changeset not published': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsAttach',
            changesetSpec: baseChangesetSpec(false),
        },
    },
    'Update changeset title': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.UPDATE],
        delta: {
            titleChanged: true,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsUpdate',
            changesetSpec: baseChangesetSpec(true),
            changeset: {
                id: '123123',
                title: 'the old title',
                state: ChangesetState.OPEN,
                currentSpec: {
                    description: {
                        __typename: 'GitBranchChangesetDescription',
                        baseRef: 'main',
                        body: 'body',
                        commits: [
                            {
                                subject: 'Abc',
                                body: null,
                                author: {
                                    avatarURL: null,
                                    displayName: 'alice',
                                    email: 'alice@sourcegraph.test',
                                    user: null,
                                },
                            },
                        ],
                        title: 'Title',
                    },
                },
                author: {
                    displayName: 'Alice',
                    email: 'alice@email.test',
                    user: {
                        displayName: 'Alice',
                        url: '/users/alice',
                        username: 'alice',
                    },
                },
            },
        },
    },
    'Update changeset body': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.UPDATE],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: true,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsUpdate',
            changesetSpec: baseChangesetSpec(true),
            changeset: {
                id: '123123',
                title: 'the old title',
                state: ChangesetState.OPEN,
                currentSpec: {
                    description: {
                        __typename: 'GitBranchChangesetDescription',
                        baseRef: 'main',
                        body: 'body',
                        commits: [
                            {
                                subject: 'Abc',
                                body: null,
                                author: {
                                    avatarURL: null,
                                    displayName: 'alice',
                                    email: 'alice@sourcegraph.test',
                                    user: null,
                                },
                            },
                        ],
                        title: 'Title',
                    },
                },
                author: {
                    displayName: 'Alice',
                    email: 'alice@email.test',
                    user: {
                        displayName: 'Alice',
                        url: '/users/alice',
                        username: 'alice',
                    },
                },
            },
        },
    },
    'Undraft changeset': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.UNDRAFT],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsUpdate',
            changesetSpec: baseChangesetSpec(true),
            changeset: {
                id: '123123',
                title: 'Le draft changeset',
                state: ChangesetState.OPEN,
                currentSpec: {
                    description: {
                        __typename: 'GitBranchChangesetDescription',
                        baseRef: 'main',
                        body: 'body',
                        commits: [
                            {
                                subject: 'Abc',
                                body: null,
                                author: {
                                    avatarURL: null,
                                    displayName: 'alice',
                                    email: 'alice@sourcegraph.test',
                                    user: null,
                                },
                            },
                        ],
                        title: 'Title',
                    },
                },
                author: {
                    displayName: 'Alice',
                    email: 'alice@email.test',
                    user: {
                        displayName: 'Alice',
                        url: '/users/alice',
                        username: 'alice',
                    },
                },
            },
        },
    },
    'Reopen changeset': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.REOPEN, ChangesetSpecOperation.UPDATE],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsUpdate',
            changesetSpec: baseChangesetSpec(true),
            changeset: {
                id: '123123',
                title: 'Le closed changeset',
                state: ChangesetState.OPEN,
                currentSpec: {
                    description: {
                        __typename: 'GitBranchChangesetDescription',
                        baseRef: 'main',
                        body: 'body',
                        commits: [
                            {
                                subject: 'Abc',
                                body: null,
                                author: {
                                    avatarURL: null,
                                    displayName: 'alice',
                                    email: 'alice@sourcegraph.test',
                                    user: null,
                                },
                            },
                        ],
                        title: 'Title',
                    },
                },
                author: {
                    displayName: 'Alice',
                    email: 'alice@email.test',
                    user: {
                        displayName: 'Alice',
                        url: '/users/alice',
                        username: 'alice',
                    },
                },
            },
        },
    },
    'Close changeset': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.CLOSE, ChangesetSpecOperation.DETACH],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsDetach',
            changeset: {
                id: '123123',
                title: 'Le open changeset',
                state: ChangesetState.OPEN,
                repository: testRepo,
                diffStat: {
                    added: 2,
                    changed: 8,
                    deleted: 10,
                },
            },
        },
    },
    'Detach changeset': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.DETACH],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsDetach',
            changeset: {
                id: '123123',
                title: 'Le open changeset',
                state: ChangesetState.OPEN,
                repository: testRepo,
                diffStat: {
                    added: 2,
                    changed: 8,
                    deleted: 10,
                },
            },
        },
    },
    'Change base ref': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.UPDATE],
        delta: {
            titleChanged: false,
            baseRefChanged: true,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsUpdate',
            changesetSpec: baseChangesetSpec(true),
            changeset: {
                id: '123123',
                title: 'Change base ref',
                state: ChangesetState.OPEN,
                currentSpec: {
                    description: {
                        __typename: 'GitBranchChangesetDescription',
                        baseRef: 'main',
                        body: 'body',
                        commits: [
                            {
                                subject: 'Abc',
                                body: null,
                                author: {
                                    avatarURL: null,
                                    displayName: 'alice',
                                    email: 'alice@sourcegraph.test',
                                    user: null,
                                },
                            },
                        ],
                        title: 'Title',
                    },
                },
                author: {
                    displayName: 'Alice',
                    email: 'alice@email.test',
                    user: {
                        displayName: 'Alice',
                        url: '/users/alice',
                        username: 'alice',
                    },
                },
            },
        },
    },
    'Change diff': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.UPDATE],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: true,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsUpdate',
            changesetSpec: baseChangesetSpec(true),
            changeset: {
                id: '123123',
                title: 'Change base ref',
                state: ChangesetState.OPEN,
                currentSpec: {
                    description: {
                        __typename: 'GitBranchChangesetDescription',
                        baseRef: 'master',
                        body: 'body',
                        commits: [
                            {
                                subject: 'Abc',
                                body: null,
                                author: {
                                    avatarURL: null,
                                    displayName: 'alice',
                                    email: 'alice@sourcegraph.test',
                                    user: null,
                                },
                            },
                        ],
                        title: 'Title',
                    },
                },
                author: {
                    displayName: 'Alice',
                    email: 'alice@email.test',
                    user: {
                        displayName: 'Alice',
                        url: '/users/alice',
                        username: 'alice',
                    },
                },
            },
        },
    },
    'Update commit message': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.PUSH],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: false,
            authorNameChanged: false,
            commitMessageChanged: true,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsUpdate',
            changesetSpec: baseChangesetSpec(true),
            changeset: {
                id: '123123',
                title: 'the old title',
                state: ChangesetState.OPEN,
                currentSpec: {
                    description: {
                        __typename: 'GitBranchChangesetDescription',
                        baseRef: 'main',
                        body: 'body',
                        commits: [
                            {
                                subject: 'Abc',
                                body: 'Current commit message',
                                author: {
                                    avatarURL: null,
                                    displayName: 'alice',
                                    email: 'alice@sourcegraph.test',
                                    user: null,
                                },
                            },
                        ],
                        title: 'Title',
                    },
                },
                author: {
                    displayName: 'Alice',
                    email: 'alice@email.test',
                    user: {
                        displayName: 'Alice',
                        url: '/users/alice',
                        username: 'alice',
                    },
                },
            },
        },
    },
    'Update commit author': {
        __typename: 'VisibleChangesetApplyPreview',
        operations: [ChangesetSpecOperation.PUSH],
        delta: {
            titleChanged: false,
            baseRefChanged: false,
            diffChanged: false,
            bodyChanged: false,
            authorEmailChanged: true,
            authorNameChanged: true,
            commitMessageChanged: false,
        },
        targets: {
            __typename: 'VisibleApplyPreviewTargetsUpdate',
            changesetSpec: baseChangesetSpec(true),
            changeset: {
                id: '123123',
                title: 'the old title',
                state: ChangesetState.OPEN,
                currentSpec: {
                    description: {
                        __typename: 'GitBranchChangesetDescription',
                        baseRef: 'main',
                        body: 'body',
                        commits: [
                            {
                                subject: 'Abc',
                                body: 'Current commit message',
                                author: {
                                    avatarURL: null,
                                    displayName: 'alice',
                                    email: 'alice@sourcegraph.test',
                                    user: null,
                                },
                            },
                        ],
                        title: 'Title',
                    },
                },
                author: {
                    displayName: 'Bob',
                    email: 'bob@email.test',
                    user: {
                        displayName: 'Bob',
                        url: '/users/bob',
                        username: 'bob',
                    },
                },
            },
        },
    },
}

const queryEmptyFileDiffs = () => of({ totalCount: 0, pageInfo: { endCursor: null, hasNextPage: false }, nodes: [] })

for (const storyName of Object.keys(visibleChangesetApplyPreviewNodeStories)) {
    add(storyName, () => (
        <EnterpriseWebStory>
            {props => (
                <VisibleChangesetApplyPreviewNode
                    {...props}
                    node={visibleChangesetApplyPreviewNodeStories[storyName]}
                    authenticatedUser={{
                        url: '/users/alice',
                        displayName: 'Alice',
                        username: 'alice',
                        email: 'alice@email.test',
                    }}
                    queryChangesetSpecFileDiffs={queryEmptyFileDiffs}
                />
            )}
        </EnterpriseWebStory>
    ))
}
