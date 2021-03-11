import { storiesOf } from '@storybook/react'
import React from 'react'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { UnpublishedNotice } from './UnpublishedNotice'

const { add } = storiesOf('web/batches/details/UnpublishedNotice', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

add('None published', () => (
    <EnterpriseWebStory>{() => <UnpublishedNotice unpublished={10} total={10} />}</EnterpriseWebStory>
))
