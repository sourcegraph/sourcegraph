import { describe, expect, test } from 'vitest'

import { getDocumentNode } from '@sourcegraph/http-client'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { MockedTestProvider, waitForNextApolloResponse } from '@sourcegraph/shared/src/testing/apollo'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import { USER_PRODUCT_SUBSCRIPTION } from './backend'
import { UserSubscriptionsProductSubscriptionPage } from './UserSubscriptionsProductSubscriptionPage'

const uuid = '43002662-f627-4550-9af6-d621d2a878de'

describe('UserSubscriptionsProductSubscriptionPage', () => {
    test('renders', async () => {
        const component = renderWithBrandedContext(
            <MockedTestProvider
                mocks={[
                    {
                        request: { query: getDocumentNode(USER_PRODUCT_SUBSCRIPTION), variables: { uuid } },
                        result: {
                            data: {
                                dotcom: {
                                    productSubscription: {
                                        __typename: 'ProductSubscription',
                                    },
                                },
                            },
                        },
                    },
                ]}
            >
                <UserSubscriptionsProductSubscriptionPage
                    user={{ settingsURL: '/u' }}
                    telemetryRecorder={noOpTelemetryRecorder}
                />
            </MockedTestProvider>,
            { path: '/:subscriptionUUID', route: `/${uuid}` }
        )
        await waitForNextApolloResponse()
        expect(component.asFragment()).toMatchSnapshot()
    })
})
