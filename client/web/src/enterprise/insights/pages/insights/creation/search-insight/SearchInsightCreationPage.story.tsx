import type { Meta, StoryFn } from '@storybook/react'
import delay from 'delay'
import { noop } from 'lodash'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { WebStory } from '../../../../../../components/WebStory'
import { useCodeInsightsLicenseState } from '../../../../stores'

import { SearchInsightCreationPage as SearchInsightCreationPageComponent } from './SearchInsightCreationPage'

const defaultStory: Meta = {
    title: 'web/insights/creation-ui/search/SearchInsightCreationPage',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
    parameters: {
        chromatic: {
            viewports: [576, 1440],
            disableSnapshot: false,
        },
    },
}

export default defaultStory

const fakeAPIRequest = async () => {
    await delay(1000)

    throw new Error('Network error')
}

export const SearchInsightCreationPage: StoryFn = () => {
    useCodeInsightsLicenseState.setState({ licensed: true, insightsLimit: null })

    return (
        <SearchInsightCreationPageComponent
            backUrl="/insights/create"
            telemetryService={NOOP_TELEMETRY_SERVICE}
            onInsightCreateRequest={fakeAPIRequest}
            onSuccessfulCreation={noop}
            onCancel={noop}
            telemetryRecorder={noOpTelemetryRecorder}
        />
    )
}
