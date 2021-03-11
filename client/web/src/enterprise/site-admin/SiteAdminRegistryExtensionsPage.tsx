import AddIcon from 'mdi-react/AddIcon'
import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Observable, Subject, Subscription } from 'rxjs'
import { catchError, map, mapTo, startWith, switchMap, tap } from 'rxjs/operators'
import { gql } from '../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../shared/src/graphql/schema'
import { asError, createAggregateError, ErrorLike, isErrorLike } from '../../../../shared/src/util/errors'
import { queryGraphQL } from '../../backend/graphql'
import { FilteredConnection, FilteredConnectionFilter } from '../../components/FilteredConnection'
import { PageTitle } from '../../components/PageTitle'
import { Timestamp } from '../../components/time/Timestamp'
import { registryExtensionFragment } from '../../extensions/extension/ExtensionArea'
import { eventLogger } from '../../tracking/eventLogger'
import { deleteRegistryExtensionWithConfirmation } from '../extensions/registry/backend'
import { RegistryExtensionSourceBadge } from '../extensions/registry/RegistryExtensionSourceBadge'
import { ErrorAlert } from '../../components/alerts'
import * as H from 'history'

interface RegistryExtensionNodeSiteAdminProps {
    node: GQL.IRegistryExtension
    onDidUpdate: () => void
    history: H.History
}

interface RegistryExtensionNodeSiteAdminState {
    /** Undefined means in progress, null means done or not started. */
    deletionOrError?: null | ErrorLike
}

/** Displays an extension in a row with actions intended for site admins. */
class RegistryExtensionNodeSiteAdminRow extends React.PureComponent<
    RegistryExtensionNodeSiteAdminProps,
    RegistryExtensionNodeSiteAdminState
> {
    public state: RegistryExtensionNodeSiteAdminState = {
        deletionOrError: null,
    }

    private deletes = new Subject<void>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        this.subscriptions.add(
            this.deletes
                .pipe(
                    switchMap(() =>
                        deleteRegistryExtensionWithConfirmation(this.props.node.id).pipe(
                            mapTo(null),
                            catchError(error => [asError(error)]),
                            map(deletionOrError => ({ deletionOrError })),
                            tap(() => {
                                if (this.props.onDidUpdate) {
                                    this.props.onDidUpdate()
                                }
                            }),
                            startWith<Pick<RegistryExtensionNodeSiteAdminState, 'deletionOrError'>>({
                                deletionOrError: undefined,
                            })
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
            <li className="registry-extension-node-row list-group-item d-block py-2">
                <div className="d-flex w-100 justify-content-between">
                    <div className="mr-2">
                        <Link className="font-weight-bold" to={this.props.node.url}>
                            {this.props.node.extensionID}
                        </Link>{' '}
                        <div className="text-muted small">
                            <RegistryExtensionSourceBadge extension={this.props.node} showText={true} />
                            {this.props.node.updatedAt && (
                                <>
                                    , updated <Timestamp date={this.props.node.updatedAt} />{' '}
                                </>
                            )}
                        </div>
                    </div>
                    <div className="d-flex align-items-center">
                        {this.props.node.viewerCanAdminister && (
                            <Link
                                to={`${this.props.node.url}/-/manage`}
                                className="btn btn-secondary btn-sm"
                                title="Manage extension"
                            >
                                Manage
                            </Link>
                        )}
                        {!this.props.node.isLocal && this.props.node.remoteURL && this.props.node.registryName && (
                            <a
                                href={this.props.node.remoteURL}
                                className="btn btn-link text-info btn-sm ml-1"
                                title={`View extension on ${this.props.node.registryName}`}
                            >
                                Visit
                            </a>
                        )}
                        {this.props.node.viewerCanAdminister && (
                            <button
                                type="button"
                                className="btn btn-danger btn-sm ml-1"
                                onClick={this.deleteExtension}
                                disabled={loading}
                                title="Delete extension"
                            >
                                Delete
                            </button>
                        )}
                    </div>
                </div>
                {isErrorLike(this.state.deletionOrError) && (
                    <ErrorAlert className="mt-2" error={this.state.deletionOrError} />
                )}
            </li>
        )
    }

    private deleteExtension = (): void => this.deletes.next()
}

interface Props extends RouteComponentProps<{}> {}

/** Displays all registry extensions on this site. */
export class SiteAdminRegistryExtensionsPage extends React.PureComponent<Props> {
    public static FILTERS: FilteredConnectionFilter[] = [
        {
            id: 'filter',
            label: 'Filter',
            type: 'radio',
            values: [
                {
                    label: 'All',
                    value: 'all',
                    tooltip: 'Show all extensions',
                    args: { remote: true, local: true },
                },
                {
                    label: 'Remote',
                    value: 'remote',
                    tooltip: 'Show only extensions from the remote registry',
                    args: { remote: true, local: false },
                },
                {
                    label: 'Local',
                    value: 'local',
                    tooltip: 'Show only extensions from the local registry',
                    args: { remote: false, local: true },
                },
            ],
        },
    ]

    private updates = new Subject<void>()

    public componentDidMount(): void {
        eventLogger.logViewEvent('SiteAdminRegistryExtensions')
    }

    public render(): JSX.Element | null {
        const nodeProps: Omit<RegistryExtensionNodeSiteAdminProps, 'node'> = {
            onDidUpdate: this.onDidUpdateRegistryExtension,
            history: this.props.history,
        }

        return (
            <div className="registry-extensions-page">
                <PageTitle title="Registry extensions" />
                <div className="d-flex justify-content-between align-items-center mb-3">
                    <h2 className="mb-0">Registry extensions</h2>
                    <div>
                        <Link className="btn btn-link mr-sm-2" to="/extensions">
                            View extensions
                        </Link>
                        <Link className="btn btn-primary" to="/extensions/registry/new">
                            <AddIcon className="icon-inline" /> Publish new extension
                        </Link>
                    </div>
                </div>
                <p>
                    Extensions add features to Sourcegraph and other connected tools (such as editors, code hosts, and
                    code review tools).
                </p>
                <FilteredConnection<GQL.IRegistryExtension, Omit<RegistryExtensionNodeSiteAdminProps, 'node'>>
                    className="list-group list-group-flush registry-extensions-list"
                    listComponent="ul"
                    noun="extension"
                    pluralNoun="extensions"
                    queryConnection={this.queryRegistryExtensions}
                    nodeComponent={RegistryExtensionNodeSiteAdminRow}
                    nodeComponentProps={nodeProps}
                    updates={this.updates}
                    filters={SiteAdminRegistryExtensionsPage.FILTERS}
                    hideSearch={false}
                    noSummaryIfAllNodesVisible={true}
                    history={this.props.history}
                    location={this.props.location}
                />
            </div>
        )
    }

    private queryRegistryExtensions = (args: {
        query?: string
        first?: number
        local?: boolean
        remote?: boolean
    }): Observable<GQL.IRegistryExtensionConnection> =>
        queryGraphQL(
            gql`
                query SiteAdminRegistryExtensions(
                    $first: Int
                    $publisher: ID
                    $query: String
                    $local: Boolean
                    $remote: Boolean
                ) {
                    extensionRegistry {
                        extensions(
                            first: $first
                            publisher: $publisher
                            query: $query
                            local: $local
                            remote: $remote
                        ) {
                            nodes {
                                ...RegistryExtensionFields
                            }
                            totalCount
                            pageInfo {
                                hasNextPage
                            }
                            error
                        }
                    }
                }
                ${registryExtensionFragment}
            `,
            {
                ...args,
                local: args.local === undefined ? true : args.local,
                remote: args.remote === undefined ? true : args.remote,
            }
        ).pipe(
            map(({ data, errors }) => {
                if (!data || !data.extensionRegistry || !data.extensionRegistry.extensions || errors) {
                    throw createAggregateError(errors)
                }
                return data.extensionRegistry.extensions
            })
        )

    private onDidUpdateRegistryExtension = (): void => this.updates.next()
}
