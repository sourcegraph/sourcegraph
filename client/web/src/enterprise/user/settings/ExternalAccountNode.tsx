import * as React from 'react'
import { Link } from 'react-router-dom'
import { Observable, Subject, Subscription } from 'rxjs'
import { catchError, filter, map, mapTo, startWith, switchMap, tap } from 'rxjs/operators'
import { dataOrThrowErrors, gql } from '../../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../../shared/src/graphql/schema'
import { asError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { requestGraphQL } from '../../../backend/graphql'
import { Timestamp } from '../../../components/time/Timestamp'
import { userURL } from '../../../user'
import { ErrorAlert } from '../../../components/alerts'
import { DeleteExternalAccountResult, DeleteExternalAccountVariables, Scalars } from '../../../graphql-operations'

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
    ).pipe(map(dataOrThrowErrors), mapTo(undefined))
}

export interface ExternalAccountNodeProps {
    node: GQL.IExternalAccount

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
                            mapTo(null),
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
                .subscribe(
                    stateUpdate => this.setState(stateUpdate),
                    error => console.error(error)
                )
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
                        <span className="badge badge-secondary">{this.props.node.serviceType}</span>{' '}
                        {this.props.node.accountID}
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
                        {this.props.node.accountData && (
                            <button type="button" className="btn btn-secondary" onClick={this.toggleShowData}>
                                {this.state.showData ? 'Hide' : 'Show'} data
                            </button>
                        )}{' '}
                        {this.props.node.refreshURL && (
                            <a className="btn btn-secondary" href={this.props.node.refreshURL}>
                                Refresh
                            </a>
                        )}{' '}
                        <button
                            type="button"
                            className="btn btn-danger"
                            onClick={this.deleteExternalAccount}
                            disabled={loading}
                        >
                            Delete
                        </button>
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
