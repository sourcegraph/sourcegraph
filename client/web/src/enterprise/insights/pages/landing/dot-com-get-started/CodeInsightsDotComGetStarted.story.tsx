import type { Meta } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { WebStory } from '../../../../../components/WebStory'

import { CodeInsightsDotComGetStarted } from './CodeInsightsDotComGetStarted'

const meta: Meta = {
    title: 'web/insights/dot-com-landing/CodeInsightsDotComGetStarted',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
}

export default meta

export const CodeInsightsDotComGetStartedStory = () => (
    <CodeInsightsDotComGetStarted
        telemetryService={NOOP_TELEMETRY_SERVICE}
        authenticatedUser={null}
        telemetryRecorder={noOpTelemetryRecorder}
    />
)
