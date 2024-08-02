import * as React from 'react'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { Badge, LinkOrSpan } from '@sourcegraph/wildcard'

import type { EnterprisePortalEnvironment } from './enterpriseportal'
import {
    type EnterpriseSubscription,
    EnterpriseSubscriptionCondition_Status,
} from './enterpriseportalgen/subscriptions_pb'

export const SiteAdminProductSubscriptionNodeHeader: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <thead>
        <tr>
            <th>Display name</th>
            <th>Salesforce subscription</th>
            <th>Created</th>
        </tr>
    </thead>
)

export interface SiteAdminProductSubscriptionNodeProps {
    env: EnterprisePortalEnvironment
    node: EnterpriseSubscription
}

/**
 * Displays a product subscription in a connection in the site admin area.
 */
export const SiteAdminProductSubscriptionNode: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminProductSubscriptionNodeProps>
> = ({ env, node }) => {
    const created = node.conditions.find(
        condition => condition.status === EnterpriseSubscriptionCondition_Status.CREATED
    )
    const archived = node.conditions.find(
        condition => condition.status === EnterpriseSubscriptionCondition_Status.ARCHIVED
    )

    return (
        <tr>
            <td>
                <LinkOrSpan to={`/site-admin/dotcom/product/subscriptions/${node.id}?env=${env}`} className="mr-2">
                    {node.displayName}
                </LinkOrSpan>
                {archived && (
                    <Badge variant="danger" small={true}>
                        Archived
                    </Badge>
                )}
            </td>
            <td className="text-nowrap">{node.salesforce?.subscriptionId || ''}</td>
            <td className="text-nowrap">
                {created?.lastTransitionTime && <Timestamp date={created.lastTransitionTime.toDate()} />}
            </td>
        </tr>
    )
}
