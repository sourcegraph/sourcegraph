import { noop } from 'lodash'
import { describe, expect, test } from 'vitest'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import { SiteAdminGenerateProductLicenseForSubscriptionForm } from './SiteAdminGenerateProductLicenseForSubscriptionForm'
import { mockLicense } from './testUtils'

describe('SiteAdminGenerateProductLicenseForSubscriptionForm', () => {
    test('renders', () => {
        expect(
            renderWithBrandedContext(
                <MockedTestProvider mocks={[]}>
                    <SiteAdminGenerateProductLicenseForSubscriptionForm
                        subscriptionID="s"
                        latestLicense={mockLicense}
                        onCancel={noop}
                        subscriptionAccount="foo"
                        onGenerate={noop}
                        telemetryRecorder={noOpTelemetryRecorder}
                    />
                </MockedTestProvider>
            ).baseElement
        ).toMatchSnapshot()
    })
})
