import React from 'react'

import { formatUserCount } from '../../../productSubscription/helpers'

/**
 * Displays a text label with the product name (e.g., "Sourcegraph Enterprise") and user count for the
 * subscription.
 */
export const ProductSubscriptionLabel: React.FunctionComponent<
    React.PropsWithChildren<{
        productName?: string
        userCount?: bigint
        className?: string
    }>
> = ({ productName, userCount, className = '' }) => (
    <span className={className}>
        {productName && userCount ? (
            <>{productSubscriptionLabel(productName, userCount)}</>
        ) : (
            <span className="text-muted font-italic">No plan selected</span>
        )}
    </span>
)

export function productSubscriptionLabel(productName?: string, userCount?: bigint): string {
    return `${productName} (${formatUserCount(Number(userCount))})`
}
