import React from 'react'
import renderer from 'react-test-renderer'
import { SiteAdminGenerateProductLicenseForSubscriptionForm } from './SiteAdminGenerateProductLicenseForSubscriptionForm'

describe('SiteAdminGenerateProductLicenseForSubscriptionForm', () => {
    test('renders', () => {
        expect(
            renderer
                .create(
                    <SiteAdminGenerateProductLicenseForSubscriptionForm
                        subscriptionID="s"
                        onGenerate={() => undefined}
                    />
                )
                .toJSON()
        ).toMatchSnapshot()
    })
})
