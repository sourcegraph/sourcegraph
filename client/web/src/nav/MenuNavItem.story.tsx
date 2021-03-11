import { storiesOf } from '@storybook/react'
import React from 'react'
import { WebStory } from '../components/WebStory'
import { BatchChangesNavItem } from '../enterprise/batches/global/nav/BatchChangesNavItem'
import { CodeMonitoringNavItem } from '../enterprise/code-monitoring/CodeMonitoringNavItem'
import { InsightsNavItem } from '../insights/InsightsNavLink'
import { MenuNavItem } from './MenuNavItem'

const { add } = storiesOf('web/nav/MenuNavItem', module)

add(
    'Menu',
    () => (
        <WebStory>
            {() => (
                <MenuNavItem openByDefault={true}>
                    <BatchChangesNavItem />
                    <InsightsNavItem />
                    <CodeMonitoringNavItem />
                </MenuNavItem>
            )}
        </WebStory>
    ),
    {
        design: {
            type: 'figma',
            url: 'https://www.figma.com/file/SFhXbl23TJ2j5tOF51NDtF/%F0%9F%93%9AWeb?node-id=1108%3A872',
        },
    }
)
