import React, { useCallback, useEffect, useMemo, useState } from 'react'

import type { ConnectError } from '@connectrpc/connect'
import { mdiPlus } from '@mdi/js'
import { QueryClientProvider, type UseQueryResult } from '@tanstack/react-query'
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { logger } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Button,
    Container,
    ErrorAlert,
    H3,
    Icon,
    LoadingSpinner,
    PageHeader,
    Select,
    Text,
} from '@sourcegraph/wildcard'

import {
    ConnectionContainer,
    ConnectionError,
    ConnectionList,
    ConnectionLoading,
} from '../../../../components/FilteredConnection/ui'
import { PageTitle } from '../../../../components/PageTitle'
import { useScrollToLocationHash } from '../../../../components/useScrollToLocationHash'
import { ProductSubscriptionLabel } from '../../../dotcom/productSubscriptions/ProductSubscriptionLabel'
import { LicenseGenerationKeyWarning } from '../../../productSubscription/LicenseGenerationKeyWarning'

import { CodyServicesSection } from './CodyServicesSection'
import {
    queryClient,
    useArchiveEnterpriseSubscription,
    useGetEnterpriseSubscription,
    useListEnterpriseSubscriptionLicenses,
    type EnterprisePortalEnvironment,
} from './enterpriseportal'
import {
    EnterpriseSubscriptionCondition_Status,
    EnterpriseSubscriptionLicenseType,
    type ListEnterpriseSubscriptionLicensesResponse,
} from './enterpriseportalgen/subscriptions_pb'
import { SiteAdminGenerateProductLicenseForSubscriptionForm } from './SiteAdminGenerateProductLicenseForSubscriptionForm'
import { SiteAdminProductLicenseNode } from './SiteAdminProductLicenseNode'
import { enterprisePortalID } from './utils'

interface Props extends TelemetryV2Props {}

export const SiteAdminProductSubscriptionPage: React.FunctionComponent<React.PropsWithChildren<Props>> = props => (
    <QueryClientProvider client={queryClient}>
        <Page {...props} />
    </QueryClientProvider>
)

const QUERY_PARAM_ENV = 'env'

/**
 * Displays a product subscription in the site admin area.
 */
const Page: React.FunctionComponent<React.PropsWithChildren<Props>> = ({ telemetryRecorder }) => {
    const navigate = useNavigate()
    const { subscriptionUUID = '' } = useParams<{ subscriptionUUID: string }>()
    useEffect(() => telemetryRecorder.recordEvent('admin.productSubscription', 'view'), [telemetryRecorder])

    const [searchParams, setSearchParams] = useSearchParams()
    const [env, setEnv] = useState<EnterprisePortalEnvironment>(
        searchParams.get(QUERY_PARAM_ENV) || window.context.deployType === 'dev' ? 'dev' : 'prod'
    )
    useEffect(() => {
        searchParams.set(QUERY_PARAM_ENV, env)
        setSearchParams(searchParams)
    }, [env, setSearchParams, searchParams])

    const { data, isLoading, error } = useGetEnterpriseSubscription(env, subscriptionUUID)

    const [showGenerate, setShowGenerate] = useState<boolean>(false)

    const licenses = useListEnterpriseSubscriptionLicenses(
        env,
        [
            {
                filter: {
                    case: 'subscriptionId',
                    value: subscriptionUUID,
                },
            },
            {
                filter: {
                    // This UI only manages old-school license keys.
                    case: 'type',
                    value: EnterpriseSubscriptionLicenseType.KEY,
                },
            },
        ],
        { limit: 100, shouldLoad: !!data }
    )

    const {
        mutateAsync: archiveProductSubscription,
        isPending: archiveLoading,
        error: archiveError,
    } = useArchiveEnterpriseSubscription(env)

    const subscription = data?.subscription

    const onArchive = useCallback(async () => {
        if (!subscription) {
            return
        }
        const reason = window.prompt(
            'Do you really want to PERMANENTLY archive this subscription? All licenses associated with this subscription will be PERMANENTLY revoked, it will no longer be available for various Sourcegraph services, and changes can no longer be made to this subscription.\n\nHowever, it does NOT refund payment or cancel billing for you.\n\nEnter a revocation reason to continue.'
        )
        if (!reason || reason.length <= 3) {
            window.alert('Aborting.')
            return
        }
        try {
            telemetryRecorder.recordEvent('admin.productSubscription', 'archive')
            await archiveProductSubscription({
                reason,
                subscriptionId: subscription.id,
            })
            navigate('/site-admin/dotcom/product/subscriptions')
        } catch (error) {
            logger.error(error)
        }
    }, [subscription, archiveProductSubscription, navigate, telemetryRecorder])

    const toggleShowGenerate = useCallback((): void => setShowGenerate(previousValue => !previousValue), [])

    const onLicenseUpdate = useCallback(async () => {
        await licenses.refetch()
        setShowGenerate(false)
    }, [licenses])

    if (isLoading && !data) {
        return <LoadingSpinner />
    }

    const created = subscription?.conditions?.find(
        condition => condition.status === EnterpriseSubscriptionCondition_Status.CREATED
    )

    return (
        <div className="site-admin-product-subscription-page">
            <PageTitle title="Enterprise subscription" />
            <PageHeader
                headingElement="h2"
                path={[
                    { text: 'Enterprise subscriptions', to: '/site-admin/dotcom/product/subscriptions' },
                    { text: enterprisePortalID(subscriptionUUID) },
                ]}
                description={
                    subscription &&
                    created?.lastTransitionTime && (
                        <span className="text-muted">
                            Created <Timestamp date={created.lastTransitionTime.toDate()} />
                            {created?.message && <strong>{created.message}</strong>}
                        </span>
                    )
                }
                actions={
                    <>
                        <Select
                            id=""
                            name="env"
                            onChange={event => {
                                setEnv(event.target.value as EnterprisePortalEnvironment)
                            }}
                            value={env ?? undefined}
                            className="mb-0"
                            isCustomStyle={true}
                            label="Environment"
                        >
                            {[
                                { label: 'Production', value: 'prod' },
                                { label: 'Development', value: 'dev' },
                            ]
                                .concat(window.context.deployType === 'dev' ? [{ label: 'Local', value: 'local' }] : [])
                                .map(opt => (
                                    <option key={opt.value} value={opt.value} label={opt.label} />
                                ))}
                        </Select>
                        ,
                        <Button onClick={onArchive} disabled={archiveLoading} variant="danger">
                            Archive
                        </Button>
                    </>
                }
                className="mb-3"
            />
            {archiveError && <ErrorAlert className="mt-2" error={archiveError} />}
            {error && <ErrorAlert className="mt-2" error={error} />}

            {data && (
                <>
                    <H3>Details</H3>
                    <Container className="mb-3">
                        <table className="table mb-0">
                            <tbody>
                                <tr>
                                    <th className="text-nowrap">ID</th>
                                    <td className="w-100">{enterprisePortalID(subscriptionUUID)}</td>
                                </tr>
                                <td className="w-100">
                                    {licenses.data?.licenses && licenses.data?.licenses?.length > 0 && (
                                        <ProductSubscriptionLabel
                                            productName={licenses.data?.licenses[0].license.value?.planDisplayName}
                                            userCount={licenses.data?.licenses[0].license.value?.info?.userCount}
                                        />
                                    )}
                                </td>
                                <tr>
                                    <th className="text-nowrap">Salesforce Subscription</th>
                                    <td className="w-100">
                                        {subscription?.salesforce?.subscriptionId ? (
                                            <>{subscription?.salesforce?.subscriptionId}</>
                                        ) : (
                                            <span className="text-muted">None</span>
                                        )}
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </Container>

                    <CodyServicesSection
                        enterprisePortalEnvironment={env}
                        viewerCanAdminister={true}
                        productSubscriptionUUID={subscriptionUUID}
                        telemetryRecorder={telemetryRecorder}
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
                            env={env}
                            licenses={licenses}
                            toggleShowGenerate={toggleShowGenerate}
                            telemetryRecorder={telemetryRecorder}
                        />
                    </Container>
                </>
            )}
            {subscription && showGenerate && (
                <SiteAdminGenerateProductLicenseForSubscriptionForm
                    env={env}
                    subscription={subscription}
                    latestLicense={licenses.data?.licenses[0] ?? undefined}
                    onGenerate={onLicenseUpdate}
                    onCancel={() => setShowGenerate(false)}
                    telemetryRecorder={telemetryRecorder}
                />
            )}
        </div>
    )
}

interface ProductSubscriptionLicensesConnectionProps extends TelemetryV2Props {
    env: EnterprisePortalEnvironment
    licenses: UseQueryResult<ListEnterpriseSubscriptionLicensesResponse, ConnectError>
    toggleShowGenerate: () => void
}

const ProductSubscriptionLicensesConnection: React.FunctionComponent<ProductSubscriptionLicensesConnectionProps> = ({
    env,
    licenses: { data, refetch, error, isLoading },
    toggleShowGenerate,
    telemetryRecorder,
}) => {
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
            {isLoading && !data && <ConnectionLoading />}
            <ConnectionList as="ul" className="list-group list-group-flush mb-0" aria-label="Subscription licenses">
                {data?.licenses?.map(node => (
                    <SiteAdminProductLicenseNode
                        env={env}
                        key={node.id}
                        node={node}
                        defaultExpanded={node.id === licenseIDFromLocationHash}
                        showSubscription={false}
                        onRevokeCompleted={refetch}
                        telemetryRecorder={telemetryRecorder}
                    />
                ))}
            </ConnectionList>
            {data?.licenses?.length === 0 && <NoProductLicense toggleShowGenerate={toggleShowGenerate} />}
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
