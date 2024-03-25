import type { Meta } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { WebStory } from '../../../../../../../../components/WebStory'

import { CodeInsightsExamplesSlider } from './CodeInsightsExamplesSlider'

const meta: Meta = {
    title: 'web/insights/dot-com-landing/CodeInsightsExampleSlider',
    decorators: [story => <WebStory>{() => story()}</WebStory>],
}

export default meta

export const CodeInsightsExampleSliderExample = () => (
    <div style={{ width: 500 }}>
        <CodeInsightsExamplesSlider
            telemetryService={NOOP_TELEMETRY_SERVICE}
            telemetryRecorder={noOpTelemetryRecorder}
        />
    </div>
)
