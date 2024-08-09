import React, { useCallback, useState } from 'react'

import { mdiChevronDown, mdiChevronUp } from '@mdi/js'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Alert,
    ErrorMessage,
    Button,
    Collapse,
    CollapseHeader,
    CollapsePanel,
    Grid,
    H3,
    Icon,
    Label,
    Link,
    Text,
    Badge,
} from '@sourcegraph/wildcard'

import { CopyableText } from '../../../../components/CopyableText'
import { LoaderButton } from '../../../../components/LoaderButton'
import { isProductLicenseExpired } from '../../../../productSubscription/helpers'
import { ProductLicenseValidity } from '../../../dotcom/productSubscriptions/ProductLicenseValidity'
import { ProductSubscriptionLabel } from '../../../dotcom/productSubscriptions/ProductSubscriptionLabel'
import { ProductLicenseTags, UnknownTagWarning, hasUnknownTags } from '../../../productSubscription/ProductLicenseTags'

import { useRevokeEnterpriseSubscriptionLicense, type EnterprisePortalEnvironment } from './enterpriseportal'
import {
    EnterpriseSubscriptionLicenseCondition_Status,
    type EnterpriseSubscriptionLicense,
} from './enterpriseportalgen/subscriptions_pb'

export interface SiteAdminProductLicenseNodeProps extends TelemetryV2Props {
    env: EnterprisePortalEnvironment
    node: EnterpriseSubscriptionLicense
    showSubscription: boolean
    defaultExpanded?: boolean
    onRevokeCompleted: () => void
    isActiveLicense?: boolean
}

/**
 * Displays a product license in a connection in the site admin area.
 */
export const SiteAdminProductLicenseNode: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminProductLicenseNodeProps>
> = ({
    env,
    node,
    showSubscription,
    onRevokeCompleted,
    defaultExpanded = false,
    telemetryRecorder,
    isActiveLicense,
}) => {
    const {
        mutate: revoke,
        isPending: isRevokeLoading,
        error: revokeError,
    } = useRevokeEnterpriseSubscriptionLicense(env)

    const onRevoke = useCallback(() => {
        const reason = window.prompt(
            '⚠️ This is a PERMANENT operation. Enter the reason for revoking the license key to continue:'
        )
        if (reason) {
            telemetryRecorder.recordEvent('admin.productSubscription.license', 'revoke')
            revoke(
                { licenseId: node.id, reason },
                {
                    onSuccess: () => {
                        if (onRevokeCompleted) {
                            onRevokeCompleted()
                        }
                    },
                }
            )
        }
    }, [revoke, node, onRevokeCompleted, telemetryRecorder])

    const [open, setOpen] = useState(defaultExpanded)
    const toggleOpen = useCallback(() => {
        setOpen(!open)
    }, [open, setOpen])

    if (node.license.case !== 'key') {
        return (
            <Alert>
                <ErrorMessage error="Unknown license type" />
            </Alert>
        )
    }
    const licenseKey = node.license.value
    const info = licenseKey?.info

    const created = node.conditions.find(
        condition => condition.status === EnterpriseSubscriptionLicenseCondition_Status.CREATED
    )
    const revoked = node.conditions.find(
        condition => condition.status === EnterpriseSubscriptionLicenseCondition_Status.REVOKED
    )

    return (
        <li className="list-group-item p-3 mb-3 border" id={node.id}>
            <Collapse isOpen={open} onOpenChange={setOpen}>
                <Grid columnCount={2} templateColumns="auto 1fr" spacing={0}>
                    <Button variant="icon" onClick={toggleOpen} className="pr-3">
                        <Icon
                            aria-label={`collapse ${open ? 'opened' : 'closed'}`}
                            svgPath={open ? mdiChevronUp : mdiChevronDown}
                        />
                    </Button>
                    <CollapseHeader as="div" className="d-flex align-items-start">
                        <div>
                            {showSubscription && (
                                <div className="text-truncate d-flex">
                                    <H3>
                                        License in{' '}
                                        <Link
                                            to={`/site-admin/dotcom/product/subscriptions/${node.subscriptionId}#${node.id}?env=${env}`}
                                            className="mr-3"
                                        >
                                            {node.subscriptionId}
                                        </Link>
                                    </H3>
                                </div>
                            )}
                            {!isRevokeLoading && revokeError && (
                                <Alert variant="danger">Error revoking license: {revokeError.message}</Alert>
                            )}
                            <div className="mb-1">
                                {info && (
                                    <ProductSubscriptionLabel
                                        productName={licenseKey.planDisplayName}
                                        userCount={info.userCount}
                                        className="mb-0"
                                    />
                                )}
                                {isActiveLicense && (
                                    <Badge variant="primary" className="ml-2" small={true}>
                                        Active license
                                    </Badge>
                                )}
                            </div>
                            {created?.lastTransitionTime && (
                                <Text className="mb-2">
                                    <small className="text-muted">
                                        Created <Timestamp date={created?.lastTransitionTime.toDate()} />
                                    </small>
                                </Text>
                            )}

                            <ProductLicenseValidity licenseInfo={info} licenseConditions={node.conditions} />
                        </div>
                        {!revoked && !isProductLicenseExpired(info?.expireTime?.toDate() ?? 0) && (
                            <LoaderButton
                                className="ml-auto"
                                variant="danger"
                                label="Revoke"
                                onClick={onRevoke}
                                loading={isRevokeLoading}
                            />
                        )}
                    </CollapseHeader>
                    <div />
                    <CollapsePanel className="mt-4">
                        <div className="d-flex">
                            <Label>License Key ID</Label>
                            <Text className="ml-3">
                                <span className="text-monospace">{node.id}</span>
                            </Text>
                        </div>
                        <div className="d-flex">
                            <Label>Key Version</Label>
                            <Text className="ml-3">{licenseKey.infoVersion}</Text>
                        </div>
                        {licenseKey.infoVersion > 1 && (
                            <>
                                <div className="d-flex">
                                    <Label>Salesforce Opportunity ID</Label>
                                    <Text className="ml-3">
                                        {info?.salesforceOpportunityId ? (
                                            <span className="text-monospace">{info?.salesforceOpportunityId}</span>
                                        ) : (
                                            <span className="text-muted">Uknown</span>
                                        )}
                                    </Text>
                                </div>
                            </>
                        )}
                        {info && info.tags.length > 0 && (
                            <>
                                {hasUnknownTags(info.tags) && <UnknownTagWarning className="mb-2" />}
                                <Label className="w-100">
                                    <Text className="mb-2">Tags</Text>
                                    <Text className="mb-2">
                                        <ProductLicenseTags tags={info.tags} />
                                    </Text>
                                </Label>
                            </>
                        )}
                        <Label className="w-100">
                            <Text className="mb-2">License Key</Text>
                            <CopyableText flex={true} text={licenseKey.licenseKey} secret={true} />
                        </Label>
                    </CollapsePanel>
                </Grid>
            </Collapse>
        </li>
    )
}
