import * as React from 'react'

import { type Observable, Subject, Subscription } from 'rxjs'
import { catchError, filter, map, startWith, switchMap, tap } from 'rxjs/operators'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { asError, type ErrorLike, isErrorLike, logger } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { Badge, Button, Link, AnchorLink, ErrorAlert } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../../backend/graphql'
import type {
    DeleteExternalAccountResult,
    DeleteExternalAccountVariables,
    ExternalAccountFields,
    Scalars,
} from '../../../graphql-operations'
import { userURL } from '../../../user'

export const externalAccountFragment = gql`
    fragment ExternalAccountFields on ExternalAccount {
        id
        user {
            id
            username
        }
        serviceType
        serviceID
        clientID
        accountID
        createdAt
        updatedAt
        refreshURL
        accountData
    }
`

export const externalAccountsConnectionFragment = gql`
    fragment ExternalAccountsConnectionFields on ExternalAccountConnection {
        nodes {
            ...ExternalAccountFields
        }
        totalCount
        pageInfo {
            hasNextPage
        }
    }

    ${externalAccountFragment}
`

function deleteExternalAccount(externalAccount: Scalars['ID']): Observable<void> {
    return requestGraphQL<DeleteExternalAccountResult, DeleteExternalAccountVariables>(
        gql`
            mutation DeleteExternalAccount($externalAccount: ID!) {
                deleteExternalAccount(externalAccount: $externalAccount) {
                    alwaysNil
                }
            }
        `,
        { externalAccount }
    ).pipe(
        map(dataOrThrowErrors),
        map(() => undefined)
    )
}

export interface ExternalAccountNodeProps {
    node: ExternalAccountFields

    showUser: boolean

    onDidUpdate: () => void
}

interface ExternalAccountNodeState {
    /** Undefined means in progress, null means done or not started. */
    deletionOrError?: null | ErrorLike

    showData: boolean
}

export class ExternalAccountNode extends React.PureComponent<ExternalAccountNodeProps, ExternalAccountNodeState> {
    public state: ExternalAccountNodeState = {
        deletionOrError: null,
        showData: false,
    }

    private deletes = new Subject<void>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        this.subscriptions.add(
            this.deletes
                .pipe(
                    filter(() => window.confirm('Really delete the association with this external account?')),
                    switchMap(() =>
                        deleteExternalAccount(this.props.node.id).pipe(
                            map(() => null),
                            catchError(error => [asError(error)]),
                            map(deletionOrError => ({ deletionOrError })),
                            tap(() => {
                                if (this.props.onDidUpdate) {
                                    this.props.onDidUpdate()
                                }
                            }),
                            startWith<Pick<ExternalAccountNodeState, 'deletionOrError'>>({ deletionOrError: undefined })
                        )
                    )
                )
                .subscribe({
                    next: stateUpdate => this.setState(stateUpdate),
                    error: error => logger.error(error),
                })
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        const loading = this.state.deletionOrError === undefined
        return (
            <li className="list-group-item py-2">
                <div className="d-flex align-items-center justify-content-between">
                    <div className="mr-2 text-truncate">
                        {this.props.showUser && (
                            <>
                                <strong>
                                    <Link to={userURL(this.props.node.user.username)}>
                                        {this.props.node.user.username}
                                    </Link>
                                </strong>{' '}
                                &mdash;{' '}
                            </>
                        )}
                        <Badge variant="secondary">{this.props.node.serviceType}</Badge> {this.props.node.accountID}
                        {(this.props.node.serviceID || this.props.node.clientID) && (
                            <small className="text-muted">
                                <br />
                                {this.props.node.serviceID}
                                {this.state.showData && this.props.node.clientID && (
                                    <> &mdash; {this.props.node.clientID}</>
                                )}
                            </small>
                        )}
                        <br />
                        <small className="text-muted">
                            Updated <Timestamp date={this.props.node.updatedAt} />
                        </small>
                    </div>
                    <div className="text-nowrap">
                        {/*
                         * Issue: This JSX tag's 'children' prop expects a single child of type 'ReactNode', but multiple children were provided
                         * It seems that v18 requires explicit boolean value
                         */}
                        {!!this.props.node.accountData && (
                            <Button onClick={this.toggleShowData} variant="secondary">
                                {this.state.showData ? 'Hide' : 'Show'} data
                            </Button>
                        )}{' '}
                        {!!this.props.node.refreshURL && (
                            <Button to={this.props.node.refreshURL} variant="secondary" as={AnchorLink}>
                                Refresh
                            </Button>
                        )}{' '}
                        <Button onClick={this.deleteExternalAccount} disabled={loading} variant="danger">
                            Delete
                        </Button>
                        {isErrorLike(this.state.deletionOrError) && (
                            <ErrorAlert className="mt-2" error={this.state.deletionOrError} />
                        )}
                    </div>
                </div>
                {this.state.showData && (
                    <pre className="p-2 mt-2 mb-4">
                        <small>{JSON.stringify(this.props.node.accountData, null, 2)}</small>
                    </pre>
                )}
            </li>
        )
    }

    private deleteExternalAccount = (): void => this.deletes.next()

    private toggleShowData = (): void => this.setState(previous => ({ showData: !previous.showData }))
}
