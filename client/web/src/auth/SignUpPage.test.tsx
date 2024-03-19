import { Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import type { AuthenticatedUser } from '../auth'
import type { SourcegraphContext } from '../jscontext'

import { SignUpPage } from './SignUpPage'

describe('SignUpPage', () => {
    const commonProps = {
        isLightTheme: true,
    }
    const authProviders: SourcegraphContext['authProviders'] = [
        {
            displayName: 'Builtin username-password authentication',
            isBuiltin: true,
            serviceType: 'builtin',
            authenticationURL: '',
            serviceID: '',
            clientID: '',
        },
        {
            serviceType: 'github',
            displayName: 'GitHub',
            isBuiltin: false,
            authenticationURL: '/.auth/github/login?pc=f00bar',
            serviceID: 'https://github.com',
            clientID: '1234',
        },
    ]

    it('renders sign up page (server)', () => {
        expect(
            renderWithBrandedContext(
                <MockedTestProvider mocks={[]}>
                    <Routes>
                        <Route
                            path="/sign-up"
                            element={
                                <SignUpPage
                                    {...commonProps}
                                    authenticatedUser={null}
                                    context={{
                                        allowSignup: true,
                                        sourcegraphDotComMode: false,
                                        authMinPasswordLength: 12,
                                        authProviders,
                                        xhrHeaders: {},
                                        externalURL: 'https://sourcegraph.test:3443',
                                    }}
                                    telemetryService={NOOP_TELEMETRY_SERVICE}
                                    telemetryRecorder={noOpTelemetryRecorder}
                                />
                            }
                        />
                    </Routes>
                </MockedTestProvider>,
                { route: '/sign-up' }
            ).asFragment()
        ).toMatchSnapshot()
    })

    it('renders sign up page (cloud)', () => {
        expect(
            renderWithBrandedContext(
                <MockedTestProvider mocks={[]}>
                    <Routes>
                        <Route
                            path="/sign-up"
                            element={
                                <SignUpPage
                                    {...commonProps}
                                    authenticatedUser={null}
                                    context={{
                                        allowSignup: true,
                                        sourcegraphDotComMode: true,
                                        authMinPasswordLength: 12,
                                        authProviders,
                                        xhrHeaders: {},
                                        externalURL: 'https://sourcegraph.test:3443',
                                    }}
                                    telemetryService={NOOP_TELEMETRY_SERVICE}
                                    telemetryRecorder={noOpTelemetryRecorder}
                                />
                            }
                        />
                    </Routes>
                </MockedTestProvider>,
                { route: '/sign-up' }
            ).asFragment()
        ).toMatchSnapshot()
    })

    it('renders redirect when user is authenticated', () => {
        // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
        const mockUser = {
            id: 'userID',
            username: 'username',
            emails: [{ email: 'user@me.com', isPrimary: true, verified: true }],
            siteAdmin: true,
        } as AuthenticatedUser

        expect(
            renderWithBrandedContext(
                <MockedTestProvider mocks={[]}>
                    <Routes>
                        <Route
                            path="/sign-up"
                            element={
                                <SignUpPage
                                    {...commonProps}
                                    authenticatedUser={mockUser}
                                    context={{
                                        allowSignup: true,
                                        sourcegraphDotComMode: false,
                                        authMinPasswordLength: 12,
                                        authProviders,
                                        xhrHeaders: {},
                                        externalURL: 'https://sourcegraph.test:3443',
                                    }}
                                    telemetryService={NOOP_TELEMETRY_SERVICE}
                                    telemetryRecorder={noOpTelemetryRecorder}
                                />
                            }
                        />
                    </Routes>
                </MockedTestProvider>,
                { route: '/sign-up' }
            ).asFragment()
        ).toMatchSnapshot()
    })
})
