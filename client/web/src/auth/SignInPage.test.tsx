import { within } from '@testing-library/dom'
import { Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import type { AuthenticatedUser } from '../auth'
import type { SourcegraphContext } from '../jscontext'

import { SignInPage } from './SignInPage'

describe('SignInPage', () => {
    const authProviders: SourcegraphContext['authProviders'] = [
        {
            displayName: 'Builtin username-password authentication',
            isBuiltin: true,
            serviceType: 'builtin',
            authenticationURL: '',
            serviceID: '',
            clientID: '1234',
        },
        {
            serviceType: 'github',
            displayName: 'GitHub',
            isBuiltin: false,
            authenticationURL: 'http://localhost/.auth/gitlab/login?pc=f00bar&returnTo=%2Fsearch',
            serviceID: 'https://github.com',
            clientID: '1234',
        },
        {
            serviceType: 'gitlab',
            displayName: 'GitLab',
            isBuiltin: false,
            authenticationURL: 'http://localhost/.auth/gitlab/login?pc=f00bar&returnTo=%2Fsearch',
            serviceID: 'https://gitlab.com',
            clientID: '1234',
        },
    ]

    const render = (
        route: string,
        props: {
            authProviders?: SourcegraphContext['authProviders']
            authenticatedUser?: AuthenticatedUser
            sourcegraphDotComMode?: SourcegraphContext['sourcegraphDotComMode']
            allowSignup?: SourcegraphContext['allowSignup']
            primaryLoginProvidersCount?: SourcegraphContext['primaryLoginProvidersCount']
        }
    ) =>
        renderWithBrandedContext(
            <Routes>
                <Route
                    path="/sign-in"
                    element={
                        <SignInPage
                            authenticatedUser={props.authenticatedUser ?? null}
                            context={{
                                allowSignup: props.allowSignup ?? true,
                                sourcegraphDotComMode: props.sourcegraphDotComMode ?? false,
                                authProviders: props.authProviders ?? authProviders,
                                resetPasswordEnabled: true,
                                xhrHeaders: {},
                                primaryLoginProvidersCount: props.primaryLoginProvidersCount ?? 5,
                            }}
                            telemetryRecorder={noOpTelemetryRecorder}
                        />
                    }
                />
            </Routes>,
            { route }
        )

    it('renders sign in page (server)', () => {
        const rendered = render('/sign-in', {})
        expect(
            within(rendered.baseElement)
                .queryByText(txt => txt.includes('Other login methods'))
                ?.closest('button')
        ).toBeInTheDocument()
        expect(
            within(rendered.baseElement)
                .queryByText(txt => txt.includes('Sign up'))
                ?.closest('a')
        ).toHaveAttribute('href', '/sign-up')

        expect(rendered.asFragment()).toMatchSnapshot()
    })

    it('renders sign in page (server) with email form expanded', () => {
        const rendered = render('/sign-in?showMore', {})
        expect(
            within(rendered.baseElement).queryByText(txt => txt.includes('Continue with Email'))
        ).not.toBeInTheDocument()
        expect(rendered.asFragment()).toMatchSnapshot()
    })

    it('renders sign in page (server) with request access link', () => {
        const rendered = render('/sign-in', { allowSignup: false })
        expect(
            within(rendered.baseElement)
                .queryByText(txt => txt.includes('Request access'))
                ?.closest('a')
        ).toHaveAttribute('href', '/request-access')

        expect(rendered.asFragment()).toMatchSnapshot()
    })

    it('renders sign in page (server) with only builtin authProvider', () => {
        const rendered = render('/sign-in', {
            authProviders: authProviders.filter(authProvider => authProvider.serviceType === 'builtin'),
        })
        expect(
            within(rendered.baseElement).queryByText(txt => txt.includes('Other login methods'))
        ).not.toBeInTheDocument()

        expect(rendered.asFragment()).toMatchSnapshot()
    })

    it('renders "Other login methods" if primary provider count is low enough', () => {
        const rendered = render('/sign-in', {
            authProviders: authProviders.filter(authProvider => authProvider.serviceType !== 'builtin'),
            primaryLoginProvidersCount: 1,
        })
        expect(within(rendered.baseElement).queryByText(txt => txt.includes('Other login methods'))).toBeInTheDocument()

        expect(rendered.asFragment()).toMatchSnapshot()
    })

    it('renders non-primary auth provider if primary provider count is low enough and showMore is in the query', () => {
        const rendered = render('/sign-in?showMore', {
            authProviders: authProviders.filter(authProvider => authProvider.serviceType !== 'builtin'),
            primaryLoginProvidersCount: 1,
        })
        expect(
            within(rendered.baseElement).queryByText(txt => txt.includes('Other login methods'))
        ).not.toBeInTheDocument()

        expect(
            within(rendered.baseElement).queryByText(txt => txt.includes('Continue with GitLab'))
        ).toBeInTheDocument()

        expect(rendered.asFragment()).toMatchSnapshot()
    })

    describe('with Sourcegraph operator auth provider', () => {
        const withSourcegraphOperator: SourcegraphContext['authProviders'] = [
            ...authProviders,
            {
                displayName: 'Sourcegraph Operators',
                isBuiltin: false,
                serviceType: 'sourcegraph-operator',
                authenticationURL: '',
                serviceID: '',
                clientID: '',
            },
        ]

        it('renders page with 2 providers', () => {
            const rendered = render('/sign-in', { authProviders: withSourcegraphOperator })
            expect(
                within(rendered.baseElement).queryByText(txt => txt.includes('Sourcegraph Operators'))
            ).not.toBeInTheDocument()
            expect(rendered.asFragment()).toMatchSnapshot()
        })

        it('renders page with 3 providers (url-param present)', () => {
            const rendered = render('/sign-in?sourcegraph-operator', { authProviders: withSourcegraphOperator })
            expect(
                within(rendered.baseElement).queryByText(txt => txt.includes('Sourcegraph Operators'))
            ).toBeInTheDocument()
            expect(rendered.asFragment()).toMatchSnapshot()
        })
    })

    describe('with Gerrit auth provider', () => {
        const withGerritProvider: SourcegraphContext['authProviders'] = [
            ...authProviders,
            {
                displayName: 'Gerrit',
                isBuiltin: false,
                serviceType: 'gerrit',
                authenticationURL: '',
                serviceID: '',
                clientID: '',
            },
        ]
        it('does not render the Gerrit provider', () => {
            const rendered = render('/sign-in', { authProviders: withGerritProvider })
            expect(within(rendered.baseElement).queryByText(txt => txt.includes('Gerrit'))).not.toBeInTheDocument()
            expect(rendered.asFragment()).toMatchSnapshot()
        })
    })

    it('renders sign in page (dotcom)', () => {
        expect(render('/sign-in', { sourcegraphDotComMode: true }).asFragment()).toMatchSnapshot()
    })

    it('renders redirect when user is authenticated', () => {
        // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
        const mockUser = {
            id: 'userID',
            username: 'username',
            emails: [{ email: 'user@me.com', isPrimary: true, verified: true }],
            siteAdmin: true,
        } as AuthenticatedUser

        expect(render('/sign-in', { authenticatedUser: mockUser }).asFragment()).toMatchSnapshot()
    })

    it('renders different prefix on provider buttons', () => {
        const providers = authProviders.map(provider => ({ ...provider, displayPrefix: 'Just login through' }))

        const rendered = render('/sign-in', { authProviders: providers })
        expect(
            within(rendered.baseElement)
                .queryByText(txt => txt.includes('Just login through GitHub'))
                ?.closest('a')
        ).toBeInTheDocument()
        expect(
            within(rendered.baseElement)
                .queryByText(txt => txt.includes('Just login through GitLab'))
                ?.closest('a')
        ).toBeInTheDocument()

        expect(rendered.asFragment()).toMatchSnapshot()
    })
})
