import type { Meta, StoryFn } from '@storybook/react'
import sinon from 'sinon'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { WebStory } from '../components/WebStory'
import type { SourcegraphContext } from '../jscontext'

import { CloudSignUpPage } from './CloudSignUpPage'

const config: Meta = {
    title: 'web/auth/CloudSignUpPage',
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

const context: Pick<SourcegraphContext, 'externalURL' | 'experimentalFeatures' | 'authMinPasswordLength'> = {
    experimentalFeatures: {},
    authMinPasswordLength: 0,
    externalURL: 'https://sourcegraph.test:3443',
}

export const Default: StoryFn = () => (
    <WebStory>
        {({ isLightTheme }) => (
            <CloudSignUpPage
                isLightTheme={isLightTheme}
                source="Monitor"
                onSignUp={sinon.stub()}
                context={context}
                showEmailForm={false}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
                isSourcegraphDotCom={true}
            />
        )}
    </WebStory>
)

export const EmailForm: StoryFn = () => (
    <WebStory>
        {({ isLightTheme }) => (
            <CloudSignUpPage
                isLightTheme={isLightTheme}
                source="SearchCTA"
                onSignUp={sinon.stub()}
                context={context}
                showEmailForm={true}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
                isSourcegraphDotCom={true}
            />
        )}
    </WebStory>
)

export const InvalidSource: StoryFn = () => (
    <WebStory>
        {({ isLightTheme }) => (
            <CloudSignUpPage
                isLightTheme={isLightTheme}
                source="test"
                onSignUp={sinon.stub()}
                context={context}
                showEmailForm={false}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
                isSourcegraphDotCom={true}
            />
        )}
    </WebStory>
)

export const OptimizationSignup: StoryFn = () => (
    <WebStory>
        {({ isLightTheme }) => (
            <CloudSignUpPage
                isLightTheme={isLightTheme}
                source="test"
                onSignUp={sinon.stub()}
                context={context}
                showEmailForm={false}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                telemetryRecorder={noOpTelemetryRecorder}
                isSourcegraphDotCom={true}
            />
        )}
    </WebStory>
)
