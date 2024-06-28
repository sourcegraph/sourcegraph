import { useEffect, useMemo, useState, type FunctionComponent, type MutableRefObject } from 'react'

import { mdiMagnify } from '@mdi/js'
import classNames from 'classnames'
import { useLocation } from 'react-router-dom'
import { useCallbackRef } from 'use-callback-ref'

import type { SearchPatternTypeProps } from '@sourcegraph/shared/src/search'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import {
    Badge,
    Button,
    Container,
    ErrorAlert,
    Icon,
    Link,
    LoadingSpinner,
    PageSwitcher,
    Text,
    useDebounce,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import type { FilteredConnectionFilter } from '../components/FilteredConnection'
import { usePageSwitcherPagination } from '../components/FilteredConnection/hooks/usePageSwitcherPagination'
import { ConnectionContainer, ConnectionForm } from '../components/FilteredConnection/ui'
import { useNamespaces } from '../enterprise/batches/create/useNamespaces'
import {
    SavedSearchesOrderBy,
    type SavedSearchFields,
    type SavedSearchesResult,
    type SavedSearchesVariables,
} from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'
import { useNavbarQueryState } from '../stores'

import { savedSearchesQuery } from './graphql'

import styles from './ListPage.module.scss'

const SavedSearchNode: FunctionComponent<
    SearchPatternTypeProps & {
        savedSearch: SavedSearchFields
        linkRef: MutableRefObject<HTMLAnchorElement | null> | null
    }
> = ({ savedSearch, patternType, linkRef }) => (
    <div className={classNames(styles.row, 'list-group-item align-items-center flex-gap-4')}>
        <Button
            as={Link}
            to={`/search?${buildSearchURLQuery(savedSearch.query, patternType, false)}`}
            variant="link"
            size="lg"
            className={classNames(
                'd-flex flex-gap-2 align-items-center flex-grow-1 text-left text-decoration-none pl-0',
                styles.searchLink
            )}
            ref={linkRef}
        >
            <Badge variant="primary" className="py-1 d-flex flex-gap-1 align-items-center mr-1">
                <Icon aria-label="Run search" svgPath={mdiMagnify} className="flex-shrink-0" size="sm" />
                Run search
            </Badge>
            <span className={styles.searchLinkDescription}>{savedSearch.description}</span>
        </Button>
        <Button to={savedSearch.url} variant="secondary" as={Link}>
            Edit
        </Button>
    </div>
)

interface FilterValue extends Pick<SavedSearchesVariables, 'query' | 'owner' | 'orderBy'> {}

export const ListPage: FunctionComponent<NamespaceProps & { authenticatedUser: AuthenticatedUser }> = ({
    namespace,
    authenticatedUser,
    telemetryRecorder,
}) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('savedSearches.list', 'view', {
            metadata: namespaceTelemetryMetadata(namespace),
        })
    }, [telemetryRecorder, namespace])

    const [filterValue, setFilterValue] = useState<FilterValue>({
        query: '',
        orderBy: SavedSearchesOrderBy.SAVED_SEARCH_UPDATED_AT,
        owner: namespace.id,
    })

    const { namespaces } = useNamespaces(authenticatedUser)
    const filters = useMemo<FilteredConnectionFilter[]>(
        () => [
            {
                label: 'Sort',
                type: 'select',
                id: 'orderBy',
                values: [
                    {
                        value: 'updated-at-desc',
                        label: 'Recently updated',
                        args: {
                            orderBy: SavedSearchesOrderBy.SAVED_SEARCH_UPDATED_AT,
                        },
                    },
                    {
                        value: 'description-asc',
                        label: 'By description',
                        args: {
                            orderBy: SavedSearchesOrderBy.SAVED_SEARCH_DESCRIPTION,
                        },
                    },
                ],
            },
            {
                label: 'Owner',
                type: 'select',
                id: 'owner',
                tooltip: 'User or organization that owns the saved search',
                values: [
                    {
                        value: 'all',
                        label: 'All',
                        args: {},
                    },
                    ...namespaces.map(namespace => ({
                        value: namespace.id,
                        label: namespace.namespaceName,
                        args: {
                            namespace: namespace.id,
                        },
                    })),
                ],
            },
        ],
        [namespaces]
    )

    const debouncedQuery = useDebounce(filterValue.query, 300)
    const { connection, loading, error, ...paginationProps } = usePageSwitcherPagination<
        SavedSearchesResult,
        SavedSearchesVariables,
        SavedSearchFields
    >({
        query: savedSearchesQuery,
        variables: { ...filterValue, query: debouncedQuery },
        getConnection: ({ data }) => data?.savedSearches || undefined,
        options: {
            useURL: true,
        },
    })

    const location = useLocation()
    const searchPatternType = useNavbarQueryState(state => state.searchPatternType)
    const callbackReference = useCallbackRef<HTMLAnchorElement>(null, ref => ref?.focus())

    return (
        <>
            <Container>
                <ConnectionContainer>
                    <ConnectionForm
                        hideSearch={false}
                        showSearchFirst={true}
                        inputClassName="mw-30"
                        inputPlaceholder="Find a saved search..."
                        inputAriaLabel=""
                        inputValue={filterValue.query ?? ''}
                        onInputChange={event => {
                            setFilterValue(prev => ({ ...prev, query: event.target.value }))
                        }}
                        autoFocus={false}
                        filters={filters}
                        onFilterSelect={(_filter, value) => setFilterValue(prev => ({ ...prev, ...value.args }))}
                        filterValues={new Map(Object.entries(filterValue))}
                        compact={false}
                        formClassName="flex-gap-4 mb-4"
                    />
                    {loading ? (
                        <LoadingSpinner />
                    ) : error ? (
                        <ErrorAlert error={error} className="mb-3" />
                    ) : !connection?.nodes || connection.nodes.length === 0 ? (
                        <Text className="text-center text-muted">No saved searches found.</Text>
                    ) : (
                        <div className="list-group list-group-flush">
                            {connection.nodes.map(savedSearch => (
                                <SavedSearchNode
                                    key={savedSearch.id}
                                    linkRef={
                                        location.state?.description === savedSearch.description
                                            ? callbackReference
                                            : null
                                    }
                                    patternType={searchPatternType}
                                    savedSearch={savedSearch}
                                />
                            ))}
                        </div>
                    )}
                </ConnectionContainer>
            </Container>
            <PageSwitcher {...paginationProps} className="mt-4" totalCount={connection?.totalCount ?? null} />
        </>
    )
}
