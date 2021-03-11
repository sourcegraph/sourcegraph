import DeleteIcon from 'mdi-react/DeleteIcon'
import MessageTextOutlineIcon from 'mdi-react/MessageTextOutlineIcon'
import PlusIcon from 'mdi-react/PlusIcon'
import SettingsIcon from 'mdi-react/SettingsIcon'
import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Link } from 'react-router-dom'
import { Subject, Subscription } from 'rxjs'
import { catchError, map, mapTo, startWith, switchMap } from 'rxjs/operators'
import * as GQL from '../../../shared/src/graphql/schema'
import { asError, ErrorLike, isErrorLike } from '../../../shared/src/util/errors'
import { buildSearchURLQuery } from '../../../shared/src/util/url'
import { NamespaceProps } from '../namespaces'
import { deleteSavedSearch, fetchSavedSearches } from '../search/backend'
import { PatternTypeProps } from '../search'
import { ErrorAlert } from '../components/alerts'
import { eventLogger } from '../tracking/eventLogger'

interface NodeProps extends RouteComponentProps, Omit<PatternTypeProps, 'setPatternType'> {
    savedSearch: GQL.ISavedSearch
    onDelete: () => void
}

interface NodeState {
    isDeleting: boolean
}

class SavedSearchNode extends React.PureComponent<NodeProps, NodeState> {
    constructor(props: NodeProps) {
        super(props)
        this.state = { isDeleting: false }
    }

    private subscriptions = new Subscription()
    private delete = new Subject<GQL.ISavedSearch>()

    public componentDidMount(): void {
        this.subscriptions.add(
            this.delete
                .pipe(
                    switchMap(search =>
                        deleteSavedSearch(search.id).pipe(
                            mapTo(undefined),
                            catchError(error => {
                                console.error(error)
                                return []
                            })
                        )
                    )
                )
                .subscribe(() => {
                    eventLogger.log('SavedSearchDeleted')
                    this.setState({ isDeleting: false })
                    this.props.onDelete()
                })
        )
    }
    public render(): JSX.Element | null {
        return (
            <div className="saved-search-list-page__row list-group-item test-saved-search-list-page-row">
                <div className="d-flex">
                    <MessageTextOutlineIcon className="saved-search-list-page__row--icon icon-inline" />
                    <Link
                        to={
                            '/search?' +
                            buildSearchURLQuery(this.props.savedSearch.query, this.props.patternType, false)
                        }
                    >
                        <div className="test-saved-search-list-page-row-title">
                            {this.props.savedSearch.description}
                        </div>
                    </Link>
                </div>
                <div>
                    <Link
                        className="btn btn-secondary btn-sm test-edit-saved-search-button"
                        to={`${this.props.match.path}/${this.props.savedSearch.id}`}
                        data-tooltip="Saved search settings"
                    >
                        <SettingsIcon className="icon-inline" /> Settings
                    </Link>{' '}
                    <button
                        type="button"
                        className="btn btn-sm btn-danger test-delete-saved-search-button"
                        onClick={this.onDelete}
                        disabled={this.state.isDeleting}
                        data-tooltip="Delete saved search"
                    >
                        <DeleteIcon className="icon-inline" />
                    </button>
                </div>
            </div>
        )
    }

    private onDelete = (): void => {
        if (!window.confirm(`Delete the external service ${this.props.savedSearch.description}?`)) {
            return
        }
        this.setState({ isDeleting: true })
        this.delete.next(this.props.savedSearch)
    }
}

interface State {
    savedSearchesOrError?: GQL.ISavedSearch[] | ErrorLike
}

interface Props extends RouteComponentProps<{}>, NamespaceProps, Omit<PatternTypeProps, 'setPatternType'> {}

export class SavedSearchListPage extends React.Component<Props, State> {
    public subscriptions = new Subscription()
    private refreshRequests = new Subject<void>()

    public state: State = {}

    public componentDidMount(): void {
        this.subscriptions.add(
            this.refreshRequests
                .pipe(
                    startWith(undefined),
                    switchMap(() =>
                        fetchSavedSearches().pipe(
                            catchError(error => {
                                console.error(error)
                                return [asError(error)]
                            })
                        )
                    ),
                    map(savedSearchesOrError => ({ savedSearchesOrError }))
                )
                .subscribe(newState => this.setState(newState as State))
        )
        eventLogger.logViewEvent('SavedSearchListPage')
    }

    public render(): JSX.Element | null {
        return (
            <div className="saved-search-list-page">
                <div className="saved-search-list-page__title">
                    <div>
                        <h2>Saved searches</h2>
                        <div>Manage notifications and alerts for specific search queries</div>
                    </div>
                    <div>
                        <Link
                            to={`${this.props.match.path}/add`}
                            className="btn btn-primary test-add-saved-search-button"
                        >
                            <PlusIcon className="icon-inline" /> Add saved search
                        </Link>
                    </div>
                </div>
                {this.state.savedSearchesOrError && isErrorLike(this.state.savedSearchesOrError) && (
                    <ErrorAlert className="mb-3" error={this.state.savedSearchesOrError} />
                )}
                <div className="list-group list-group-flush">
                    {this.state.savedSearchesOrError &&
                        !isErrorLike(this.state.savedSearchesOrError) &&
                        this.state.savedSearchesOrError.length > 0 &&
                        this.state.savedSearchesOrError
                            .filter(search => this.props.namespace.id === search.namespace.id)
                            .map(search => (
                                <SavedSearchNode
                                    key={search.id}
                                    {...this.props}
                                    savedSearch={search}
                                    onDelete={this.onDelete}
                                />
                            ))}
                </div>
            </div>
        )
    }

    private onDelete = (): void => {
        this.refreshRequests.next()
    }
}
