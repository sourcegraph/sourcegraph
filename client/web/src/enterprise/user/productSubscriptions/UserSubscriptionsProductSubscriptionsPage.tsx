import React, { useCallback, useEffect } from 'react'

import type { Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { createAggregateError } from '@sourcegraph/common'
import { gql } from '@sourcegraph/http-client'
import { Container, Link, PageHeader, Text } from '@sourcegraph/wildcard'

import { queryGraphQL } from '../../../backend/graphql'
import { FilteredConnection } from '../../../components/FilteredConnection'
import { PageTitle } from '../../../components/PageTitle'
import type {
    ProductSubscriptionFields,
    ProductSubscriptionsResult,
    ProductSubscriptionsVariables,
    UserAreaUserFields,
} from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'
import {
    ProductSubscriptionNode,
    ProductSubscriptionNodeHeader,
    productSubscriptionFragment,
    type ProductSubscriptionNodeProps,
} from '../../dotcom/productSubscriptions/ProductSubscriptionNode'

interface Props {
    user: UserAreaUserFields
}

/**
 * Displays the enterprise subscriptions (formerly known as "product subscriptions") associated with this
 * account.
 */
export const UserSubscriptionsProductSubscriptionsPage: React.FunctionComponent<
    React.PropsWithChildren<Props>
> = props => {
    useEffect(() => {
        eventLogger.logViewEvent('UserSubscriptionsProductSubscriptions')
    }, [])

    const queryLicenses = useCallback(
        (args: { first?: number }): Observable<ProductSubscriptionsResult['dotcom']['productSubscriptions']> => {
            const variables: ProductSubscriptionsVariables = {
                first: args.first ?? null,
                account: props.user.id,
            }
            return queryGraphQL<ProductSubscriptionsResult>(
                gql`
                    query ProductSubscriptions($first: Int, $account: ID) {
                        dotcom {
                            productSubscriptions(first: $first, account: $account) {
                                nodes {
                                    ...ProductSubscriptionFields
                                }
                                totalCount
                                pageInfo {
                                    hasNextPage
                                }
                            }
                        }
                    }
                    ${productSubscriptionFragment}
                `,
                variables
            ).pipe(
                map(({ data, errors }) => {
                    if (!data?.dotcom?.productSubscriptions || (errors && errors.length > 0)) {
                        throw createAggregateError(errors)
                    }
                    return data.dotcom.productSubscriptions
                })
            )
        },
        [props.user.id]
    )

    return (
        <div className="user-subscriptions-product-subscriptions-page">
            <PageTitle title="Enterprise subscriptions" />
            <PageHeader
                headingElement="h2"
                path={[{ text: 'Enterprise subscriptions' }]}
                description={
                    <>
                        Search your private code with{' '}
                        <Link
                            to="https://sourcegraph.com"
                            onClick={() => eventLogger.log('ClickedOnEnterpriseCTA', { location: 'Subscriptions' })}
                        >
                            Sourcegraph Enterprise
                        </Link>
                        . See <Link to="https://sourcegraph.com/pricing">pricing</Link> for more information.
                    </>
                }
                className="mb-3"
            />
            <Container className="mb-3">
                <FilteredConnection<ProductSubscriptionFields, ProductSubscriptionNodeProps>
                    listComponent="table"
                    listClassName="table mb-0"
                    noun="Enterprise subscription"
                    pluralNoun="Enterprise subscriptions"
                    queryConnection={queryLicenses}
                    headComponent={ProductSubscriptionNodeHeader}
                    nodeComponent={ProductSubscriptionNode}
                    hideSearch={true}
                    noSummaryIfAllNodesVisible={true}
                    emptyElement={
                        <Text alignment="center" className="w-100 mb-0 text-muted">
                            You have no Enterprise subscriptions.
                        </Text>
                    }
                />
            </Container>
        </div>
    )
}
