import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { parseISO } from 'date-fns'
import React, { useEffect, useMemo } from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Observable } from 'rxjs'
import { catchError, map, startWith } from 'rxjs/operators'
import { gql } from '../../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../../shared/src/graphql/schema'
import { asError, createAggregateError, isErrorLike } from '../../../../../shared/src/util/errors'
import { queryGraphQL } from '../../../backend/graphql'
import { PageTitle } from '../../../components/PageTitle'
import { SiteAdminAlert } from '../../../site-admin/SiteAdminAlert'
import { eventLogger } from '../../../tracking/eventLogger'
import { mailtoSales } from '../../productSubscription/helpers'
import { BackToAllSubscriptionsLink } from './BackToAllSubscriptionsLink'
import { ProductSubscriptionBilling } from './ProductSubscriptionBilling'
import { ProductSubscriptionHistory } from './ProductSubscriptionHistory'
import { UserProductSubscriptionStatus } from './UserProductSubscriptionStatus'
import { ErrorAlert } from '../../../components/alerts'
import { useObservable } from '../../../../../shared/src/util/useObservable'
import * as H from 'history'

interface Props extends Pick<RouteComponentProps<{ subscriptionUUID: string }>, 'match'> {
    user: Pick<GQL.IUser, 'settingsURL'>

    /** For mocking in tests only. */
    _queryProductSubscription?: typeof queryProductSubscription
    history: H.History
}

const LOADING = 'loading' as const

/**
 * Displays a product subscription in the user subscriptions area.
 */
export const UserSubscriptionsProductSubscriptionPage: React.FunctionComponent<Props> = ({
    user,
    match: {
        params: { subscriptionUUID },
    },
    _queryProductSubscription = queryProductSubscription,
    history,
}) => {
    useEffect(() => eventLogger.logViewEvent('UserSubscriptionsProductSubscription'), [])

    /**
     * The product subscription, or loading, or an error.
     */
    const productSubscription =
        useObservable(
            useMemo(
                () =>
                    _queryProductSubscription(subscriptionUUID).pipe(
                        catchError(error => [asError(error)]),
                        startWith(LOADING)
                    ),
                [_queryProductSubscription, subscriptionUUID]
            )
        ) || LOADING

    return (
        <div className="user-subscriptions-product-subscription-page">
            <PageTitle title="Subscription" />
            <div className="d-flex align-items-center justify-content-between">
                <BackToAllSubscriptionsLink user={user} />
                {productSubscription !== LOADING &&
                    !isErrorLike(productSubscription) &&
                    productSubscription.urlForSiteAdmin && (
                        <SiteAdminAlert className="small m-0 p-1">
                            <Link to={productSubscription.urlForSiteAdmin} className="mt-2 d-block">
                                View subscription
                            </Link>
                        </SiteAdminAlert>
                    )}
            </div>
            {productSubscription === LOADING ? (
                <LoadingSpinner className="icon-inline" />
            ) : isErrorLike(productSubscription) ? (
                <ErrorAlert className="my-2" error={productSubscription} />
            ) : (
                <>
                    <h2>Subscription {productSubscription.name}</h2>
                    {(productSubscription.invoiceItem || productSubscription.activeLicense?.info) && (
                        <UserProductSubscriptionStatus
                            subscriptionName={productSubscription.name}
                            productNameWithBrand={
                                productSubscription.activeLicense?.info
                                    ? productSubscription.activeLicense.info.productNameWithBrand
                                    : productSubscription.invoiceItem!.plan.nameWithBrand
                            }
                            userCount={
                                productSubscription.activeLicense?.info
                                    ? productSubscription.activeLicense.info.userCount
                                    : productSubscription.invoiceItem!.userCount
                            }
                            expiresAt={
                                productSubscription.activeLicense?.info
                                    ? parseISO(productSubscription.activeLicense.info.expiresAt)
                                    : parseISO(productSubscription.invoiceItem!.expiresAt)
                            }
                            licenseKey={productSubscription.activeLicense?.licenseKey ?? null}
                        />
                    )}
                    <div className="card mt-3">
                        <div className="card-header">Billing</div>
                        {productSubscription.invoiceItem ? (
                            <>
                                <ProductSubscriptionBilling productSubscription={productSubscription} />
                                <div className="card-footer">
                                    <a
                                        href={mailtoSales({
                                            subject: `Change payment method for subscription ${productSubscription.name}`,
                                        })}
                                    >
                                        Contact sales
                                    </a>{' '}
                                    to change your payment method.
                                </div>
                            </>
                        ) : (
                            <div className="card-body">
                                <span className="text-muted ">
                                    No billing information is associated with this subscription.{' '}
                                    <a
                                        href={mailtoSales({
                                            subject: `Billing for subscription ${productSubscription.name}`,
                                        })}
                                    >
                                        Contact sales
                                    </a>{' '}
                                    for help.
                                </span>
                            </div>
                        )}
                    </div>
                    <div className="card mt-3">
                        <div className="card-header">History</div>
                        <ProductSubscriptionHistory productSubscription={productSubscription} />
                    </div>
                </>
            )}
        </div>
    )
}

function queryProductSubscription(uuid: string): Observable<GQL.IProductSubscription> {
    return queryGraphQL(
        gql`
            query ProductSubscription($uuid: String!) {
                dotcom {
                    productSubscription(uuid: $uuid) {
                        ...ProductSubscriptionFieldsOnSubscriptionPage
                    }
                }
            }

            fragment ProductSubscriptionFieldsOnSubscriptionPage on ProductSubscription {
                id
                name
                account {
                    id
                    username
                    displayName
                    emails {
                        email
                        verified
                    }
                }
                invoiceItem {
                    plan {
                        billingPlanID
                        name
                        nameWithBrand
                        pricePerUserPerYear
                    }
                    userCount
                    expiresAt
                }
                events {
                    id
                    date
                    title
                    description
                    url
                }
                activeLicense {
                    licenseKey
                    info {
                        productNameWithBrand
                        tags
                        userCount
                        expiresAt
                    }
                }
                createdAt
                isArchived
                url
                urlForSiteAdmin
            }
        `,
        { uuid }
    ).pipe(
        map(({ data, errors }) => {
            if (!data || !data.dotcom || !data.dotcom.productSubscription || (errors && errors.length > 0)) {
                throw createAggregateError(errors)
            }
            return data.dotcom.productSubscription
        })
    )
}
