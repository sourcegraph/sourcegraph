import React, { useEffect } from 'react'

import { parseISO } from 'date-fns'
import { useParams } from 'react-router-dom'
import { validate as validateUUID } from 'uuid'

import { useQuery } from '@sourcegraph/http-client'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { LoadingSpinner, H4, Text, Link, ErrorAlert, PageHeader, Container } from '@sourcegraph/wildcard'

import { PageTitle } from '../../../components/PageTitle'
import type {
    UserAreaUserFields,
    UserProductSubscriptionResult,
    UserProductSubscriptionVariables,
} from '../../../graphql-operations'
import { SiteAdminAlert } from '../../../site-admin/SiteAdminAlert'
import { CodyServicesSection } from '../../site-admin/dotcom/productSubscriptions/CodyServicesSection'
import { accessTokenPath, errorForPath } from '../../site-admin/dotcom/productSubscriptions/utils'

import { USER_PRODUCT_SUBSCRIPTION } from './backend'
import { UserProductSubscriptionStatus } from './UserProductSubscriptionStatus'

interface Props extends TelemetryV2Props {
    user: Pick<UserAreaUserFields, 'settingsURL'>
}

/**
 * Displays a product subscription in the user subscriptions area.
 */
export const UserSubscriptionsProductSubscriptionPage: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    user,
    telemetryRecorder,
}) => {
    const { subscriptionUUID = '' } = useParams<{ subscriptionUUID: string }>()

    useEffect(() => telemetryRecorder.recordEvent('settings.userSubscription', 'view'), [telemetryRecorder])

    const isValidUUID = validateUUID(subscriptionUUID)
    const validationError = !isValidUUID && new Error('Subscription ID is not a valid UUID')

    const { data, loading, error, refetch } = useQuery<UserProductSubscriptionResult, UserProductSubscriptionVariables>(
        USER_PRODUCT_SUBSCRIPTION,
        {
            variables: { uuid: subscriptionUUID },
            errorPolicy: 'all',
        }
    )

    if (loading) {
        return <LoadingSpinner />
    }

    if (!isValidUUID) {
        return <ErrorAlert className="my-2" error={validationError} />
    }

    // If there's an error, and the entire request failed loading, simply render an error page.
    // Otherwise, we want to get more specific with error handling.
    if (
        error &&
        (error.networkError ||
            error.clientErrors.length > 0 ||
            !(error.graphQLErrors.length === 1 && errorForPath(error, accessTokenPath)))
    ) {
        return <ErrorAlert className="my-2" error={error} />
    }

    const productSubscription = data!.dotcom.productSubscription

    return (
        <div className="user-subscriptions-product-subscription-page">
            <PageTitle title="Subscription" />
            <PageHeader
                headingElement="h2"
                path={[
                    { text: 'Subscriptions', to: `${user.settingsURL}/subscriptions` },
                    { text: productSubscription.name },
                ]}
                className="mb-3"
            />

            {productSubscription.urlForSiteAdmin && (
                <SiteAdminAlert className="mb-3">
                    To manage this subscription for the customer, go to{' '}
                    <Link to={productSubscription.urlForSiteAdmin}>view subscription</Link>.
                </SiteAdminAlert>
            )}

            {productSubscription.activeLicense?.info && (
                <UserProductSubscriptionStatus
                    subscriptionName={productSubscription.name}
                    productNameWithBrand={productSubscription.activeLicense?.info.productNameWithBrand}
                    userCount={productSubscription.activeLicense?.info.userCount}
                    expiresAt={parseISO(productSubscription.activeLicense.info.expiresAt)}
                    licenseKey={productSubscription.activeLicense?.licenseKey ?? null}
                    className="mb-3"
                    telemetryRecorder={telemetryRecorder}
                />
            )}

            {productSubscription.activeLicense === null && (
                <Container className="text-center mb-3">
                    <H4 className="text-muted">License expired</H4>
                    <Text className="text-muted mb-0">This subscription has no active subscription attached.</Text>
                </Container>
            )}

            <CodyServicesSection
                viewerCanAdminister={false}
                currentSourcegraphAccessToken={productSubscription.currentSourcegraphAccessToken}
                accessTokenError={errorForPath(error, accessTokenPath)}
                codyGatewayAccess={productSubscription.codyGatewayAccess}
                productSubscriptionID={productSubscription.id}
                productSubscriptionUUID={subscriptionUUID}
                refetchSubscription={refetch}
                telemetryRecorder={telemetryRecorder}
            />
        </div>
    )
}
