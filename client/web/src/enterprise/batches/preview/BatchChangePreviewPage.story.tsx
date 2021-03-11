import { storiesOf } from '@storybook/react'
import { boolean } from '@storybook/addon-knobs'
import React from 'react'
import { BatchChangePreviewPage } from './BatchChangePreviewPage'
import { of, Observable } from 'rxjs'
import {
    BatchSpecApplyPreviewConnectionFields,
    BatchSpecFields,
    ChangesetApplyPreviewFields,
    ExternalServiceKind,
} from '../../../graphql-operations'
import { visibleChangesetApplyPreviewNodeStories } from './list/VisibleChangesetApplyPreviewNode.story'
import { hiddenChangesetApplyPreviewStories } from './list/HiddenChangesetApplyPreviewNode.story'
import { fetchBatchSpecById } from './backend'
import { addDays, subDays } from 'date-fns'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/preview/BatchChangePreviewPage', module)
    .addDecorator(story => <div className="p-3 container web-content">{story()}</div>)
    .addParameters({
        chromatic: {
            viewports: [320, 576, 978, 1440],
        },
    })

const nodes: ChangesetApplyPreviewFields[] = [
    ...Object.values(visibleChangesetApplyPreviewNodeStories),
    ...Object.values(hiddenChangesetApplyPreviewStories),
]

const batchSpec = (): BatchSpecFields => ({
    appliesToBatchChange: null,
    createdAt: subDays(new Date(), 5).toISOString(),
    creator: {
        url: '/users/alice',
        username: 'alice',
    },
    description: {
        name: 'awesome-batch-change',
        description: 'This is the description',
    },
    diffStat: {
        added: 10,
        changed: 8,
        deleted: 10,
    },
    expiresAt: addDays(new Date(), 7).toISOString(),
    id: 'specid',
    namespace: {
        namespaceName: 'alice',
        url: '/users/alice',
    },
    supersedingBatchSpec: boolean('supersedingBatchSpec', false)
        ? {
              createdAt: subDays(new Date(), 1).toISOString(),
              applyURL: '/users/alice/batch-changes/apply/newspecid',
          }
        : null,
    viewerCanAdminister: boolean('viewerCanAdminister', true),
    viewerBatchChangesCodeHosts: {
        totalCount: 0,
        nodes: [],
    },
    applyPreview: {
        stats: {
            close: 10,
            detach: 10,
            import: 10,
            publish: 10,
            publishDraft: 10,
            push: 10,
            reopen: 10,
            undraft: 10,
            update: 10,

            added: 5,
            modified: 10,
            removed: 3,
        },
    },
})

const fetchBatchSpecCreate: typeof fetchBatchSpecById = () => of(batchSpec())

const fetchBatchSpecMissingCredentials: typeof fetchBatchSpecById = () =>
    of({
        ...batchSpec(),
        viewerBatchChangesCodeHosts: {
            totalCount: 2,
            nodes: [
                {
                    externalServiceKind: ExternalServiceKind.GITHUB,
                    externalServiceURL: 'https://github.com/',
                },
                {
                    externalServiceKind: ExternalServiceKind.GITLAB,
                    externalServiceURL: 'https://gitlab.com/',
                },
            ],
        },
    })

const fetchBatchSpecUpdate: typeof fetchBatchSpecById = () =>
    of({
        ...batchSpec(),
        appliesToBatchChange: {
            id: 'somebatch',
            name: 'awesome-batch-change',
            url: '/users/alice/batch-changes/awesome-batch-change',
        },
    })

const queryChangesetApplyPreview = (): Observable<BatchSpecApplyPreviewConnectionFields> =>
    of({
        pageInfo: {
            endCursor: null,
            hasNextPage: false,
        },
        totalCount: nodes.length,
        nodes,
    })

const queryEmptyChangesetApplyPreview = (): Observable<BatchSpecApplyPreviewConnectionFields> =>
    of({
        pageInfo: {
            endCursor: null,
            hasNextPage: false,
        },
        totalCount: 0,
        nodes: [],
    })

const queryEmptyFileDiffs = () => of({ totalCount: 0, pageInfo: { endCursor: null, hasNextPage: false }, nodes: [] })

add('Create', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangePreviewPage
                {...props}
                expandChangesetDescriptions={true}
                batchSpecID="123123"
                fetchBatchSpecById={fetchBatchSpecCreate}
                queryChangesetApplyPreview={queryChangesetApplyPreview}
                queryChangesetSpecFileDiffs={queryEmptyFileDiffs}
                authenticatedUser={{
                    url: '/users/alice',
                    displayName: 'Alice',
                    username: 'alice',
                    email: 'alice@email.test',
                }}
            />
        )}
    </EnterpriseWebStory>
))

add('Update', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangePreviewPage
                {...props}
                expandChangesetDescriptions={true}
                batchSpecID="123123"
                fetchBatchSpecById={fetchBatchSpecUpdate}
                queryChangesetApplyPreview={queryChangesetApplyPreview}
                queryChangesetSpecFileDiffs={queryEmptyFileDiffs}
                authenticatedUser={{
                    url: '/users/alice',
                    displayName: 'Alice',
                    username: 'alice',
                    email: 'alice@email.test',
                }}
            />
        )}
    </EnterpriseWebStory>
))

add('Missing credentials', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangePreviewPage
                {...props}
                expandChangesetDescriptions={true}
                batchSpecID="123123"
                fetchBatchSpecById={fetchBatchSpecMissingCredentials}
                queryChangesetApplyPreview={queryChangesetApplyPreview}
                queryChangesetSpecFileDiffs={queryEmptyFileDiffs}
                authenticatedUser={{
                    url: '/users/alice',
                    displayName: 'Alice',
                    username: 'alice',
                    email: 'alice@email.test',
                }}
            />
        )}
    </EnterpriseWebStory>
))

add('No changesets', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangePreviewPage
                {...props}
                expandChangesetDescriptions={true}
                batchSpecID="123123"
                fetchBatchSpecById={fetchBatchSpecCreate}
                queryChangesetApplyPreview={queryEmptyChangesetApplyPreview}
                queryChangesetSpecFileDiffs={queryEmptyFileDiffs}
                authenticatedUser={{
                    url: '/users/alice',
                    displayName: 'Alice',
                    username: 'alice',
                    email: 'alice@email.test',
                }}
            />
        )}
    </EnterpriseWebStory>
))
