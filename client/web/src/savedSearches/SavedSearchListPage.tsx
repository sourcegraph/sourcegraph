import * as React from 'react'

import { mdiMessageTextOutline, mdiCog, mdiDelete, mdiPlus } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'
import { useLocation } from 'react-router-dom'
import { Subject, Subscription } from 'rxjs'
import { catchError, map, switchMap } from 'rxjs/operators'
import { useCallbackRef } from 'use-callback-ref'

import { logger } from '@sourcegraph/common'
import type { SearchPatternTypeProps } from '@sourcegraph/shared/src/search'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import {
    Container,
    PageHeader,
    LoadingSpinner,
    Button,
    Link,
    Icon,
    Tooltip,
    ErrorAlert,
    PageSwitcher,
} from '@sourcegraph/wildcard'

import { usePageSwitcherPagination } from '../components/FilteredConnection/hooks/usePageSwitcherPagination'
import { PageTitle } from '../components/PageTitle'
import type { SavedSearchFields, SavedSearchesResult, SavedSearchesVariables } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { deleteSavedSearch, savedSearchesQuery } from '../search/backend'
import { useNavbarQueryState } from '../stores'
import { eventLogger } from '../tracking/eventLogger'

import styles from './SavedSearchListPage.module.scss'

interface NodeProps extends SearchPatternTypeProps {
    savedSearch: SavedSearchFields
    onDelete: () => void
    linkRef: React.MutableRefObject<HTMLAnchorElement | null> | null
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
    private delete = new Subject<SavedSearchFields>()

    public componentDidMount(): void {
        this.subscriptions.add(
            this.delete
                .pipe(
                    switchMap(search =>
                        deleteSavedSearch(search.id).pipe(
                            map(() => undefined),
                            catchError(error => {
                                logger.error(error)
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
            <div className={classNames(styles.row, 'list-group-item test-saved-search-list-page-row')}>
                <div className="d-flex">
                    <Icon className={styles.rowIcon} aria-hidden={true} svgPath={mdiMessageTextOutline} />
                    <Link
                        to={
                            '/search?' +
                            buildSearchURLQuery(this.props.savedSearch.query, this.props.patternType, false)
                        }
                        ref={this.props.linkRef}
                    >
                        <div className="test-saved-search-list-page-row-title">
                            <VisuallyHidden>Run saved search: </VisuallyHidden>
                            {this.props.savedSearch.description}
                        </div>
                    </Link>
                </div>
                <div>
                    <Tooltip content="Saved search settings">
                        <Button
                            className="test-edit-saved-search-button"
                            to={this.props.savedSearch.id}
                            variant="secondary"
                            size="sm"
                            as={Link}
                        >
                            <Icon aria-hidden={true} svgPath={mdiCog} /> Settings
                        </Button>
                    </Tooltip>{' '}
                    <Tooltip content="Delete saved search">
                        <Button
                            aria-label="Delete"
                            className="test-delete-saved-search-button"
                            onClick={this.onDelete}
                            disabled={this.state.isDeleting}
                            variant="danger"
                            size="sm"
                        >
                            <Icon aria-hidden={true} svgPath={mdiDelete} />
                        </Button>
                    </Tooltip>
                </div>
                {this.state.isDeleting && (
                    <VisuallyHidden aria-live="polite">{`Deleted saved search: ${this.props.savedSearch.description}`}</VisuallyHidden>
                )}
            </div>
        )
    }

    private onDelete = (): void => {
        if (!window.confirm(`Delete the saved search ${this.props.savedSearch.description}?`)) {
            return
        }
        this.setState({ isDeleting: true })
        this.delete.next(this.props.savedSearch)
    }
}

interface Props extends NamespaceProps {}

export const SavedSearchListPage: React.FunctionComponent<Props> = props => {
    React.useEffect(() => {
        eventLogger.logViewEvent('SavedSearchListPage')
    }, [])

    const { connection, loading, error, refetch, ...paginationProps } = usePageSwitcherPagination<
        SavedSearchesResult,
        SavedSearchesVariables,
        SavedSearchFields
    >({
        query: savedSearchesQuery,
        variables: { namespace: props.namespace.id },
        getConnection: ({ data }) => data?.savedSearches || undefined,
    })

    return (
        <div className={styles.savedSearchListPage} data-testid="saved-searches-list-page">
            <PageHeader
                actions={
                    <Button to="add" className="test-add-saved-search-button" variant="primary" as={Link}>
                        <Icon aria-hidden={true} svgPath={mdiPlus} /> Add saved search
                    </Button>
                }
                className="mb-3"
            >
                <PageTitle title="Saved searches" />
                <PageHeader.Heading as="h3" styleAs="h2">
                    <PageHeader.Breadcrumb>Saved searches</PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>
            <SavedSearchListPageContent
                {...props}
                onDelete={refetch}
                savedSearches={connection?.nodes || []}
                error={error}
                loading={loading}
            />
            <PageSwitcher {...paginationProps} className="mt-4" totalCount={connection?.totalCount || 0} />
        </div>
    )
}

interface SavedSearchListPageContentProps extends Props {
    onDelete: () => void
    savedSearches: SavedSearchFields[]
    error: unknown
    loading: boolean
}

const SavedSearchListPageContent: React.FunctionComponent<React.PropsWithChildren<SavedSearchListPageContentProps>> = ({
    namespace,
    savedSearches,
    error,
    loading,
    ...props
}) => {
    const location = useLocation()
    const searchPatternType = useNavbarQueryState(state => state.searchPatternType)
    const callbackReference = useCallbackRef<HTMLAnchorElement>(null, ref => ref?.focus())

    if (loading) {
        return <LoadingSpinner />
    }

    if (error) {
        return <ErrorAlert className="mb-3" error={error} />
    }

    if (savedSearches.length === 0) {
        return <Container className="text-center text-muted">You haven't created a saved search yet.</Container>
    }

    return (
        <Container>
            <div className="list-group list-group-flush">
                {savedSearches.map(search => (
                    <SavedSearchNode
                        key={search.id}
                        linkRef={location.state?.description === search.description ? callbackReference : null}
                        {...props}
                        patternType={searchPatternType}
                        savedSearch={search}
                    />
                ))}
            </div>
        </Container>
    )
}
