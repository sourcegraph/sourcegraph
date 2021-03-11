import { storiesOf } from '@storybook/react'
import React from 'react'
import { EnterpriseWebStory } from '../components/EnterpriseWebStory'
import { Description } from './Description'

const { add } = storiesOf('web/batches/Description', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

add('Overview', () => (
    <EnterpriseWebStory>
        {props => (
            <Description
                {...props}
                description="This is an awesome batch change. It will do great things to your codebase."
            />
        )}
    </EnterpriseWebStory>
))
