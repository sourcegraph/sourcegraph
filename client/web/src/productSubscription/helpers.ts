import { parseISO, formatDistanceStrict, isAfter } from 'date-fns'

import { numberWithCommas, pluralize } from '@sourcegraph/common'

/**
 * Returns "N users" (properly pluralized and with commas added to N as needed).
 */
export function formatUserCount(userCount: number, hyphenate?: boolean): string {
    if (hyphenate) {
        return `${numberWithCommas(userCount)}-user`
    }
    return `${numberWithCommas(userCount)} ${pluralize('user', userCount)}`
}

/**
 * Reports whether {@link expiresAt} is in the past.
 */
export function isProductLicenseExpired(expiresAt: string | number | Date): boolean {
    return !isAfter(typeof expiresAt === 'string' ? parseISO(expiresAt) : expiresAt, Date.now())
}

/**
 * Returns "T remaining" or "T ago" for an expiration date.
 */
export function formatRelativeExpirationDate(date: string | number | Date): string {
    return `${formatDistanceStrict(typeof date === 'string' ? parseISO(date) : date, Date.now())} ${
        isProductLicenseExpired(date) ? 'ago' : 'remaining'
    }`
}

/**
 * Returns a mailto:sales@sourcegraph.com URL with an optional subject.
 */
export function mailtoSales(args: { subject?: string }): string {
    return `mailto:sales@sourcegraph.com?subject=${encodeURIComponent(args.subject || '')}`
}
