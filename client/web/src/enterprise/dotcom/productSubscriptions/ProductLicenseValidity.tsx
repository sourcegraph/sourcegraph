import React from 'react'

import { mdiCheckCircle, mdiCloseCircle, mdiShieldRemove } from '@mdi/js'
import classNames from 'classnames'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { Icon, Label } from '@sourcegraph/wildcard'

import { isProductLicenseExpired } from '../../../productSubscription/helpers'
import {
    EnterpriseSubscriptionLicenseCondition_Status,
    type EnterpriseSubscriptionLicenseKey_Info,
    type EnterpriseSubscriptionLicenseCondition,
} from '../../site-admin/dotcom/productSubscriptions/enterpriseportalgen/subscriptions_pb'

const getIcon = (isExpired: boolean, isRevoked: boolean): string => {
    if (isExpired) {
        return mdiCloseCircle
    }
    if (isRevoked) {
        return mdiShieldRemove
    }
    return mdiCheckCircle
}

const ValidityIcon: React.FC<{ isExpired: boolean; isRevoked: boolean }> = ({ isExpired, isRevoked }) => (
    <Icon
        svgPath={getIcon(isExpired, isRevoked)}
        aria-hidden={true}
        className={classNames('mr-1', {
            ['text-success']: !isExpired && !isRevoked,
            ['text-muted']: isExpired && !isRevoked,
            ['text-danger']: isRevoked,
        })}
    />
)

const getText = (isExpired: boolean, isRevoked: boolean): string => {
    if (isExpired) {
        return 'Expired'
    }
    if (isRevoked) {
        return 'Revoked'
    }
    return 'Valid'
}

/**
 * Displays an alert indicating the validity of a product license.
 */
export const ProductLicenseValidity: React.FunctionComponent<
    React.PropsWithChildren<{
        licenseInfo: EnterpriseSubscriptionLicenseKey_Info | undefined
        licenseConditions: EnterpriseSubscriptionLicenseCondition[]
        variant?: 'icon-only' | 'no-icon'
        className?: string
    }>
> = ({ licenseInfo: info, licenseConditions: conditions, variant, className = '' }) => {
    const expiresAt = info?.expireTime?.toDate() ?? 0
    const isExpired = isProductLicenseExpired(expiresAt)

    const revoked = conditions.find(
        condition => condition.status === EnterpriseSubscriptionLicenseCondition_Status.REVOKED
    )
    const timestamp = revoked?.lastTransitionTime?.toDate() ?? expiresAt
    const timestampSuffix = isExpired || revoked ? 'ago' : 'remaining'

    if (variant === 'icon-only') {
        return (
            <div className={className}>
                <ValidityIcon isExpired={isExpired} isRevoked={!!revoked} />
            </div>
        )
    }
    return (
        <div className={className}>
            {variant !== 'no-icon' && <ValidityIcon isExpired={isExpired} isRevoked={!!revoked} />}
            {getText(isExpired, !!revoked)}, <Timestamp date={timestamp} noAbout={true} noAgo={true} utc={true} />{' '}
            {timestampSuffix}
            {revoked?.message && (
                <>
                    <Label className="ml-2 mb-0 d-inline">Reason:</Label> {revoked.message}
                </>
            )}
        </div>
    )
}
