import type { Meta } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { WebStory } from '../../../../../../../components/WebStory'

import { CodeInsightsExamples } from './CodeInsightsExamples'

const meta: Meta = {
    title: 'web/insights/getting-started/CodeInsightExamples',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
}

export default meta

export const StandardExample = () => (
    <CodeInsightsExamples telemetryService={NOOP_TELEMETRY_SERVICE} telemetryRecorder={noOpTelemetryRecorder} />
)
