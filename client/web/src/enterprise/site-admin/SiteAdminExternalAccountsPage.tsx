import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Observable, Subject, Subscription } from 'rxjs'
import { map } from 'rxjs/operators'
import { Scalars } from '../../../../shared/src/graphql-operations'
import { gql } from '../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../shared/src/graphql/schema'
import { createAggregateError } from '../../../../shared/src/util/errors'
import { queryGraphQL } from '../../backend/graphql'
import { FilteredConnection } from '../../components/FilteredConnection'
import { PageTitle } from '../../components/PageTitle'
import { eventLogger } from '../../tracking/eventLogger'
import {
    externalAccountFragment,
    ExternalAccountNode,
    ExternalAccountNodeProps,
} from '../user/settings/ExternalAccountNode'

interface Props extends RouteComponentProps<{}> {}

interface FilterParameters {
    user?: Scalars['ID']
    serviceType?: string
    serviceID?: string
    clientID?: string
}

/**
 * Displays the external accounts (from authentication providers) associated with the user's account.
 */
export class SiteAdminExternalAccountsPage extends React.Component<Props> {
    private subscriptions = new Subscription()
    private externalAccountUpdates = new Subject<void>()

    public componentDidMount(): void {
        eventLogger.logViewEvent('SiteAdminExternalAccounts')
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        const nodeProps: Omit<ExternalAccountNodeProps, 'node'> = {
            onDidUpdate: this.onDidUpdateExternalAccount,
            showUser: true,
        }

        return (
            <div className="user-settings-external-accounts-page">
                <PageTitle title="External accounts" />
                <div className="d-flex justify-content-between align-items-center mb-3">
                    <h2 className="mb-0">External user accounts</h2>
                    <Link to="/site-admin/auth/providers" className="btn btn-secondary">
                        View auth providers
                    </Link>
                </div>
                <p>
                    An external account (on an <Link to="/site-admin/auth/providers">authentication provider</Link>) is
                    linked to a Sourcegraph user when it's used to sign into Sourcegraph.
                </p>
                <FilteredConnection<GQL.IExternalAccount, Omit<ExternalAccountNodeProps, 'node'>>
                    className="list-group list-group-flush mt-3"
                    noun="external user account"
                    pluralNoun="external user accounts"
                    queryConnection={this.queryExternalAccounts}
                    nodeComponent={ExternalAccountNode}
                    nodeComponentProps={nodeProps}
                    updates={this.externalAccountUpdates}
                    hideSearch={true}
                    history={this.props.history}
                    location={this.props.location}
                />
            </div>
        )
    }

    private queryExternalAccounts = (
        args: {
            first?: number
        } & FilterParameters
    ): Observable<GQL.IExternalAccountConnection> =>
        queryGraphQL(
            gql`
                query ExternalAccounts(
                    $first: Int
                    $user: ID
                    $serviceType: String
                    $serviceID: String
                    $clientID: String
                ) {
                    site {
                        externalAccounts(
                            first: $first
                            user: $user
                            serviceType: $serviceType
                            serviceID: $serviceID
                            clientID: $clientID
                        ) {
                            nodes {
                                ...ExternalAccountFields
                            }
                            totalCount
                            pageInfo {
                                hasNextPage
                            }
                        }
                    }
                }
                ${externalAccountFragment}
            `,
            args
        ).pipe(
            map(({ data, errors }) => {
                if (!data || !data.site || !data.site.externalAccounts) {
                    throw createAggregateError(errors)
                }
                return data.site.externalAccounts
            })
        )

    private onDidUpdateExternalAccount = (): void => this.externalAccountUpdates.next()
}
