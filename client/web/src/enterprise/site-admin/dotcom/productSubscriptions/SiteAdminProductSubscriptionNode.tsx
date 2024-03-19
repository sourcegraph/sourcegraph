import * as React from 'react'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { LinkOrSpan } from '@sourcegraph/wildcard'

import type { SiteAdminProductSubscriptionFields } from '../../../../graphql-operations'
import { AccountName } from '../../../dotcom/productSubscriptions/AccountName'
import { ProductSubscriptionLabel } from '../../../dotcom/productSubscriptions/ProductSubscriptionLabel'
import { ProductLicenseTags } from '../../../productSubscription/ProductLicenseTags'

export const SiteAdminProductSubscriptionNodeHeader: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <thead>
        <tr>
            <th>ID</th>
            <th>Customer</th>
            <th>Plan</th>
            <th>Expiration</th>
            <th>Tags</th>
        </tr>
    </thead>
)

export interface SiteAdminProductSubscriptionNodeProps {
    node: SiteAdminProductSubscriptionFields
}

/**
 * Displays a product subscription in a connection in the site admin area.
 */
export const SiteAdminProductSubscriptionNode: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminProductSubscriptionNodeProps>
> = ({ node }) => (
    <tr>
        <td className="text-nowrap">
            <LinkOrSpan to={node.urlForSiteAdmin} className="mr-3">
                {node.name}
            </LinkOrSpan>
        </td>
        <td className="w-100">
            <AccountName account={node.account} />
        </td>
        <td className="text-nowrap">
            <ProductSubscriptionLabel productSubscription={node} className="mr-3" />
        </td>
        <td className="text-nowrap">
            {node.activeLicense?.info ? (
                <Timestamp date={node.activeLicense.info.expiresAt} utc={true} />
            ) : (
                <span className="text-muted font-italic">None</span>
            )}
        </td>
        <td className="w-100">
            {node.activeLicense?.info && node.activeLicense.info.tags.length > 0 ? (
                <ProductLicenseTags tags={node.activeLicense.info.tags} />
            ) : (
                <span className="text-muted font-italic">None</span>
            )}
        </td>
    </tr>
)
