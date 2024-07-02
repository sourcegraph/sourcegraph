import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import sinon from 'sinon'
import { afterAll, beforeAll, afterEach, describe, expect, test, vi, beforeEach } from 'vitest'

import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import { AnchorLink, RouterLink, setLinkComponent } from '@sourcegraph/wildcard'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import * as codyProHooks from '../cody/useCodyProNavLinks'

import { UserNavItem, type UserNavItemProps } from './UserNavItem'

describe('UserNavItem', () => {
    const origCodyEnabledForCurrentUser = window.context?.codyEnabledForCurrentUser ?? true
    const reset = () => {
        if (!window.context) {
            window.context = {} as any
        }
        window.context.codyEnabledForCurrentUser = origCodyEnabledForCurrentUser
    }
    beforeEach(reset)
    afterEach(reset)

    beforeAll(() => {
        setLinkComponent(RouterLink)
    })

    afterAll(() => {
        setLinkComponent(AnchorLink)
    })

    const useCodyProNavLinksMock = vi.spyOn(codyProHooks, 'useCodyProNavLinks')
    beforeEach(() => {
        useCodyProNavLinksMock.mockReturnValue([])
    })
    afterEach(() => {
        useCodyProNavLinksMock.mockReset()
    })

    const USER: UserNavItemProps['authenticatedUser'] = {
        username: 'alice',
        displayName: 'alice doe',
        avatarURL: null,
        session: { canSignOut: true },
        settingsURL: '#',
        siteAdmin: true,
        organizations: {
            nodes: [
                {
                    __typename: 'Org',
                    id: '0',
                    name: 'acme',
                    displayName: 'Acme Corp',
                    url: '/organizations/acme',
                    settingsURL: '/organizations/acme/settings',
                },
                {
                    __typename: 'Org',
                    id: '1',
                    name: 'beta',
                    displayName: 'Beta Inc',
                    url: '/organizations/beta',
                    settingsURL: '/organizations/beta/settings',
                },
            ],
        },
        emails: [],
    }

    test('simple', () => {
        expect(
            render(
                <MemoryRouter>
                    <MockedTestProvider>
                        <UserNavItem
                            showKeyboardShortcutsHelp={() => undefined}
                            authenticatedUser={USER}
                            isSourcegraphDotCom={true}
                            showFeedbackModal={() => undefined}
                            telemetryService={NOOP_TELEMETRY_SERVICE}
                        />
                    </MockedTestProvider>
                </MemoryRouter>
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('logout click triggers page refresh instead of performing client-side only navigation', async () => {
        const result = renderWithBrandedContext(
            <MockedTestProvider>
                <UserNavItem
                    showKeyboardShortcutsHelp={() => undefined}
                    authenticatedUser={USER}
                    isSourcegraphDotCom={true}
                    showFeedbackModal={() => undefined}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                />
            </MockedTestProvider>
        )

        // Prevent console.error cause by "Not implemented: navigation (except hash changes)"
        // https://github.com/jsdom/jsdom/issues/2112
        sinon.stub(console, 'error')
        userEvent.click(screen.getByRole('button'))
        userEvent.click(await screen.findByText('Sign out'))

        expect(result.locationRef.entries.length).toBe(1)
        expect(result.locationRef.entries.find(({ pathname }) => pathname.includes('sign-out'))).toBe(undefined)
    })

    describe('Cody Pro section', () => {
        const setup = (isSourcegraphDotCom: boolean) => {
            renderWithBrandedContext(
                <MockedTestProvider>
                    <UserNavItem
                        showKeyboardShortcutsHelp={() => undefined}
                        authenticatedUser={USER}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        showFeedbackModal={() => undefined}
                        telemetryService={NOOP_TELEMETRY_SERVICE}
                    />
                </MockedTestProvider>
            )
            userEvent.click(screen.getByRole('button'))
        }

        describe('dotcom', () => {
            test('renders provided links', () => {
                const links = [
                    { to: '/foo', label: 'Foo' },
                    { to: '/bar', label: 'Bar' },
                ]
                useCodyProNavLinksMock.mockReturnValue(links)
                setup(true)

                for (const link of links) {
                    const el = screen.getByText(link.label)
                    expect(el).toHaveAttribute('href', link.to)
                }
            })

            test('is not rendered if no links provided', () => {
                useCodyProNavLinksMock.mockReturnValue([])
                setup(true)

                expect(useCodyProNavLinksMock).toHaveBeenCalled()
                expect(screen.queryByText('Cody Pro')).not.toBeInTheDocument()
            })
        })

        describe('enterprise', () => {
            test('is not rendered', () => {
                setup(false)

                // Cody Pro section is not rendered thus useCodyProNavLinks hook is not called
                expect(useCodyProNavLinksMock).not.toHaveBeenCalled()
            })
        })
    })
})
