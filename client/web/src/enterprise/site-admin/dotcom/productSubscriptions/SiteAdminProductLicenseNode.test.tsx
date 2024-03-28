import { describe, expect, test, vi } from 'vitest'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import { SiteAdminProductLicenseNode } from './SiteAdminProductLicenseNode'

vi.mock('../../../dotcom/productSubscriptions/AccountName', () => ({ AccountName: () => 'AccountName' }))

describe('SiteAdminProductLicenseNode', () => {
    test('active', () => {
        expect(
            renderWithBrandedContext(
                <MockedTestProvider>
                    <SiteAdminProductLicenseNode
                        node={{
                            createdAt: '2020-01-01',
                            id: 'l1',
                            licenseKey: 'lk1',
                            version: 1,
                            revokedAt: null,
                            revokeReason: null,
                            siteID: null,
                            info: {
                                __typename: 'ProductLicenseInfo',
                                expiresAt: '2021-01-01',
                                productNameWithBrand: 'NB',
                                tags: ['a'],
                                userCount: 123,
                                salesforceSubscriptionID: null,
                                salesforceOpportunityID: null,
                            },
                            subscription: {
                                uuid: 'uuid',
                                id: 'id1',
                                account: null,
                                name: 's',
                                activeLicense: { id: 'l1' },
                                urlForSiteAdmin: '/s',
                            },
                        }}
                        showSubscription={true}
                        onRevokeCompleted={function (): void {
                            throw new Error('Function not implemented.')
                        }}
                        telemetryRecorder={noOpTelemetryRecorder}
                    />
                </MockedTestProvider>
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('inactive', () => {
        expect(
            renderWithBrandedContext(
                <MockedTestProvider>
                    <SiteAdminProductLicenseNode
                        node={{
                            createdAt: '2020-01-01',
                            id: 'l1',
                            licenseKey: 'lk1',
                            version: 1,
                            revokedAt: null,
                            revokeReason: null,
                            siteID: null,
                            info: {
                                __typename: 'ProductLicenseInfo',
                                expiresAt: '2021-01-01',
                                productNameWithBrand: 'NB',
                                tags: ['a'],
                                userCount: 123,
                                salesforceSubscriptionID: null,
                                salesforceOpportunityID: null,
                            },
                            subscription: {
                                uuid: 'uuid',
                                id: 'id1',
                                account: null,
                                name: 's',
                                activeLicense: { id: 'l0' },
                                urlForSiteAdmin: '/s',
                            },
                        }}
                        showSubscription={true}
                        onRevokeCompleted={function (): void {
                            throw new Error('Function not implemented.')
                        }}
                        telemetryRecorder={noOpTelemetryRecorder}
                    />
                </MockedTestProvider>
            ).asFragment()
        ).toMatchSnapshot()
    })
})
