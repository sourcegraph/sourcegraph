import { storiesOf } from '@storybook/react'
import React from 'react'
import { HiddenChangesetApplyPreviewNode } from './HiddenChangesetApplyPreviewNode'
import { ChangesetSpecType, HiddenChangesetApplyPreviewFields } from '../../../../graphql-operations'
import { EnterpriseWebStory } from '../../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/preview/HiddenChangesetApplyPreviewNode', module).addDecorator(story => (
    <div className="p-3 container web-content preview-list__grid">{story()}</div>
))

export const hiddenChangesetApplyPreviewStories: Record<string, HiddenChangesetApplyPreviewFields> = {
    'Import changeset': {
        __typename: 'HiddenChangesetApplyPreview',
        targets: {
            __typename: 'HiddenApplyPreviewTargetsAttach',
            changesetSpec: {
                __typename: 'HiddenChangesetSpec',
                id: 'someidh1',
                type: ChangesetSpecType.EXISTING,
            },
        },
    },
    'Create changeset': {
        __typename: 'HiddenChangesetApplyPreview',
        targets: {
            __typename: 'HiddenApplyPreviewTargetsAttach',
            changesetSpec: {
                __typename: 'HiddenChangesetSpec',
                id: 'someidh2',
                type: ChangesetSpecType.BRANCH,
            },
        },
    },
}

for (const storyName of Object.keys(hiddenChangesetApplyPreviewStories)) {
    add(storyName, () => (
        <EnterpriseWebStory>
            {props => (
                <HiddenChangesetApplyPreviewNode {...props} node={hiddenChangesetApplyPreviewStories[storyName]} />
            )}
        </EnterpriseWebStory>
    ))
}
