import { useCallback, useEffect, type FunctionComponent, type MutableRefObject, type PropsWithChildren } from 'react'

import { mdiPlus } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'
import { useLocation } from 'react-router-dom'
import { useCallbackRef } from 'use-callback-ref'

import { logger } from '@sourcegraph/common'
import { useMutation } from '@sourcegraph/http-client'
import type { SearchPatternTypeProps } from '@sourcegraph/shared/src/search'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import {
    Button,
    Code,
    Container,
    ErrorAlert,
    H2,
    Icon,
    Link,
    LoadingSpinner,
    PageHeader,
    PageSwitcher,
} from '@sourcegraph/wildcard'

import { usePageSwitcherPagination } from '../components/FilteredConnection/hooks/usePageSwitcherPagination'
import { PageTitle } from '../components/PageTitle'
import type { SavedSearchFields, SavedSearchesResult, SavedSearchesVariables } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import type { NamespaceAreaContext } from '../namespaces/NamespaceArea'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'
import { useNavbarQueryState } from '../stores'

import { deleteSavedSearchMutation, savedSearchesQuery } from './backend'

import styles from './SavedSearchListPage.module.scss'

const SavedSearchNode: FunctionComponent<
    SearchPatternTypeProps &
        TelemetryV2Props & {
            namespace: Pick<NamespaceAreaContext['namespace'], '__typename'>
            savedSearch: SavedSearchFields
            onDelete: () => void
            linkRef: MutableRefObject<HTMLAnchorElement | null> | null
        }
> = ({ savedSearch, patternType, onDelete, telemetryRecorder, namespace, linkRef }) => {
    const [deleteSavedSearch, { loading }] = useMutation(deleteSavedSearchMutation)

    const handleDelete = useCallback(async (): Promise<void> => {
        if (!window.confirm(`Delete the saved search ${JSON.stringify(savedSearch.description)}?`)) {
            return
        }
        try {
            await deleteSavedSearch({ variables: { id: savedSearch.id } })
            telemetryRecorder.recordEvent('savedSearches', 'delete', {
                metadata: namespaceTelemetryMetadata(namespace),
            })
            onDelete()
        } catch (error) {
            logger.error(error)
        }
    }, [deleteSavedSearch, onDelete, savedSearch, telemetryRecorder, namespace])

    return (
        <div className={classNames(styles.row, 'list-group-item test-saved-search-list-page-row')}>
            <div className="flex-1">
                <H2 className="text-base mb-0">
                    <Link
                        to={'/search?' + buildSearchURLQuery(savedSearch.query, patternType, false)}
                        ref={linkRef}
                        className="font-weight-bold"
                    >
                        <span className="test-saved-search-list-page-row-title">
                            <VisuallyHidden>Run saved search: </VisuallyHidden>
                            {savedSearch.description}
                        </span>
                    </Link>
                </H2>
                <Code>{savedSearch.query}</Code>
            </div>
            <div className="flex-0">
                <Button
                    className="test-edit-saved-search-button"
                    to={savedSearch.id}
                    variant="secondary"
                    size="sm"
                    as={Link}
                >
                    Edit
                </Button>{' '}
                <Button
                    aria-label="Delete"
                    className="test-delete-saved-search-button"
                    onClick={handleDelete}
                    disabled={loading}
                    variant="danger"
                    size="sm"
                >
                    Delete
                </Button>
            </div>
            {loading && (
                <VisuallyHidden aria-live="polite">{`Deleted saved search: ${savedSearch.description}`}</VisuallyHidden>
            )}
        </div>
    )
}

export const SavedSearchListPage: FunctionComponent<NamespaceProps> = ({ namespace, telemetryRecorder }) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('savedSearches.list', 'view', {
            metadata: namespaceTelemetryMetadata(namespace),
        })
    }, [telemetryRecorder, namespace])

    const { connection, loading, error, refetch, ...paginationProps } = usePageSwitcherPagination<
        SavedSearchesResult,
        SavedSearchesVariables,
        SavedSearchFields
    >({
        query: savedSearchesQuery,
        variables: { owner: namespace.id },
        getConnection: ({ data }) => data?.savedSearches || undefined,
    })

    return (
        <div className={styles.savedSearchListPage} data-testid="saved-searches-list-page">
            <PageHeader
                actions={
                    <Button to="new" className="test-new-saved-search-button" variant="primary" as={Link}>
                        <Icon aria-hidden={true} svgPath={mdiPlus} /> New saved search
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
                namespace={namespace}
                telemetryRecorder={telemetryRecorder}
                onDelete={refetch}
                savedSearches={connection?.nodes || []}
                error={error}
                loading={loading}
            />
            <PageSwitcher {...paginationProps} className="mt-4" totalCount={connection?.totalCount || 0} />
        </div>
    )
}

const SavedSearchListPageContent: FunctionComponent<
    PropsWithChildren<
        {
            onDelete: () => void
            savedSearches: SavedSearchFields[]
            error: unknown
            loading: boolean
        } & NamespaceProps
    >
> = ({ namespace, savedSearches, error, loading, ...props }) => {
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
                {savedSearches.map(savedSearch => (
                    <SavedSearchNode
                        key={savedSearch.id}
                        linkRef={location.state?.description === savedSearch.description ? callbackReference : null}
                        patternType={searchPatternType}
                        savedSearch={savedSearch}
                        namespace={namespace}
                        {...props}
                    />
                ))}
            </div>
        </Container>
    )
}
