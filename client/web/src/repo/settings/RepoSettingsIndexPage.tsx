import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import CheckCircleIcon from 'mdi-react/CheckCircleIcon'
import prettyBytes from 'pretty-bytes'
import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Observable, Subject, Subscription } from 'rxjs'
import { map, switchMap, tap } from 'rxjs/operators'
import { LinkOrSpan } from '../../../../shared/src/components/LinkOrSpan'
import { gql } from '../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../shared/src/graphql/schema'
import { createAggregateError } from '../../../../shared/src/util/errors'
import { pluralize } from '../../../../shared/src/util/strings'
import { queryGraphQL } from '../../backend/graphql'
import { PageTitle } from '../../components/PageTitle'
import { Timestamp } from '../../components/time/Timestamp'
import { eventLogger } from '../../tracking/eventLogger'
import { ErrorAlert } from '../../components/alerts'
import { Scalars, SettingsAreaRepositoryFields } from '../../graphql-operations'

/**
 * Fetches a repository's text search index information.
 */
function fetchRepositoryTextSearchIndex(id: Scalars['ID']): Observable<GQL.IRepositoryTextSearchIndex | null> {
    return queryGraphQL(
        gql`
            query RepositoryTextSearchIndex($id: ID!) {
                node(id: $id) {
                    ... on Repository {
                        textSearchIndex {
                            status {
                                updatedAt
                                contentByteSize
                                contentFilesCount
                                indexByteSize
                                indexShardsCount
                                newLinesCount
                                defaultBranchNewLinesCount
                                otherBranchesNewLinesCount
                            }
                            refs {
                                ref {
                                    displayName
                                    url
                                }
                                indexed
                                current
                                indexedCommit {
                                    oid
                                    abbreviatedOID
                                    commit {
                                        url
                                    }
                                }
                            }
                        }
                    }
                }
            }
        `,
        { id }
    ).pipe(
        map(({ data, errors }) => {
            if (!data || !data.node || errors) {
                throw createAggregateError(errors)
            }
            return (data.node as GQL.IRepository).textSearchIndex
        })
    )
}

const TextSearchIndexedReference: React.FunctionComponent<{
    repo: SettingsAreaRepositoryFields
    indexedRef: GQL.IRepositoryTextSearchIndexedRef
}> = ({ repo, indexedRef }) => {
    let Icon: React.ComponentType<{ className?: string }>
    let iconClassName: string
    if (indexedRef.indexed && indexedRef.current) {
        Icon = CheckCircleIcon
        iconClassName = 'current'
    } else {
        Icon = LoadingSpinner
        iconClassName = 'stale'
    }

    return (
        <li className="repo-settings-index-page__ref">
            <Icon
                className={`icon-inline repo-settings-index-page__ref-icon repo-settings-index-page__ref-icon--${iconClassName}`}
            />
            <LinkOrSpan to={indexedRef.ref.url}>
                <strong>
                    <code>{indexedRef.ref.displayName}</code>
                </strong>
            </LinkOrSpan>{' '}
            {indexedRef.indexed ? (
                <span>
                    &nbsp;&mdash; indexed at{' '}
                    <code>
                        <LinkOrSpan
                            to={indexedRef.indexedCommit?.commit ? indexedRef.indexedCommit.commit.url : repo.url}
                        >
                            {indexedRef.indexedCommit!.abbreviatedOID}
                        </LinkOrSpan>
                    </code>{' '}
                    {indexedRef.current ? '(up to date)' : '(index update in progress)'}
                </span>
            ) : (
                <span>&nbsp;&mdash; initial indexing in progress</span>
            )}
        </li>
    )
}

interface Props extends RouteComponentProps<{}> {
    repo: SettingsAreaRepositoryFields
}

interface State {
    textSearchIndex?: GQL.IRepositoryTextSearchIndex | null
    loading: boolean
    error?: Error
}

/**
 * The repository settings index page.
 */
export class RepoSettingsIndexPage extends React.PureComponent<Props, State> {
    public state: State = { loading: true }

    private updates = new Subject<void>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        eventLogger.logViewEvent('RepoSettingsIndex')

        this.subscriptions.add(
            this.updates
                .pipe(
                    tap(() => this.setState({ loading: true })),
                    switchMap(() => fetchRepositoryTextSearchIndex(this.props.repo.id))
                )
                .subscribe(
                    textSearchIndex => this.setState({ textSearchIndex, loading: false }),
                    error => this.setState({ error, loading: false })
                )
        )
        this.updates.next()
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <div className="repo-settings-index-page">
                <PageTitle title="Index settings" />
                <h2>Indexing</h2>
                {this.state.loading && <LoadingSpinner className="icon-inline" />}
                {this.state.error && (
                    <ErrorAlert prefix="Error getting repository index status" error={this.state.error} />
                )}
                {!this.state.error &&
                    !this.state.loading &&
                    (this.state.textSearchIndex ? (
                        <>
                            <p>Index branches to enhance search performance at scale.</p>
                            {this.state.textSearchIndex.refs && (
                                <ul className="repo-settings-index-page__refs">
                                    {this.state.textSearchIndex.refs.map((reference, index) => (
                                        <TextSearchIndexedReference
                                            key={index}
                                            repo={this.props.repo}
                                            indexedRef={reference}
                                        />
                                    ))}
                                </ul>
                            )}
                            {this.state.textSearchIndex.status && (
                                <>
                                    <h3>Statistics</h3>
                                    <table className="table repo-settings-index-page__stats">
                                        <tbody>
                                            <tr>
                                                <th>Last updated</th>
                                                <td>
                                                    <Timestamp date={this.state.textSearchIndex.status.updatedAt} />
                                                </td>
                                            </tr>
                                            <tr>
                                                <th>Content size</th>
                                                <td>
                                                    {prettyBytes(this.state.textSearchIndex.status.contentByteSize)} (
                                                    {this.state.textSearchIndex.status.contentFilesCount}{' '}
                                                    {pluralize(
                                                        'file',
                                                        this.state.textSearchIndex.status.contentFilesCount
                                                    )}
                                                    )
                                                </td>
                                            </tr>
                                            <tr>
                                                <th>Index size</th>
                                                <td>
                                                    {prettyBytes(this.state.textSearchIndex.status.indexByteSize)} (
                                                    {this.state.textSearchIndex.status.indexShardsCount}{' '}
                                                    {pluralize(
                                                        'shard',
                                                        this.state.textSearchIndex.status.indexShardsCount
                                                    )}
                                                    )
                                                </td>
                                            </tr>
                                            <tr>
                                                <th>New lines count</th>
                                                <td>
                                                    {this.state.textSearchIndex.status.newLinesCount.toLocaleString()}{' '}
                                                    (default branch:{' '}
                                                    {this.state.textSearchIndex.status.defaultBranchNewLinesCount.toLocaleString()}
                                                    ) (other branches:{' '}
                                                    {this.state.textSearchIndex.status.otherBranchesNewLinesCount.toLocaleString()}
                                                    )
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </>
                            )}
                        </>
                    ) : (
                        <div className="alert alert-info">
                            This Sourcegraph site has not enabled indexed search. See{' '}
                            <Link to="/help/admin/search">search documentation</Link> for information on how to enable
                            it.
                        </div>
                    ))}
            </div>
        )
    }
}
