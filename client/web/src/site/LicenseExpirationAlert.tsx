import React from 'react'

import classNames from 'classnames'
import { formatDistanceStrict } from 'date-fns'

import { Link } from '@sourcegraph/wildcard'

import { DismissibleAlert } from '../components/DismissibleAlert'
import { isProductLicenseExpired, formatRelativeExpirationDate } from '../productSubscription/helpers'

/**
 * A global alert that appears telling the site admin that their license key is about to expire. Even after being dismissed,
 * it reappears every day.
 */
export const LicenseExpirationAlert: React.FunctionComponent<
    React.PropsWithChildren<{
        expiresAt: Date
        daysLeft: number
        className?: string
    }>
> = ({ expiresAt, daysLeft, className }) => (
    <DismissibleAlert
        partialStorageKey={`licenseExpiring.${daysLeft}`}
        variant="warning"
        className={classNames('align-items-center', className)}
    >
        Your Sourcegraph license{' '}
        {
            isProductLicenseExpired(expiresAt)
                ? 'expired ' + formatRelativeExpirationDate(expiresAt) // 'Expired two months ago'
                : 'will expire in ' + formatDistanceStrict(expiresAt, Date.now()) // 'Will expire in two months'
        }
        .&nbsp;
        <Link className="site-alert__link" to="/site-admin/license">
            <span className="underline">Renew now</span>
        </Link>
        &nbsp;or&nbsp;
        <Link className="site-alert__link" to="https://sourcegraph.com/contact">
            <span className="underline">contact Sourcegraph</span>
        </Link>
    </DismissibleAlert>
)
