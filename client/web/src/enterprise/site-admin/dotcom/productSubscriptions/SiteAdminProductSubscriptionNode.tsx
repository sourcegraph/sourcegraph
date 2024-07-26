import * as React from 'react'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { LinkOrSpan } from '@sourcegraph/wildcard'

import {
    type EnterpriseSubscription,
    EnterpriseSubscriptionCondition_Status,
} from './enterpriseportalgen/subscriptions_pb'

export const SiteAdminProductSubscriptionNodeHeader: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <thead>
        <tr>
            <th>ID</th>
            <th>Display name</th>
            <th>Salesforce subscription ID</th>
            <th>Created at</th>
        </tr>
    </thead>
)

export interface SiteAdminProductSubscriptionNodeProps {
    node: EnterpriseSubscription
}

/**
 * Displays a product subscription in a connection in the site admin area.
 */
export const SiteAdminProductSubscriptionNode: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminProductSubscriptionNodeProps>
> = ({ node }) => {
    const created = node.conditions.find(
        condition => condition.status === EnterpriseSubscriptionCondition_Status.CREATED
    )

    return (
        <tr>
            <td>
                <LinkOrSpan to={`/site-admin/dotcom/product/subscriptions/${node.id}`} className="mr-3">
                    {node.id}
                </LinkOrSpan>
            </td>
            <td className="text-nowrap">{node.displayName}</td>
            <td className="text-nowrap">{node.salesforce?.subscriptionId || ''}</td>
            <td className="text-nowrap">
                {created?.lastTransitionTime && <Timestamp date={created.lastTransitionTime.toDate()} />}
            </td>
        </tr>
    )
}
