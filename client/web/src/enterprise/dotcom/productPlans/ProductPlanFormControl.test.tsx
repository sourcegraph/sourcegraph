import React from 'react'
import * as GQL from '../../../../../shared/src/graphql/schema'
import renderer, { act } from 'react-test-renderer'
import { ProductPlanFormControl } from './ProductPlanFormControl'
import { of } from 'rxjs'

jest.mock('./ProductPlanPrice', () => ({
    ProductPlanPrice: 'ProductPlanPrice',
}))

jest.mock('./ProductPlanTiered', () => ({
    ProductPlanTiered: 'ProductPlanTiered',
}))

describe('ProductPlanFormControl', () => {
    test('new subscription', () => {
        const component = renderer.create(
            <ProductPlanFormControl
                value="p"
                onChange={() => undefined}
                _queryProductPlans={() =>
                    of<GQL.IProductPlan[]>([
                        {
                            __typename: 'ProductPlan',
                            billingPlanID: 'p0',
                            productPlanID: 'pp0',
                            minQuantity: null,
                            maxQuantity: 10,
                            name: 'n0',
                            nameWithBrand: 'nb0',
                            planTiers: [],
                            pricePerUserPerYear: 123,
                            tiersMode: 'volume',
                        },
                        {
                            __typename: 'ProductPlan',
                            billingPlanID: 'p1',
                            productPlanID: 'pp1',
                            minQuantity: null,
                            maxQuantity: null,
                            name: 'n1',
                            nameWithBrand: 'nb1',
                            planTiers: [],
                            pricePerUserPerYear: 456,
                            tiersMode: 'volume',
                        },
                    ])
                }
            />
        )
        act(() => undefined)
        expect(component.toJSON()).toMatchSnapshot()
    })
})
