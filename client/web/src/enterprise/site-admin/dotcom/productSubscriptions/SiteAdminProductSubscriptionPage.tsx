import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react'

import { mdiPlus } from '@mdi/js'
import { useLocation, useNavigate, useParams } from 'react-router-dom'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { logger } from '@sourcegraph/common'
import { useMutation, useQuery } from '@sourcegraph/http-client'
import { Button, LoadingSpinner, Link, Icon, ErrorAlert, PageHeader, Container, H3, Text } from '@sourcegraph/wildcard'

import {
    ConnectionContainer,
    ConnectionError,
    ConnectionList,
    ConnectionLoading,
    ConnectionSummary,
    ShowMoreButton,
    SummaryContainer,
} from '../../../../components/FilteredConnection/ui'
import { PageTitle } from '../../../../components/PageTitle'
import { useScrollToLocationHash } from '../../../../components/useScrollToLocationHash'
import type {
    DotComProductSubscriptionResult,
    DotComProductSubscriptionVariables,
    ArchiveProductSubscriptionResult,
    ArchiveProductSubscriptionVariables,
} from '../../../../graphql-operations'
import { eventLogger } from '../../../../tracking/eventLogger'
import { AccountName } from '../../../dotcom/productSubscriptions/AccountName'
import { ProductSubscriptionLabel } from '../../../dotcom/productSubscriptions/ProductSubscriptionLabel'
import { LicenseGenerationKeyWarning } from '../../../productSubscription/LicenseGenerationKeyWarning'

import {
    ARCHIVE_PRODUCT_SUBSCRIPTION,
    DOTCOM_PRODUCT_SUBSCRIPTION,
    useProductSubscriptionLicensesConnection,
} from './backend'
import { CodyServicesSection } from './CodyServicesSection'
import { SiteAdminGenerateProductLicenseForSubscriptionForm } from './SiteAdminGenerateProductLicenseForSubscriptionForm'
import { SiteAdminProductLicenseNode } from './SiteAdminProductLicenseNode'
import { accessTokenPath, errorForPath } from './utils'

interface Props {}

/**
 * Displays a product subscription in the site admin area.
 */
export const SiteAdminProductSubscriptionPage: React.FunctionComponent<React.PropsWithChildren<Props>> = () => {
    const navigate = useNavigate()
    const { subscriptionUUID = '' } = useParams<{ subscriptionUUID: string }>()
    useEffect(() => eventLogger.logViewEvent('SiteAdminProductSubscription'), [])

    const [showGenerate, setShowGenerate] = useState<boolean>(false)

    const { data, loading, error, refetch } = useQuery<
        DotComProductSubscriptionResult,
        DotComProductSubscriptionVariables
    >(DOTCOM_PRODUCT_SUBSCRIPTION, {
        variables: { uuid: subscriptionUUID },
        errorPolicy: 'all',
    })

    const [archiveProductSubscription, { loading: archiveLoading, error: archiveError }] = useMutation<
        ArchiveProductSubscriptionResult,
        ArchiveProductSubscriptionVariables
    >(ARCHIVE_PRODUCT_SUBSCRIPTION)

    const onArchive = useCallback(async () => {
        if (!data) {
            return
        }
        if (
            !window.confirm(
                'Do you really want to archive this product subscription? This will hide it from site admins and users.\n\nHowever, it does NOT:\n\n- invalidate the license key\n- refund payment or cancel billing\n\nYou must manually do those things.'
            )
        ) {
            return
        }
        try {
            await archiveProductSubscription({ variables: { id: data.dotcom.productSubscription.id } })
            navigate('/site-admin/dotcom/product/subscriptions')
        } catch (error) {
            logger.error(error)
        }
    }, [data, archiveProductSubscription, navigate])

    const toggleShowGenerate = useCallback((): void => setShowGenerate(previousValue => !previousValue), [])

    const refetchRef = useRef<(() => void) | null>(null)
    const setRefetchRef = useCallback(
        (refetch: (() => void) | null) => {
            refetchRef.current = refetch
        },
        [refetchRef]
    )

    const onLicenseUpdate = useCallback(async () => {
        await refetch()
        if (refetchRef.current) {
            refetchRef.current()
        }
        setShowGenerate(false)
    }, [refetch, refetchRef])

    if (loading && !data) {
        return <LoadingSpinner />
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
        <>
            <div className="site-admin-product-subscription-page">
                <PageTitle title="Enterprise subscription" />
                <PageHeader
                    headingElement="h2"
                    path={[
                        { text: 'Enterprise subscriptions', to: '/site-admin/dotcom/product/subscriptions' },
                        { text: productSubscription.name },
                    ]}
                    description={
                        <span className="text-muted">
                            Created <Timestamp date={productSubscription.createdAt} />
                        </span>
                    }
                    actions={
                        <Button onClick={onArchive} disabled={archiveLoading} variant="danger">
                            Archive
                        </Button>
                    }
                    className="mb-3"
                />
                {archiveError && <ErrorAlert className="mt-2" error={archiveError} />}

                <H3>Details</H3>
                <Container className="mb-3">
                    <table className="table mb-0">
                        <tbody>
                            <tr>
                                <th className="text-nowrap">ID</th>
                                <td className="w-100">{productSubscription.name}</td>
                            </tr>
                            <tr>
                                <th className="text-nowrap">Current Plan</th>
                                <td className="w-100">
                                    <ProductSubscriptionLabel productSubscription={productSubscription} />
                                </td>
                            </tr>
                            <tr>
                                <th className="text-nowrap">Account</th>
                                <td className="w-100">
                                    <AccountName account={productSubscription.account} /> &mdash;{' '}
                                    <Link to={productSubscription.url}>View as user</Link>
                                </td>
                            </tr>
                            <tr>
                                <th className="text-nowrap">Salesforce Opportunity</th>
                                <td className="w-100">
                                    {(!productSubscription.activeLicense ||
                                        productSubscription.activeLicense.info?.salesforceOpportunityID === null) && (
                                        <span className="text-muted">None</span>
                                    )}
                                    {productSubscription.activeLicense &&
                                        productSubscription.activeLicense.info?.salesforceOpportunityID !== null && (
                                            <>{productSubscription.activeLicense.info?.salesforceOpportunityID}</>
                                        )}
                                </td>
                            </tr>
                            <tr>
                                <th className="text-nowrap">Salesforce Subscription</th>
                                <td className="w-100">
                                    {(!productSubscription.activeLicense ||
                                        productSubscription.activeLicense.info?.salesforceSubscriptionID === null) && (
                                        <span className="text-muted">None</span>
                                    )}
                                    {productSubscription.activeLicense &&
                                        productSubscription.activeLicense.info?.salesforceSubscriptionID !== null && (
                                            <>{productSubscription.activeLicense.info?.salesforceSubscriptionID}</>
                                        )}
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </Container>

                <CodyServicesSection
                    viewerCanAdminister={true}
                    currentSourcegraphAccessToken={productSubscription.currentSourcegraphAccessToken}
                    accessTokenError={errorForPath(error, accessTokenPath)}
                    codyGatewayAccess={productSubscription.codyGatewayAccess}
                    productSubscriptionID={productSubscription.id}
                    productSubscriptionUUID={subscriptionUUID}
                    refetchSubscription={refetch}
                />

                <H3 className="d-flex align-items-start">
                    Licenses
                    <Button className="ml-auto" onClick={toggleShowGenerate} variant="primary">
                        <Icon aria-hidden={true} svgPath={mdiPlus} /> New license key
                    </Button>
                </H3>
                <LicenseGenerationKeyWarning className="mb-2" />
                <Container className="mb-2">
                    <ProductSubscriptionLicensesConnection
                        subscriptionUUID={subscriptionUUID}
                        toggleShowGenerate={toggleShowGenerate}
                        setRefetch={setRefetchRef}
                    />
                </Container>
            </div>

            {showGenerate && (
                <SiteAdminGenerateProductLicenseForSubscriptionForm
                    subscriptionID={productSubscription.id}
                    subscriptionAccount={productSubscription.account?.username || ''}
                    latestLicense={productSubscription.productLicenses?.nodes[0] ?? undefined}
                    onGenerate={onLicenseUpdate}
                    onCancel={() => setShowGenerate(false)}
                />
            )}
        </>
    )
}

const ProductSubscriptionLicensesConnection: React.FunctionComponent<{
    subscriptionUUID: string
    toggleShowGenerate: () => void
    setRefetch: (refetch: () => void) => void
}> = ({ subscriptionUUID, setRefetch, toggleShowGenerate }) => {
    const { loading, hasNextPage, fetchMore, refetchAll, connection, error } = useProductSubscriptionLicensesConnection(
        subscriptionUUID,
        20
    )

    useEffect(() => {
        setRefetch(refetchAll)
    }, [setRefetch, refetchAll])

    const location = useLocation()
    const licenseIDFromLocationHash = useMemo(() => {
        if (location.hash.length > 1) {
            return decodeURIComponent(location.hash.slice(1))
        }
        return
    }, [location.hash])
    useScrollToLocationHash(location)

    return (
        <ConnectionContainer>
            {error && <ConnectionError errors={[error.message]} />}
            {loading && !connection && <ConnectionLoading />}
            <ConnectionList as="ul" className="list-group list-group-flush mb-0" aria-label="Subscription licenses">
                {connection?.nodes?.map(node => (
                    <SiteAdminProductLicenseNode
                        key={node.id}
                        node={node}
                        defaultExpanded={node.id === licenseIDFromLocationHash}
                        showSubscription={false}
                        onRevokeCompleted={refetchAll}
                    />
                ))}
            </ConnectionList>
            {connection && (
                <SummaryContainer centered={true}>
                    <ConnectionSummary
                        first={15}
                        centered={true}
                        connection={connection}
                        noun="product license"
                        pluralNoun="product licenses"
                        hasNextPage={hasNextPage}
                        emptyElement={<NoProductLicense toggleShowGenerate={toggleShowGenerate} />}
                    />
                    {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                </SummaryContainer>
            )}
        </ConnectionContainer>
    )
}

const NoProductLicense: React.FunctionComponent<{
    toggleShowGenerate: () => void
}> = ({ toggleShowGenerate }) => (
    <>
        <Text className="text-muted">No license key has been generated yet.</Text>
        <Button onClick={toggleShowGenerate} variant="primary">
            <Icon aria-hidden={true} svgPath={mdiPlus} /> New license key
        </Button>
    </>
)
