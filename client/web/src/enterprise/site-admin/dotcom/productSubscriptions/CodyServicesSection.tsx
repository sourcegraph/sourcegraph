import React, { useCallback, useState } from 'react'

import type { ConnectError } from '@connectrpc/connect'
import { mdiInformationOutline, mdiPencil, mdiTrashCan } from '@mdi/js'
import type { UseQueryResult } from '@tanstack/react-query'

import { Toggle } from '@sourcegraph/branded/src/components/Toggle'
import { logger } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    H3,
    Container,
    Text,
    H4,
    ErrorAlert,
    LoadingSpinner,
    Button,
    Icon,
    Badge,
    Tooltip,
    Label,
    H5,
    LineChart,
    type Series,
    Modal,
    Alert,
} from '@sourcegraph/wildcard'

import { Collapsible } from '../../../../components/Collapsible'
import { CopyableText } from '../../../../components/CopyableText'
import { ChartContainer } from '../../../../site-admin/analytics/components/ChartContainer'

import { CodyGatewayRateLimitModal } from './CodyGatewayRateLimitModal'
import {
    useGetCodyGatewayAccess,
    useGetCodyGatewayUsage,
    useUpdateCodyGatewayAccess,
    type EnterprisePortalEnvironment,
} from './enterpriseportal'
import {
    type CodyGatewayUsage_UsageDatapoint,
    type GetCodyGatewayUsageResponse,
    type CodyGatewayRateLimit,
    CodyGatewayRateLimitSource,
} from './enterpriseportalgen/codyaccess_pb'
import { numberFormatter, prettyInterval } from './utils'

import styles from './CodyServicesSection.module.scss'

interface Props extends TelemetryV2Props {
    enterprisePortalEnvironment: EnterprisePortalEnvironment
    productSubscriptionUUID: string
    viewerCanAdminister: boolean
}

export const CodyServicesSection: React.FunctionComponent<Props> = ({
    enterprisePortalEnvironment,
    productSubscriptionUUID,
    viewerCanAdminister,
    telemetryRecorder,
}) => {
    const codyGatewayUsageQuery = useGetCodyGatewayUsage(enterprisePortalEnvironment, productSubscriptionUUID)

    const {
        data: codyGatewayAccessResponse,
        isLoading: getCodyGatewayAccessLoading,
        error: getCodyGatewayAccessError,
        refetch: refetchCodyGatewayAccess,
    } = useGetCodyGatewayAccess(enterprisePortalEnvironment, productSubscriptionUUID)

    const {
        mutateAsync: updateCodyGatewayConfig,
        isPending: updateCodyGatewayConfigLoading,
        error: updateCodyGatewayConfigError,
    } = useUpdateCodyGatewayAccess(enterprisePortalEnvironment)

    const [codyServicesStateChange, setCodyServicesStateChange] = useState<boolean | undefined>()

    const onCancelToggleCodyServices = useCallback(() => {
        setCodyServicesStateChange(undefined)
    }, [])

    const onToggleCodyServices = useCallback(async () => {
        if (typeof codyServicesStateChange !== 'boolean') {
            return
        }
        try {
            telemetryRecorder.recordEvent(
                'admin.productSubscription.codyAccess',
                codyServicesStateChange ? 'enable' : 'disable'
            )
            await updateCodyGatewayConfig({
                updateMask: { paths: ['enabled'] },
                access: {
                    subscriptionId: productSubscriptionUUID,
                    enabled: codyServicesStateChange,
                },
            })
            await refetchCodyGatewayAccess()
        } catch (error) {
            logger.error(error)
        } finally {
            // Reset the intent to change state.
            setCodyServicesStateChange(undefined)
        }
    }, [
        productSubscriptionUUID,
        refetchCodyGatewayAccess,
        updateCodyGatewayConfig,
        codyServicesStateChange,
        telemetryRecorder,
    ])

    const header = <H3>Cody services</H3>

    if (getCodyGatewayAccessLoading && !codyGatewayAccessResponse) {
        return (
            <>
                {header}
                <LoadingSpinner />
            </>
        )
    }

    if (getCodyGatewayAccessError) {
        return (
            <>
                {header}
                <ErrorAlert className="my-2" error={getCodyGatewayAccessError} />
            </>
        )
    }

    const { access: codyGatewayAccess } = codyGatewayAccessResponse!

    return (
        <>
            {header}
            <Container className="mb-3">
                <>
                    <div className="form-group mb-2">
                        {updateCodyGatewayConfigError && <ErrorAlert error={updateCodyGatewayConfigError} />}
                        <Label className="mb-0">
                            <Toggle
                                id="cody-gateway-enabled"
                                value={codyGatewayAccess?.enabled}
                                disabled={updateCodyGatewayConfigLoading || !viewerCanAdminister}
                                onToggle={setCodyServicesStateChange}
                                className="mr-1 align-text-bottom"
                            />
                            Access to hosted Cody services
                            {updateCodyGatewayConfigLoading && (
                                <>
                                    {' '}
                                    <LoadingSpinner />
                                </>
                            )}
                        </Label>
                    </div>

                    {codyGatewayAccess?.enabled && (
                        <>
                            <hr className="my-3" />

                            <H4>Completions</H4>
                            <Label className="mb-2">Rate limits</Label>
                            <table className={styles.limitsTable}>
                                <thead>
                                    <tr>
                                        <th>Feature</th>
                                        <th>
                                            Source{' '}
                                            <Tooltip content="Where the displayed rate limit comes from - hover over each badge to learn more.">
                                                <Icon aria-label="Show help text" svgPath={mdiInformationOutline} />
                                            </Tooltip>
                                        </th>
                                        <th>Rate limit</th>
                                        {viewerCanAdminister && <th>Actions</th>}
                                    </tr>
                                </thead>
                                <tbody>
                                    <RateLimitRow
                                        mode="chat"
                                        enterprisePortalEnvironment={enterprisePortalEnvironment}
                                        productSubscriptionUUID={productSubscriptionUUID}
                                        rateLimit={codyGatewayAccess?.chatCompletionsRateLimit}
                                        refetchCodyGatewayAccess={refetchCodyGatewayAccess}
                                        title="Chat and recipes"
                                        viewerCanAdminister={viewerCanAdminister}
                                    />
                                    <RateLimitRow
                                        mode="code"
                                        enterprisePortalEnvironment={enterprisePortalEnvironment}
                                        productSubscriptionUUID={productSubscriptionUUID}
                                        rateLimit={codyGatewayAccess?.codeCompletionsRateLimit}
                                        refetchCodyGatewayAccess={refetchCodyGatewayAccess}
                                        title="Code completions"
                                        viewerCanAdminister={viewerCanAdminister}
                                    />
                                </tbody>
                            </table>
                            <RateLimitUsage mode="completions" usageQuery={codyGatewayUsageQuery} />

                            <hr className="my-3" />

                            <H4>Embeddings</H4>
                            <Label className="mb-2">Rate limits</Label>
                            <table className={styles.limitsTable}>
                                <thead>
                                    <tr>
                                        <th>Feature</th>
                                        <th>Source</th>
                                        <th>Rate limit</th>
                                        {viewerCanAdminister && <th>Actions</th>}
                                    </tr>
                                </thead>
                                <tbody>
                                    <RateLimitRow
                                        mode="embeddings"
                                        enterprisePortalEnvironment={enterprisePortalEnvironment}
                                        productSubscriptionUUID={productSubscriptionUUID}
                                        rateLimit={codyGatewayAccess?.embeddingsRateLimit}
                                        refetchCodyGatewayAccess={refetchCodyGatewayAccess}
                                        title="Embeddings tokens"
                                        viewerCanAdminister={viewerCanAdminister}
                                    />
                                </tbody>
                            </table>
                            <EmbeddingsRateLimitUsage mode="embeddings" usageQuery={codyGatewayUsageQuery} />
                        </>
                    )}

                    <hr className="my-3" />
                </>
                <Collapsible titleAtStart={true} title={<H4>Cody Gateway access token</H4>} defaultExpanded={false}>
                    <Text className="mb-2">
                        Access tokens are automatically generated from each instance's configured license key. The only
                        action required for Cody Gateway access is to configure a valid, current license key on the
                        Sourcegraph instance.
                        <Alert variant="warning" className="mt-2">
                            <strong>DO NOT USE the access token here unless you know what you are doing!</strong> The
                            displayed token is only valid until the current license's expiry or revocation. This should
                            only be used for debugging purposes.
                        </Alert>
                    </Text>
                    {codyGatewayAccess?.accessTokens && codyGatewayAccess?.accessTokens.length > 0 ? (
                        <CopyableText
                            label="Access token"
                            secret={true}
                            flex={true}
                            text={codyGatewayAccess?.accessTokens[0].token}
                            className="mb-2"
                        />
                    ) : (
                        <Alert variant="info" className="mb-0">
                            {viewerCanAdminister && (
                                <>Create a license key to generate an access token automatically.</>
                            )}
                            {!viewerCanAdminister && (
                                <>
                                    Once an active subscription has been purchased, an access token will be
                                    automatically generated.
                                </>
                            )}
                        </Alert>
                    )}
                </Collapsible>
            </Container>

            {typeof codyServicesStateChange === 'boolean' && (
                <ToggleCodyServicesConfirmationModal
                    onAccept={onToggleCodyServices}
                    onCancel={onCancelToggleCodyServices}
                    targetState={codyServicesStateChange}
                />
            )}
        </>
    )
}

export const CodyGatewayRateLimitSourceBadge: React.FunctionComponent<{
    source: CodyGatewayRateLimitSource
    className?: string
}> = ({ source, className }) => {
    switch (source) {
        case CodyGatewayRateLimitSource.OVERRIDE: {
            return (
                <Tooltip content="The limit has been specified by a fixed, custom override. Removing this override will revert the limit to the value derived from the active license plan.">
                    <Badge variant="warning" className={className}>
                        Override
                    </Badge>
                </Tooltip>
            )
        }
        case CodyGatewayRateLimitSource.PLAN: {
            return (
                <Tooltip content="The limit is derived from the actve license plan, which scales off of user count on the active license as well.">
                    <Badge variant="primary" className={className}>
                        Plan
                    </Badge>
                </Tooltip>
            )
        }
        case CodyGatewayRateLimitSource.UNSPECIFIED: {
            return (
                <Tooltip content="The limit has an unknown source">
                    <Badge variant="danger" className={className}>
                        Unknown
                    </Badge>
                </Tooltip>
            )
        }
    }
}

function generateSeries(data: CodyGatewayUsage_UsageDatapoint[]): [string, CodyGatewayUsage_UsageDatapoint[]][] {
    const series: Record<string, CodyGatewayUsage_UsageDatapoint[]> = {}
    for (const entry of data) {
        if (!series[entry.model]) {
            series[entry.model] = []
        }
        series[entry.model].push(entry)
    }
    return Object.entries(series).map(entry => [entry[0], entry[1]])
}

interface RateLimitRowProps {
    enterprisePortalEnvironment: EnterprisePortalEnvironment
    productSubscriptionUUID: string
    title: string
    viewerCanAdminister: boolean
    refetchCodyGatewayAccess: () => Promise<any>
    mode: 'chat' | 'code' | 'embeddings'
    rateLimit: CodyGatewayRateLimit | undefined
}

const RateLimitRow: React.FunctionComponent<RateLimitRowProps> = ({
    enterprisePortalEnvironment,
    productSubscriptionUUID,
    title,
    mode,
    viewerCanAdminister,
    refetchCodyGatewayAccess,
    rateLimit,
}) => {
    const [showConfigModal, setShowConfigModal] = useState<boolean>(false)

    const {
        error: updateCodyGatewayAccessError,
        isPending: updateCodyGatewayAccessLoading,
        mutateAsync: updateCodyGatewayAccess,
    } = useUpdateCodyGatewayAccess(enterprisePortalEnvironment)

    const onRemoveRateLimitOverride = useCallback(async () => {
        try {
            // FieldMask dictates fields to update, which allows us to forcibly
            // set null values to clear out an override.
            const paths: string[] = []
            switch (mode) {
                case 'chat': {
                    paths.push('chat_completions_rate_limit.limit', 'chat_completions_rate_limit.interval_duration')
                    break
                }
                case 'code': {
                    paths.push('code_completions_rate_limit.limit', 'code_completions_rate_limit.interval_duration')
                    break
                }
                case 'embeddings': {
                    paths.push('embeddings_rate_limit.limit', 'embeddings_rate_limit.interval_duration')
                    break
                }
            }
            await updateCodyGatewayAccess({
                updateMask: { paths },
                access: {
                    subscriptionId: productSubscriptionUUID,
                },
            })
            await refetchCodyGatewayAccess()
        } catch (error) {
            logger.error(error)
        }
    }, [productSubscriptionUUID, refetchCodyGatewayAccess, updateCodyGatewayAccess, mode])

    const afterSaveRateLimit = useCallback(async () => {
        try {
            await refetchCodyGatewayAccess()
        } catch {
            // Ignore, these errors are shown elsewhere.
        }
        setShowConfigModal(false)
    }, [refetchCodyGatewayAccess])

    return (
        <>
            <tr>
                <td colSpan={rateLimit !== undefined ? 1 : viewerCanAdminister ? 5 : 4}>
                    <strong>{title}</strong>
                </td>
                {rateLimit !== undefined && (
                    <>
                        <td>
                            <CodyGatewayRateLimitSourceBadge source={rateLimit.source} />
                        </td>
                        <td>
                            {numberFormatter.format(BigInt(rateLimit.limit))}{' '}
                            {mode === 'embeddings' ? 'tokens' : 'requests'} /{' '}
                            {prettyInterval(Number(rateLimit.intervalDuration?.seconds || 0))}
                        </td>
                        {viewerCanAdminister && (
                            <td>
                                <Button
                                    size="sm"
                                    variant="link"
                                    aria-label="Edit rate limit"
                                    className="ml-1"
                                    onClick={() => setShowConfigModal(true)}
                                >
                                    <Icon aria-hidden={true} svgPath={mdiPencil} />
                                </Button>
                                {rateLimit.source === CodyGatewayRateLimitSource.OVERRIDE && (
                                    <Tooltip content="Remove rate limit override, allowing the active-license-based defaults to take precedence.">
                                        <Button
                                            size="sm"
                                            variant="link"
                                            aria-label="Remove rate limit override"
                                            className="ml-1"
                                            disabled={updateCodyGatewayAccessLoading}
                                            onClick={onRemoveRateLimitOverride}
                                        >
                                            <Icon aria-hidden={true} svgPath={mdiTrashCan} className="text-danger" />
                                        </Button>
                                    </Tooltip>
                                )}
                                {updateCodyGatewayAccessError && <ErrorAlert error={updateCodyGatewayAccessError} />}
                            </td>
                        )}
                    </>
                )}
            </tr>
            {showConfigModal && (
                <CodyGatewayRateLimitModal
                    enterprisePortalEnvironment={enterprisePortalEnvironment}
                    productSubscriptionUUID={productSubscriptionUUID}
                    afterSave={afterSaveRateLimit}
                    current={rateLimit}
                    onCancel={() => setShowConfigModal(false)}
                    mode={mode}
                />
            )}
        </>
    )
}

interface RateLimitUsageProps {
    usageQuery: UseQueryResult<GetCodyGatewayUsageResponse, ConnectError>
    mode: 'completions' | 'embeddings'
}

const RateLimitUsage: React.FunctionComponent<RateLimitUsageProps> = ({ usageQuery: { data, isLoading, error } }) => {
    if (isLoading && !data) {
        return (
            <>
                <H5 className="mb-2">Usage</H5>
                <LoadingSpinner />
            </>
        )
    }

    if (error) {
        return (
            <>
                <H5 className="mb-2">Usage</H5>
                <ErrorAlert error={error} />
            </>
        )
    }

    const usage = data?.usage
    return (
        <>
            <H5 className="mb-2">Usage</H5>
            <ChartContainer labelX="Date" labelY="Daily usage">
                {width => (
                    <LineChart
                        width={width}
                        height={200}
                        series={[
                            ...generateSeries(usage?.chatCompletionsUsage ?? []).map(
                                ([model, data]): Series<CodyGatewayUsage_UsageDatapoint> => ({
                                    data,
                                    getXValue(datum) {
                                        return datum.time?.toDate() || new Date()
                                    },
                                    getYValue(datum) {
                                        return Number(datum.usage)
                                    },
                                    id: 'chat-usage',
                                    name: 'Chat completions: ' + model,
                                    color: 'var(--purple)',
                                })
                            ),
                            ...generateSeries(data?.usage?.codeCompletionsUsage ?? []).map(
                                ([model, data]): Series<CodyGatewayUsage_UsageDatapoint> => ({
                                    data,
                                    getXValue(datum) {
                                        return datum.time?.toDate() || new Date()
                                    },
                                    getYValue(datum) {
                                        return Number(datum.usage)
                                    },
                                    id: 'code-completions-usage',
                                    name: 'Code completions: ' + model,
                                    color: 'var(--orange)',
                                })
                            ),
                        ]}
                    />
                )}
            </ChartContainer>
        </>
    )
}

const EmbeddingsRateLimitUsage: React.FunctionComponent<RateLimitUsageProps> = ({
    usageQuery: { data, isLoading, error },
}) => {
    if (isLoading && !data) {
        return (
            <>
                <H5 className="mb-2">Usage</H5>
                <LoadingSpinner />
            </>
        )
    }

    if (error) {
        return (
            <>
                <H5 className="mb-2">Usage</H5>
                <ErrorAlert error={error} />
            </>
        )
    }

    const usage = data?.usage
    return (
        <>
            <H5 className="mb-2">Usage</H5>
            <ChartContainer labelX="Date" labelY="Daily usage">
                {width => (
                    <LineChart
                        width={width}
                        height={200}
                        series={[
                            ...generateSeries(usage?.embeddingsUsage ?? []).map(
                                ([model, data]): Series<CodyGatewayUsage_UsageDatapoint> => ({
                                    data,
                                    getXValue(datum) {
                                        return datum.time?.toDate() || new Date()
                                    },
                                    getYValue(datum) {
                                        return Number(datum.usage)
                                    },
                                    id: 'embeddings-usage',
                                    name: 'Embedded tokens: ' + model,
                                    color: 'var(--purple)',
                                })
                            ),
                        ]}
                    />
                )}
            </ChartContainer>
        </>
    )
}

interface ToggleCodyServicesConfirmationModalProps {
    onAccept: () => void
    onCancel: () => void
    targetState: boolean
}

const ToggleCodyServicesConfirmationModal: React.FunctionComponent<ToggleCodyServicesConfirmationModalProps> = ({
    onCancel,
    onAccept,
    targetState,
}) => {
    const labelId = 'toggle-cody-services'
    return (
        <Modal onDismiss={onCancel} aria-labelledby={labelId}>
            <H3 id={labelId}>{targetState ? 'Enable' : 'Disable'} access to Cody Gateway</H3>
            <Text>
                Cody Gateway is a Sourcegraph managed service that allows customer instances to talk to upstream LLMs
                and generate embeddings under our negotiated terms with third party providers in a safe manner.
            </Text>

            <Alert variant="info">Note that changes may take up to 10 minutes to propagate.</Alert>

            <div className="d-flex justify-content-end">
                <Button className="mr-2" onClick={onCancel} outline={true} variant="secondary">
                    Cancel
                </Button>
                <Button variant="primary" onClick={onAccept}>
                    {targetState ? 'Enable' : 'Disable'}
                </Button>
            </div>
        </Modal>
    )
}
