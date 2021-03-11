import { storiesOf } from '@storybook/react'
import { boolean } from '@storybook/addon-knobs'
import React from 'react'
import { BatchChangeNode } from './BatchChangeNode'
import isChromatic from 'chromatic/isChromatic'
import { subDays } from 'date-fns'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { nodes, now } from './testData'

const { add } = storiesOf('web/batches/BatchChangeNode', module).addDecorator(story => (
    <div className="p-3 container web-content batch-change-list-page__grid">{story()}</div>
))

for (const key of Object.keys(nodes)) {
    add(key, () => (
        <EnterpriseWebStory>
            {props => (
                <BatchChangeNode
                    {...props}
                    node={nodes[key]}
                    displayNamespace={boolean('Display namespace', true)}
                    now={isChromatic() ? () => subDays(now, 5) : undefined}
                />
            )}
        </EnterpriseWebStory>
    ))
}
