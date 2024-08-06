import * as React from 'react'

import { Badge, LinkOrSpan } from '@sourcegraph/wildcard'

import type { EnterprisePortalEnvironment } from './enterpriseportal'
import {
    type EnterpriseSubscription,
    EnterpriseSubscriptionCondition_Status,
    EnterpriseSubscriptionInstanceType,
} from './enterpriseportalgen/subscriptions_pb'

export const SiteAdminProductSubscriptionNodeHeader: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <thead>
        <tr>
            <th>Display name</th>
            <th>Salesforce subscription</th>
            <th>Instance type</th>
            <th>Instance domain</th>
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
    const archived = node.conditions.find(
        condition => condition.status === EnterpriseSubscriptionCondition_Status.ARCHIVED
    )

    return (
        <tr>
            <td>
                {archived && (
                    <Badge variant="danger" small={true} className="mr-2">
                        Archived
                    </Badge>
                )}
                <LinkOrSpan to={`/site-admin/dotcom/product/subscriptions/${node.id}?env=${env}`}>
                    {node.displayName}
                </LinkOrSpan>
            </td>
            <td className="text-nowrap">
                {node?.salesforce?.subscriptionId ? (
                    <span className="text-monospace">{node?.salesforce?.subscriptionId}</span>
                ) : (
                    <span className="text-muted">Not set</span>
                )}
            </td>
            <td className="text-nowrap">
                {node?.instanceType ? (
                    <span className="text-monospace">{EnterpriseSubscriptionInstanceType[node?.instanceType]}</span>
                ) : (
                    <span className="text-muted">Not set</span>
                )}
            </td>
            <td className="text-nowrap">
                {node?.instanceDomain ? <>{node?.instanceDomain}</> : <span className="text-muted">Not set</span>}
            </td>
        </tr>
    )
}
