import { createMemoryHistory } from 'history'
import React from 'react'
import { Router } from 'react-router'
import renderer from 'react-test-renderer'

import { ProductSubscriptionForm } from './ProductSubscriptionForm'

jest.mock('../../dotcom/billing/StripeWrapper', () => ({
    StripeWrapper: ({
        component: Component,
        ...props
    }: {
        component: React.ComponentType<{ stripe: unknown }>
        [name: string]: unknown
    }) => <Component {...props} stripe={{}} />,
}))

jest.mock('@stripe/react-stripe-js', () => ({
    useStripe: () => null,
    useElements: () => null,
    CardElement: 'CardElement',
}))

jest.mock('../../dotcom/productPlans/ProductSubscriptionUserCountFormControl', () => ({
    ProductSubscriptionUserCountFormControl: 'ProductSubscriptionUserCountFormControl',
}))

jest.mock('../../dotcom/productPlans/ProductPlanFormControl', () => ({
    ProductPlanFormControl: 'ProductPlanFormControl',
}))

jest.mock('./NewProductSubscriptionPaymentSection', () => ({
    NewProductSubscriptionPaymentSection: 'NewProductSubscriptionPaymentSection',
}))

describe('ProductSubscriptionForm', () => {
    test('new subscription for anonymous viewer (no account)', () => {
        const history = createMemoryHistory()
        expect(
            renderer
                .create(
                    <Router history={history}>
                        <ProductSubscriptionForm
                            accountID={null}
                            subscriptionID={null}
                            onSubmit={() => undefined}
                            submissionState={undefined}
                            primaryButtonText="Submit"
                            isLightTheme={false}
                            history={history}
                        />
                    </Router>
                )
                .toJSON()
        ).toMatchSnapshot()
    })

    test('new subscription for existing account', () => {
        const history = createMemoryHistory()
        expect(
            renderer
                .create(
                    <Router history={history}>
                        <ProductSubscriptionForm
                            accountID="a"
                            subscriptionID={null}
                            onSubmit={() => undefined}
                            submissionState={undefined}
                            primaryButtonText="Submit"
                            isLightTheme={false}
                            history={history}
                        />
                    </Router>
                )
                .toJSON()
        ).toMatchSnapshot()
    })

    test('edit existing subscription', () => {
        const history = createMemoryHistory()
        expect(
            renderer
                .create(
                    <Router history={history}>
                        <ProductSubscriptionForm
                            accountID="a"
                            subscriptionID="s"
                            initialValue={{ userCount: 123, billingPlanID: 'p' }}
                            onSubmit={() => undefined}
                            submissionState={undefined}
                            primaryButtonText="Submit"
                            isLightTheme={false}
                            history={history}
                        />
                    </Router>
                )
                .toJSON()
        ).toMatchSnapshot()
    })
})
