import React, { useCallback, useState } from 'react'

import { mdiPencil, mdiTrashCan } from '@mdi/js'
import { parseISO } from 'date-fns'
import type { GraphQLError } from 'graphql'

import { Toggle } from '@sourcegraph/branded/src/components/Toggle'
import { logger } from '@sourcegraph/common'
import { useMutation, useQuery } from '@sourcegraph/http-client'
import { CodyGatewayRateLimitSource } from '@sourcegraph/shared/src/graphql-operations'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    H3,
    ProductStatusBadge,
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

import { CopyableText } from '../../../../components/CopyableText'
import type {
    CodyGatewayAccessFields,
    Scalars,
    UpdateCodyGatewayConfigResult,
    UpdateCodyGatewayConfigVariables,
    CodyGatewayRateLimitUsageDatapoint,
    CodyGatewayRateLimitFields,
    DotComProductSubscriptionCodyGatewayCompletionsUsageResult,
    DotComProductSubscriptionCodyGatewayCompletionsUsageVariables,
    DotComProductSubscriptionCodyGatewayEmbeddingsUsageVariables,
    DotComProductSubscriptionCodyGatewayEmbeddingsUsageResult,
} from '../../../../graphql-operations'
import { ChartContainer } from '../../../../site-admin/analytics/components/ChartContainer'

import {
    DOTCOM_PRODUCT_SUBSCRIPTION_CODY_GATEWAY_COMPLETIONS_USAGE,
    DOTCOM_PRODUCT_SUBSCRIPTION_CODY_GATEWAY_EMBEDDINGS_USAGE,
    UPDATE_CODY_GATEWAY_CONFIG,
} from './backend'
import { CodyGatewayRateLimitModal } from './CodyGatewayRateLimitModal'
import { ModelBadges } from './ModelBadges'
import { numberFormatter, prettyInterval } from './utils'

import styles from './CodyServicesSection.module.scss'

interface Props extends TelemetryV2Props {
    productSubscriptionUUID: string
    productSubscriptionID: Scalars['ID']
    currentSourcegraphAccessToken: string | null
    accessTokenError?: GraphQLError
    viewerCanAdminister: boolean
    refetchSubscription: () => Promise<any>
    codyGatewayAccess: CodyGatewayAccessFields
}

export const CodyServicesSection: React.FunctionComponent<Props> = ({
    productSubscriptionUUID,
    productSubscriptionID,
    viewerCanAdminister,
    currentSourcegraphAccessToken,
    accessTokenError,
    refetchSubscription,
    codyGatewayAccess,
    telemetryRecorder,
}) => {
    const [updateCodyGatewayConfig, { loading: updateCodyGatewayConfigLoading, error: updateCodyGatewayConfigError }] =
        useMutation<UpdateCodyGatewayConfigResult, UpdateCodyGatewayConfigVariables>(UPDATE_CODY_GATEWAY_CONFIG)

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
                variables: {
                    productSubscriptionID,
                    access: { enabled: codyServicesStateChange },
                },
            })
            await refetchSubscription()
        } catch (error) {
            logger.error(error)
        } finally {
            // Reset the intent to change state.
            setCodyServicesStateChange(undefined)
        }
    }, [
        productSubscriptionID,
        refetchSubscription,
        updateCodyGatewayConfig,
        codyServicesStateChange,
        telemetryRecorder,
    ])

    return (
        <>
            <H3>
                Cody services <ProductStatusBadge status="beta" />
            </H3>
            <Container className="mb-3">
                {currentSourcegraphAccessToken && (
                    <>
                        <div className="form-group mb-2">
                            {updateCodyGatewayConfigError && <ErrorAlert error={updateCodyGatewayConfigError} />}
                            <Label className="mb-0">
                                <Toggle
                                    id="cody-gateway-enabled"
                                    value={codyGatewayAccess.enabled}
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

                        {codyGatewayAccess.enabled && (
                            <>
                                <hr className="my-3" />

                                <H4>Completions</H4>
                                <Label className="mb-2">Rate limits</Label>
                                <table className={styles.limitsTable}>
                                    <thead>
                                        <tr>
                                            <th>Feature</th>
                                            <th>Source</th>
                                            <th>Rate limit</th>
                                            <th>Allowed models</th>
                                            {viewerCanAdminister && <th>Actions</th>}
                                        </tr>
                                    </thead>
                                    <tbody>
                                        <RateLimitRow
                                            mode="chat"
                                            productSubscriptionID={productSubscriptionID}
                                            rateLimit={codyGatewayAccess.chatCompletionsRateLimit}
                                            refetchSubscription={refetchSubscription}
                                            title="Chat and recipes"
                                            viewerCanAdminister={viewerCanAdminister}
                                        />
                                        <RateLimitRow
                                            mode="code"
                                            productSubscriptionID={productSubscriptionID}
                                            rateLimit={codyGatewayAccess.codeCompletionsRateLimit}
                                            refetchSubscription={refetchSubscription}
                                            title="Code completions"
                                            viewerCanAdminister={viewerCanAdminister}
                                        />
                                    </tbody>
                                </table>
                                <RateLimitUsage mode="completions" productSubscriptionUUID={productSubscriptionUUID} />

                                <hr className="my-3" />

                                <H4>Embeddings</H4>
                                <Label className="mb-2">Rate limits</Label>
                                <table className={styles.limitsTable}>
                                    <thead>
                                        <tr>
                                            <th>Feature</th>
                                            <th>Source</th>
                                            <th>Rate limit</th>
                                            <th>Allowed models</th>
                                            {viewerCanAdminister && <th>Actions</th>}
                                        </tr>
                                    </thead>
                                    <tbody>
                                        <RateLimitRow
                                            mode="embeddings"
                                            productSubscriptionID={productSubscriptionID}
                                            rateLimit={codyGatewayAccess.embeddingsRateLimit}
                                            refetchSubscription={refetchSubscription}
                                            title="Embeddings tokens"
                                            viewerCanAdminister={viewerCanAdminister}
                                        />
                                    </tbody>
                                </table>
                                <EmbeddingsRateLimitUsage
                                    mode="embeddings"
                                    productSubscriptionUUID={productSubscriptionUUID}
                                />
                            </>
                        )}

                        <hr className="my-3" />
                    </>
                )}

                <H4>Access token</H4>
                <Text className="mb-2">
                    Access tokens can be used for Cody Gateway access. In most cases this is not needed, since access
                    tokens are automatically generated from each instance's configured license key.
                </Text>
                {currentSourcegraphAccessToken && (
                    <CopyableText
                        label="Access token"
                        secret={true}
                        flex={true}
                        text={currentSourcegraphAccessToken}
                        className="mb-2"
                    />
                )}
                {(accessTokenError?.extensions?.code === 'ErrActiveLicenseRequired' && (
                    <Alert variant="info" className="mb-0">
                        {viewerCanAdminister && <>Create a license key to generate an access token automatically.</>}
                        {!viewerCanAdminister && (
                            <>
                                Once an active subscription has been purchased, an access token will be automatically
                                generated.
                            </>
                        )}
                    </Alert>
                )) ||
                    (accessTokenError && <ErrorAlert error={accessTokenError} className="mb-0" />)}
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
                <Tooltip content="The limit has been specified by a custom override">
                    <Badge variant="primary" className={className}>
                        Override
                    </Badge>
                </Tooltip>
            )
        }
        case CodyGatewayRateLimitSource.PLAN: {
            return (
                <Tooltip content="The limit is derived from the current subscription plan">
                    <Badge variant="primary" className={className}>
                        Plan
                    </Badge>
                </Tooltip>
            )
        }
    }
}

function generateSeries(data: CodyGatewayRateLimitUsageDatapoint[]): [string, CodyGatewayRateLimitUsageDatapoint[]][] {
    const series: Record<string, CodyGatewayRateLimitUsageDatapoint[]> = {}
    for (const entry of data) {
        if (!series[entry.model]) {
            series[entry.model] = []
        }
        series[entry.model].push(entry)
    }
    return Object.entries(series).map(entry => [entry[0], entry[1]])
}

interface RateLimitRowProps {
    productSubscriptionID: Scalars['ID']
    title: string
    viewerCanAdminister: boolean
    refetchSubscription: () => Promise<any>
    mode: 'chat' | 'code' | 'embeddings'
    rateLimit: CodyGatewayRateLimitFields | null
}

const RateLimitRow: React.FunctionComponent<RateLimitRowProps> = ({
    productSubscriptionID,
    title,
    mode,
    viewerCanAdminister,
    refetchSubscription,
    rateLimit,
}) => {
    const [showConfigModal, setShowConfigModal] = useState<boolean>(false)

    const [updateCodyGatewayConfig, { loading: updateCodyGatewayConfigLoading, error: updateCodyGatewayConfigError }] =
        useMutation<UpdateCodyGatewayConfigResult, UpdateCodyGatewayConfigVariables>(UPDATE_CODY_GATEWAY_CONFIG)

    const onRemoveRateLimitOverride = useCallback(async () => {
        try {
            await updateCodyGatewayConfig({
                variables: {
                    productSubscriptionID,
                    access:
                        mode === 'chat'
                            ? {
                                  chatCompletionsRateLimit: '0',
                                  chatCompletionsRateLimitIntervalSeconds: 0,
                                  chatCompletionsAllowedModels: [],
                              }
                            : mode === 'code'
                            ? {
                                  codeCompletionsRateLimit: '0',
                                  codeCompletionsRateLimitIntervalSeconds: 0,
                                  codeCompletionsAllowedModels: [],
                              }
                            : {
                                  embeddingsRateLimit: '0',
                                  embeddingsRateLimitIntervalSeconds: 0,
                                  embeddingsAllowedModels: [],
                              },
                },
            })
            await refetchSubscription()
        } catch (error) {
            logger.error(error)
        }
    }, [productSubscriptionID, refetchSubscription, updateCodyGatewayConfig, mode])

    const afterSaveRateLimit = useCallback(async () => {
        try {
            await refetchSubscription()
        } catch {
            // Ignore, these errors are shown elsewhere.
        }
        setShowConfigModal(false)
    }, [refetchSubscription])

    return (
        <>
            <tr>
                <td colSpan={rateLimit !== null ? 1 : viewerCanAdminister ? 5 : 4}>
                    <strong>{title}</strong>
                </td>
                {rateLimit !== null && (
                    <>
                        <td>
                            <CodyGatewayRateLimitSourceBadge source={rateLimit.source} />
                        </td>
                        <td>
                            {numberFormatter.format(BigInt(rateLimit.limit))}{' '}
                            {mode === 'embeddings' ? 'tokens' : 'requests'} /{' '}
                            {prettyInterval(rateLimit.intervalSeconds)}
                        </td>
                        <td>
                            <ModelBadges
                                models={rateLimit.allowedModels}
                                mode={mode === 'embeddings' ? 'embeddings' : 'completions'}
                            />
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
                                    <Tooltip content="Remove rate limit override">
                                        <Button
                                            size="sm"
                                            variant="link"
                                            aria-label="Remove rate limit override"
                                            className="ml-1"
                                            disabled={updateCodyGatewayConfigLoading}
                                            onClick={onRemoveRateLimitOverride}
                                        >
                                            <Icon aria-hidden={true} svgPath={mdiTrashCan} className="text-danger" />
                                        </Button>
                                    </Tooltip>
                                )}
                                {updateCodyGatewayConfigError && <ErrorAlert error={updateCodyGatewayConfigError} />}
                            </td>
                        )}
                    </>
                )}
            </tr>
            {showConfigModal && (
                <CodyGatewayRateLimitModal
                    productSubscriptionID={productSubscriptionID}
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
    productSubscriptionUUID: string
    mode: 'completions' | 'embeddings'
}

const RateLimitUsage: React.FunctionComponent<RateLimitUsageProps> = ({ productSubscriptionUUID }) => {
    const { data, loading, error } = useQuery<
        DotComProductSubscriptionCodyGatewayCompletionsUsageResult,
        DotComProductSubscriptionCodyGatewayCompletionsUsageVariables
    >(DOTCOM_PRODUCT_SUBSCRIPTION_CODY_GATEWAY_COMPLETIONS_USAGE, { variables: { uuid: productSubscriptionUUID } })

    if (loading && !data) {
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

    const { codyGatewayAccess } = data!.dotcom.productSubscription

    return (
        <>
            <H5 className="mb-2">Usage</H5>
            <ChartContainer labelX="Date" labelY="Daily usage">
                {width => (
                    <LineChart
                        width={width}
                        height={200}
                        series={[
                            ...generateSeries(codyGatewayAccess.chatCompletionsRateLimit?.usage ?? []).map(
                                ([model, data]): Series<CodyGatewayRateLimitUsageDatapoint> => ({
                                    data,
                                    getXValue(datum) {
                                        return parseISO(datum.date)
                                    },
                                    getYValue(datum) {
                                        return Number(datum.count)
                                    },
                                    id: 'chat-usage',
                                    name: 'Chat completions: ' + model,
                                    color: 'var(--purple)',
                                })
                            ),
                            ...generateSeries(codyGatewayAccess.codeCompletionsRateLimit?.usage ?? []).map(
                                ([model, data]): Series<CodyGatewayRateLimitUsageDatapoint> => ({
                                    data,
                                    getXValue(datum) {
                                        return parseISO(datum.date)
                                    },
                                    getYValue(datum) {
                                        return Number(datum.count)
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

const EmbeddingsRateLimitUsage: React.FunctionComponent<RateLimitUsageProps> = ({ productSubscriptionUUID }) => {
    const { data, loading, error } = useQuery<
        DotComProductSubscriptionCodyGatewayEmbeddingsUsageResult,
        DotComProductSubscriptionCodyGatewayEmbeddingsUsageVariables
    >(DOTCOM_PRODUCT_SUBSCRIPTION_CODY_GATEWAY_EMBEDDINGS_USAGE, { variables: { uuid: productSubscriptionUUID } })

    if (loading && !data) {
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

    const { codyGatewayAccess } = data!.dotcom.productSubscription

    return (
        <>
            <H5 className="mb-2">Usage</H5>
            <ChartContainer labelX="Date" labelY="Daily usage">
                {width => (
                    <LineChart
                        width={width}
                        height={200}
                        series={[
                            ...generateSeries(codyGatewayAccess.embeddingsRateLimit?.usage ?? []).map(
                                ([model, data]): Series<CodyGatewayRateLimitUsageDatapoint> => ({
                                    data,
                                    getXValue(datum) {
                                        return parseISO(datum.date)
                                    },
                                    getYValue(datum) {
                                        return Number(datum.count)
                                    },
                                    id: 'chat-usage',
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
