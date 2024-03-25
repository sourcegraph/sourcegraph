import type { Meta, StoryFn } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { WebStory } from '../../../../../../components/WebStory'
import { CodeInsightsBackendContext, CodeInsightsGqlBackend } from '../../../../core'
import { useCodeInsightsLicenseState } from '../../../../stores'

import { IntroCreationPage } from './IntroCreationPage'

const config: Meta = {
    title: 'web/insights/creation-ui/IntroPage',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
    parameters: {
        chromatic: {
            viewports: [576, 978, 1440],
            disableSnapshot: false,
        },
    },
}

export default config

const API = new CodeInsightsGqlBackend({} as any)

export const IntroPageLicensed: StoryFn = () => {
    useCodeInsightsLicenseState.setState({ licensed: true, insightsLimit: null })

    return (
        <CodeInsightsBackendContext.Provider value={API}>
            <IntroCreationPage telemetryService={NOOP_TELEMETRY_SERVICE} telemetryRecorder={noOpTelemetryRecorder} />
        </CodeInsightsBackendContext.Provider>
    )
}

export const IntroPageUnLicensed: StoryFn = () => {
    useCodeInsightsLicenseState.setState({ licensed: false, insightsLimit: 2 })

    return (
        <CodeInsightsBackendContext.Provider value={API}>
            <IntroCreationPage telemetryService={NOOP_TELEMETRY_SERVICE} telemetryRecorder={noOpTelemetryRecorder} />
        </CodeInsightsBackendContext.Provider>
    )
}
