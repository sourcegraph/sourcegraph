import { renderHook } from '@testing-library/react'
import { vi, describe, afterEach, test, expect, beforeEach } from 'vitest'

import { CodyProRoutes } from './codyProRoutes'
import * as subscriptionQueries from './management/api/react-query/subscriptions'
import type { SubscriptionSummary } from './management/api/teamSubscriptions'
import { useCodyProNavLinks } from './useCodyProNavLinks'

describe('useCodyProNavLinks', () => {
    const useSubscriptionSummaryMock = vi.spyOn(subscriptionQueries, 'useSubscriptionSummary')

    afterEach(() => {
        useSubscriptionSummaryMock.mockReset()
    })

    const mockSubscriptionSummary = (summary?: SubscriptionSummary): void => {
        useSubscriptionSummaryMock.mockReturnValue({ data: summary } as ReturnType<
            typeof subscriptionQueries.useSubscriptionSummary
        >)
    }

    test('returns empty array if subscription summary is undefined', () => {
        mockSubscriptionSummary()
        const { result } = renderHook(() => useCodyProNavLinks())
        expect(result.current).toHaveLength(0)
    })

    test('returns empty array user is not admin', () => {
        const summary: SubscriptionSummary = {
            teamId: '018ff1b3-118c-7789-82e4-ab9106eed204',
            userRole: 'member',
            teamCurrentMembers: 2,
            teamMaxMembers: 6,
            subscriptionStatus: 'active',
            cancelAtPeriodEnd: false,
        }
        mockSubscriptionSummary(summary)
        const { result } = renderHook(() => useCodyProNavLinks())
        expect(result.current).toHaveLength(0)
    })

    describe('user is admin', () => {
        const summary: SubscriptionSummary = {
            teamId: '018ff1b3-118c-7789-82e4-ab9106eed204',
            userRole: 'admin',
            teamCurrentMembers: 2,
            teamMaxMembers: 6,
            subscriptionStatus: 'active',
            cancelAtPeriodEnd: false,
        }

        const setUseEmbeddedUI = (useEmbeddedUI: boolean) => {
            vi.stubGlobal('context', {
                frontendCodyProConfig: {
                    stripePublishableKey: 'pk_test_123',
                    sscBaseUrl: '',
                    useEmbeddedUI,
                },
            })
        }

        beforeEach(() => {
            vi.stubGlobal('context', {})
            mockSubscriptionSummary(summary)
        })

        test.skip('returns links to subscription and team management pages if embedded UI is enabled', () => {
            setUseEmbeddedUI(true)
            const { result } = renderHook(() => useCodyProNavLinks())
            expect(result.current).toHaveLength(2)
            expect(result.current[0].label).toBe('Manage subscription')
            expect(result.current[0].to).toBe(CodyProRoutes.SubscriptionManage)
            expect(result.current[1].label).toBe('Manage team')
            expect(result.current[1].to).toBe(CodyProRoutes.ManageTeam)
        })

        test('returns link to subscription management page if embedded UI is disabled', () => {
            setUseEmbeddedUI(false)
            const { result } = renderHook(() => useCodyProNavLinks())
            expect(result.current).toHaveLength(1)
            expect(result.current[0].label).toBe('Manage subscription')
            expect(result.current[0].to).toBe('https://accounts.sourcegraph.com/cody/subscription')
        })
    })
})
