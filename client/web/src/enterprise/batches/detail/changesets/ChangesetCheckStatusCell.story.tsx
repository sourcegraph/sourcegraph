import { storiesOf } from '@storybook/react'
import React from 'react'
import { ChangesetCheckStatusCell } from './ChangesetCheckStatusCell'
import { ChangesetCheckState } from '../../../../graphql-operations'
import { capitalize } from 'lodash'
import { EnterpriseWebStory } from '../../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/ChangesetCheckStatusCell', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

for (const state of Object.values(ChangesetCheckState)) {
    add(capitalize(state), () => (
        <EnterpriseWebStory>{props => <ChangesetCheckStatusCell {...props} checkState={state} />}</EnterpriseWebStory>
    ))
}
