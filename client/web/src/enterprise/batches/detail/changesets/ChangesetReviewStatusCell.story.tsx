import { storiesOf } from '@storybook/react'
import React from 'react'
import { ChangesetReviewStatusCell } from './ChangesetReviewStatusCell'
import { ChangesetReviewState } from '../../../../graphql-operations'
import { capitalize } from 'lodash'
import { EnterpriseWebStory } from '../../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/ChangesetReviewStatusCell', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

for (const state of Object.values(ChangesetReviewState)) {
    add(capitalize(state), () => (
        <EnterpriseWebStory>{props => <ChangesetReviewStatusCell {...props} reviewState={state} />}</EnterpriseWebStory>
    ))
}
