import type { Meta, StoryFn } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { WebStory } from '../components/WebStory'

import { VsCodeSignUpPage, type VsCodeSignUpPageProps } from './VsCodeSignUpPage'

const config: Meta = {
    title: 'web/auth/VsCodeSignUpPage',
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

const context: VsCodeSignUpPageProps['context'] = {
    authMinPasswordLength: 12,
    externalURL: 'https://sourcegraph.test:3443',
}

export const WithoutEmailForm: StoryFn = () => (
    <WebStory>
        {() => (
            <VsCodeSignUpPage
                onSignUp={async () => {}}
                showEmailForm={false}
                source="test"
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
                context={context}
            />
        )}
    </WebStory>
)

export const WithEmailForm: StoryFn = () => (
    <WebStory>
        {() => (
            <VsCodeSignUpPage
                onSignUp={async () => {}}
                showEmailForm={true}
                source="test"
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
                context={context}
            />
        )}
    </WebStory>
)
