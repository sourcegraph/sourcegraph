import { storiesOf } from '@storybook/react'
import React from 'react'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { ClosedNotice } from './ClosedNotice'

const { add } = storiesOf('web/batches/details/ClosedNotice', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

add('Batch change closed', () => (
    <EnterpriseWebStory>{() => <ClosedNotice closedAt="2021-02-02" />}</EnterpriseWebStory>
))
