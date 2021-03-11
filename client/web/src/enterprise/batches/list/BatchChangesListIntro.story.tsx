import { storiesOf } from '@storybook/react'
import { radios } from '@storybook/addon-knobs'
import React from 'react'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { BatchChangesListIntro } from './BatchChangesListIntro'

const { add } = storiesOf('web/batches/BatchChangesListIntro', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

enum LicensingState {
    Licensed = 'Licensed',
    Unlicensed = 'Unlicensed',
    Loading = 'Loading',
}

function stateToInput(state: LicensingState): boolean | undefined {
    switch (state) {
        case LicensingState.Licensed:
            return true
        case LicensingState.Unlicensed:
            return false
        default:
            return undefined
    }
}

for (const state of Object.values(LicensingState)) {
    add(state, () => (
        <EnterpriseWebStory>
            {() => <BatchChangesListIntro licensed={stateToInput(radios('licensed', LicensingState, state))} />}
        </EnterpriseWebStory>
    ))
}
