import React, { useEffect, useMemo } from 'react'

import { type Observable, Subject } from 'rxjs'
import { map } from 'rxjs/operators'

import { createAggregateError } from '@sourcegraph/common'
import { gql } from '@sourcegraph/http-client'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { H2 } from '@sourcegraph/wildcard'

import { queryGraphQL } from '../../../../backend/graphql'
import { FilteredConnection } from '../../../../components/FilteredConnection'
import { PageTitle } from '../../../../components/PageTitle'
import type { CustomerFields, CustomersResult, CustomersVariables } from '../../../../graphql-operations'
import { userURL } from '../../../../user'
import { AccountName } from '../../../dotcom/productSubscriptions/AccountName'

const siteAdminCustomerFragment = gql`
    fragment CustomerFields on User {
        id
        username
        displayName
    }
`

interface SiteAdminCustomerNodeProps {
    node: CustomerFields
}

/**
 * Displays a customer in a connection in the site admin area.
 */
const SiteAdminCustomerNode: React.FunctionComponent<React.PropsWithChildren<SiteAdminCustomerNodeProps>> = ({
    node,
}) => (
    <li className="list-group-item py-2">
        <div className="d-flex align-items-center justify-content-between">
            <span className="mr-3">
                <AccountName account={node} link={`${userURL(node.username)}/subscriptions`} />
            </span>
        </div>
    </li>
)

interface Props extends TelemetryV2Props {}

/**
 * Displays a list of customers associated with user accounts on Sourcegraph.com.
 */
export const SiteAdminProductCustomersPage: React.FunctionComponent<React.PropsWithChildren<Props>> = props => {
    useEffect(() => props.telemetryRecorder.recordEvent('admin.customers', 'view'), [props.telemetryRecorder])

    const updates = useMemo(() => new Subject<void>(), [])
    const nodeProps: Pick<SiteAdminCustomerNodeProps, Exclude<keyof SiteAdminCustomerNodeProps, 'node'>> = {}

    return (
        <div className="site-admin-customers-page">
            <PageTitle title="Customers" />
            <div className="d-flex justify-content-between align-items-center mb-1">
                <H2 className="mb-0">Customers</H2>
            </div>
            <FilteredConnection<
                CustomerFields,
                Pick<SiteAdminCustomerNodeProps, Exclude<keyof SiteAdminCustomerNodeProps, 'node'>>
            >
                className="list-group list-group-flush mt-3"
                noun="customer"
                pluralNoun="customers"
                queryConnection={queryCustomers}
                nodeComponent={SiteAdminCustomerNode}
                nodeComponentProps={nodeProps}
                noSummaryIfAllNodesVisible={true}
                updates={updates}
            />
        </div>
    )
}

function queryCustomers(args: Partial<CustomersVariables>): Observable<CustomersResult['users']> {
    return queryGraphQL<CustomersResult>(
        gql`
            query Customers($first: Int, $query: String) {
                users(first: $first, query: $query) {
                    nodes {
                        ...CustomerFields
                    }
                    totalCount
                    pageInfo {
                        hasNextPage
                    }
                }
            }
            ${siteAdminCustomerFragment}
        `,
        {
            first: args.first,
            query: args.query,
        }
    ).pipe(
        map(({ data, errors }) => {
            if (!data?.users || (errors && errors.length > 0)) {
                throw createAggregateError(errors)
            }
            return data.users
        })
    )
}
