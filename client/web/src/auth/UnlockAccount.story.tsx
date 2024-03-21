import type { Meta, StoryFn } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import { WebStory } from '../components/WebStory'
import type { SourcegraphContext } from '../jscontext'

import { UnlockAccountPage } from './UnlockAccount'

const config: Meta = {
    title: 'web/auth/UnlockAccountPage',
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

const authProviders: SourcegraphContext['authProviders'] = [
    {
        clientID: '001',
        displayName: 'Builtin username-password authentication',
        isBuiltin: true,
        serviceType: 'builtin',
        authenticationURL: '',
        serviceID: '',
        noSignIn: false,
    },
    {
        clientID: '002',
        serviceType: 'github',
        displayName: 'GitHub',
        isBuiltin: false,
        authenticationURL: '/.auth/github/login?pc=f00bar',
        serviceID: 'https://github.com',
        noSignIn: false,
    },
    {
        clientID: '003',
        serviceType: 'gitlab',
        displayName: 'GitLab',
        isBuiltin: false,
        authenticationURL: '/.auth/gitlab/login?pc=f00bar',
        serviceID: 'https://gitlab.com',
        noSignIn: false,
    },
]

export const Default: StoryFn = () => (
    <WebStory>
        {() => (
            <UnlockAccountPage
                context={{
                    authProviders,
                    xhrHeaders: {},
                    allowSignup: true,
                    sourcegraphDotComMode: false,
                    resetPasswordEnabled: true,
                }}
                mockSuccess={true}
                authenticatedUser={null}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        )}
    </WebStory>
)
