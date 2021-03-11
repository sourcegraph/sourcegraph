import { storiesOf } from '@storybook/react'
import React from 'react'
import { ChangesetApplyPreviewNode } from './ChangesetApplyPreviewNode'
import { visibleChangesetApplyPreviewNodeStories } from './VisibleChangesetApplyPreviewNode.story'
import { hiddenChangesetApplyPreviewStories } from './HiddenChangesetApplyPreviewNode.story'
import { of } from 'rxjs'
import { EnterpriseWebStory } from '../../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/preview/ChangesetApplyPreviewNode', module).addDecorator(story => (
    <div className="p-3 container web-content preview-list__grid">{story()}</div>
))

const queryEmptyFileDiffs = () => of({ totalCount: 0, pageInfo: { endCursor: null, hasNextPage: false }, nodes: [] })

add('Overview', () => {
    const nodes = [
        ...Object.values(visibleChangesetApplyPreviewNodeStories),
        ...Object.values(hiddenChangesetApplyPreviewStories),
    ]
    return (
        <EnterpriseWebStory>
            {props => (
                <>
                    {nodes.map((node, index) => (
                        <ChangesetApplyPreviewNode
                            {...props}
                            key={index}
                            node={node}
                            authenticatedUser={{
                                url: '/users/alice',
                                displayName: 'Alice',
                                username: 'alice',
                                email: 'alice@email.test',
                            }}
                            queryChangesetSpecFileDiffs={queryEmptyFileDiffs}
                        />
                    ))}
                </>
            )}
        </EnterpriseWebStory>
    )
})
