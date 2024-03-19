import React from 'react'

import { UTCDate } from '@date-fns/utc'
import { format } from 'date-fns'

import { formatRelativeExpirationDate, isProductLicenseExpired } from '../../productSubscription/helpers'

/** Displays an expiration date (for product subscriptions or licenses). */
export const ExpirationDate: React.FunctionComponent<
    React.PropsWithChildren<{
        /** The expiration date of a product subscription or license. */
        date: Date | number

        /** Show the time of day of expiration. */
        showTime?: boolean

        /** Show "(T remaining)" or "(T ago)" relative times. */
        showRelative?: boolean

        /** Show the "Expired on" or "Valid until" prefix. */
        showPrefix?: boolean

        lowercase?: boolean
    }>
> = ({ date, showTime, showRelative, showPrefix, lowercase }) => {
    const dateInUTC = new UTCDate(date)

    let text: string | undefined
    if (showPrefix) {
        text = isProductLicenseExpired(dateInUTC) ? 'Expired on ' : 'Valid until '
    }
    return (
        <span>
            {text && lowercase ? text.toLowerCase() : text}
            {showTime ? (
                format(dateInUTC, 'PPpp zzz')
            ) : (
                <span title={format(dateInUTC, 'PPpp zzz')}>{format(dateInUTC, 'yyyy-MM-dd zzz')}</span>
            )}
            {showRelative && ` (${formatRelativeExpirationDate(dateInUTC)})`}
        </span>
    )
}
