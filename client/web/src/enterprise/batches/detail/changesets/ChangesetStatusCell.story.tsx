import { storiesOf } from '@storybook/react'
import { capitalize } from 'lodash'
import React from 'react'
import { ChangesetState } from '../../../../graphql-operations'
import { EnterpriseWebStory } from '../../../components/EnterpriseWebStory'
import { ChangesetStatusCell } from './ChangesetStatusCell'

const { add } = storiesOf('web/batches/ChangesetStatusCell', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

for (const state of Object.values(ChangesetState)) {
    add(capitalize(state), () => (
        <EnterpriseWebStory>
            {() => <ChangesetStatusCell state={state} className="d-flex text-muted" />}
        </EnterpriseWebStory>
    ))
}
