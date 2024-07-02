import { useCallback, useEffect, useMemo, useState, type FunctionComponent } from 'react'

import { useApolloClient } from '@apollo/client'
import { mdiChevronRight, mdiDelete, mdiMapSearch, mdiRedo } from '@mdi/js'
import classNames from 'classnames'
import { useLocation } from 'react-router-dom'
import { Subject } from 'rxjs'
import { tap } from 'rxjs/operators'

import { isErrorLike } from '@sourcegraph/common'
import { gql, useQuery } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { RepoLink } from '@sourcegraph/shared/src/components/RepoLink'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Checkbox,
    Container,
    ErrorAlert,
    H3,
    Icon,
    Label,
    Link,
    PageHeader,
    Text,
    Tooltip,
} from '@sourcegraph/wildcard'

import {
    FilteredConnection,
    type Filter,
    type FilteredConnectionQueryArguments,
} from '../../../../components/FilteredConnection'
import { PageTitle } from '../../../../components/PageTitle'
import {
    PreciseIndexState,
    type IndexerListResult,
    type IndexerListVariables,
    type PreciseIndexFields,
    type PreciseIndexesVariables,
} from '../../../../graphql-operations'
import { FlashMessage } from '../../configuration/components/FlashMessage'
import { PreciseIndexLastUpdated } from '../components/CodeIntelLastUpdated'
import { CodeIntelStateIcon } from '../components/CodeIntelStateIcon'
import { CodeIntelStateLabel } from '../components/CodeIntelStateLabel'
import { EnqueueForm } from '../components/EnqueueForm'
import { ProjectDescription } from '../components/ProjectDescription'
import { queryPreciseIndexes as defaultQueryPreciseIndexes, statesFromString } from '../hooks/queryPreciseIndexes'
import { useDeletePreciseIndex as defaultUseDeletePreciseIndex } from '../hooks/useDeletePreciseIndex'
import { useDeletePreciseIndexes as defaultUseDeletePreciseIndexes } from '../hooks/useDeletePreciseIndexes'
import { useReindexPreciseIndex as defaultUseReindexPreciseIndex } from '../hooks/useReindexPreciseIndex'
import { useReindexPreciseIndexes as defaultUseReindexPreciseIndexes } from '../hooks/useReindexPreciseIndexes'

import styles from './CodeIntelPreciseIndexesPage.module.scss'

export const INDEXER_LIST = gql`
    query IndexerList {
        indexerKeys
    }
`

export interface CodeIntelPreciseIndexesPageProps extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    repo?: { id: string; name: string }
    queryPreciseIndexes?: typeof defaultQueryPreciseIndexes
    useDeletePreciseIndex?: typeof defaultUseDeletePreciseIndex
    useDeletePreciseIndexes?: typeof defaultUseDeletePreciseIndexes
    useReindexPreciseIndex?: typeof defaultUseReindexPreciseIndex
    useReindexPreciseIndexes?: typeof defaultUseReindexPreciseIndexes
    indexingEnabled?: boolean
}

const STATE_FILTER: Filter = {
    id: 'filters',
    label: 'State',
    type: 'select',
    options: [
        {
            label: 'All',
            value: 'all',
            tooltip: 'Show all indexes',
            args: {},
        },
        {
            label: 'Completed',
            value: 'completed',
            tooltip: 'Show completed indexes only',
            args: { states: PreciseIndexState.COMPLETED },
        },

        {
            label: 'Queued',
            value: 'queued',
            tooltip: 'Show queued indexes only',
            args: {
                states: [
                    PreciseIndexState.UPLOADING_INDEX,
                    PreciseIndexState.QUEUED_FOR_INDEXING,
                    PreciseIndexState.QUEUED_FOR_PROCESSING,
                ].join(','),
            },
        },
        {
            label: 'In progress',
            value: 'in-progress',
            tooltip: 'Show in-progress indexes only',
            args: { states: [PreciseIndexState.INDEXING, PreciseIndexState.PROCESSING].join(',') },
        },
        {
            label: 'Errored',
            value: 'errored',
            tooltip: 'Show errored indexes only',
            args: { states: [PreciseIndexState.INDEXING_ERRORED, PreciseIndexState.PROCESSING_ERRORED].join(',') },
        },
    ],
}

export const CodeIntelPreciseIndexesPage: FunctionComponent<CodeIntelPreciseIndexesPageProps> = ({
    authenticatedUser,
    repo,
    queryPreciseIndexes = defaultQueryPreciseIndexes,
    useDeletePreciseIndex = defaultUseDeletePreciseIndex,
    useDeletePreciseIndexes = defaultUseDeletePreciseIndexes,
    useReindexPreciseIndex = defaultUseReindexPreciseIndex,
    useReindexPreciseIndexes = defaultUseReindexPreciseIndexes,
    indexingEnabled = window.context?.codeIntelAutoIndexingEnabled,
    telemetryService,
    telemetryRecorder,
}) => {
    const location = useLocation()
    useEffect(() => {
        telemetryService.logViewEvent('CodeIntelPreciseIndexesPage')
        if (repo) {
            telemetryRecorder.recordEvent('repo.codeIntel.preciseIndexes', 'view')
        } else {
            telemetryRecorder.recordEvent('admin.codeIntel.preciseIndexes', 'view')
        }
    }, [telemetryService, telemetryRecorder, repo])

    const apolloClient = useApolloClient()
    const { handleDeletePreciseIndex, deleteError } = useDeletePreciseIndex()
    const { handleDeletePreciseIndexes, deletesError } = useDeletePreciseIndexes()
    const { handleReindexPreciseIndex, reindexError } = useReindexPreciseIndex()
    const { handleReindexPreciseIndexes, reindexesError } = useReindexPreciseIndexes()

    const { data: indexerData } = useQuery<IndexerListResult, IndexerListVariables>(INDEXER_LIST, {})

    const filters = useMemo<Filter[]>(() => {
        const indexerFilter: Filter = {
            id: 'filters-indexer',
            label: 'Indexer',
            type: 'select',
            options: [
                {
                    label: 'All',
                    value: 'all',
                    args: {},
                },
            ],
        }

        const keys = (indexerData?.indexerKeys || []).filter(key => Boolean(key))

        for (const key of keys) {
            indexerFilter.options.push({
                label: key,
                value: key,
                args: { indexerKey: key },
            })
        }

        return [STATE_FILTER, indexerFilter]
    }, [indexerData?.indexerKeys])

    // Poke filtered connection to refresh
    const refresh = useMemo(() => new Subject<void>(), [])
    const querySubject = useMemo(() => new Subject<string>(), [])

    // State used to control bulk index selection
    const [selection, setSelection] = useState<Set<string> | 'all'>(new Set())

    // Updates state of bulk index selection
    const onCheckboxToggle = useCallback(
        (id: string, checked: boolean) =>
            setSelection(selection =>
                selection === 'all'
                    ? selection
                    : checked
                    ? new Set([...selection, id])
                    : new Set([...selection].filter(selected => selected !== id))
            ),
        [setSelection]
    )

    // State used to spy on the query connection callback defined below
    const [args, setArgs] = useState<
        Partial<Omit<PreciseIndexesVariables, 'states'>> & { states?: string; isLatestForRepo?: boolean }
    >()
    const [totalCount, setTotalCount] = useState<number | undefined>(undefined)

    // Query indexes matching filter criteria
    const queryConnection = useCallback(
        (args: FilteredConnectionQueryArguments) => {
            const stashArgs = {
                repo: repo?.id,
                query: args.query,
                states: (args as any).states,
                isLatestForRepo: (args as any).isLatestForRepo,
                indexerKey: (args as any).indexerKey,
            }

            setArgs(stashArgs)
            setSelection(new Set())

            return queryPreciseIndexes(
                {
                    ...args,
                    ...stashArgs,
                },
                apolloClient
            ).pipe(
                tap(connection => {
                    setTotalCount(connection.totalCount ?? undefined)
                })
            )
        },
        [repo?.id, queryPreciseIndexes, apolloClient]
    )

    const onRawDelete = async (): Promise<void> => {
        if (selection === 'all') {
            if (args !== undefined && confirm(`Delete ${totalCount} indexes?`)) {
                const typedStates = statesFromString(args?.states)
                await handleDeletePreciseIndexes({
                    variables: {
                        repo: args.repo ?? null,
                        query: args.query ?? null,
                        states: typedStates.length > 0 ? typedStates : null,
                        indexerKey: args.indexerKey ?? null,
                        isLatestForRepo: args.isLatestForRepo ?? null,
                    },
                    update: cache => cache.modify({ fields: { node: () => {} } }),
                })
            }

            return
        }

        await Promise.all(
            [...selection].map(id =>
                handleDeletePreciseIndex({
                    variables: { id },
                    update: cache => cache.modify({ fields: { node: () => {} } }),
                })
            )
        )
    }

    const onRawReindex = async (): Promise<void> => {
        if (selection === 'all') {
            if (args !== undefined && confirm(`Mark ${totalCount} indexes as replaceable by auto-indexing?`)) {
                const typedStates = statesFromString(args?.states)
                await handleReindexPreciseIndexes({
                    variables: {
                        repo: args.repo ?? null,
                        query: args.query ?? null,
                        states: typedStates.length > 0 ? typedStates : null,
                        indexerKey: args.indexerKey ?? null,
                        isLatestForRepo: args.isLatestForRepo ?? null,
                    },
                    update: cache => cache.modify({ fields: { node: () => {} } }),
                })
            }

            return
        }

        await Promise.all(
            [...selection].map(id =>
                handleReindexPreciseIndex({
                    variables: { id },
                    update: cache => cache.modify({ fields: { node: () => {} } }),
                })
            )
        )
    }

    const onDelete = async (): Promise<void> => {
        await onRawDelete()
        refresh.next()
    }
    const onReindex = async (): Promise<void> => {
        await onRawReindex()
        refresh.next()
    }

    return (
        <div>
            <PageTitle title="Precise indexes" />
            <PageHeader
                headingElement="h2"
                path={[
                    {
                        text: repo ? (
                            <>
                                Precise indexes for <RepoLink repoName={repo.name} to={null} />
                            </>
                        ) : (
                            'Precise indexes'
                        ),
                    },
                ]}
                description="Precise code intelligence index data and auto-indexing jobs."
                actions={
                    repo &&
                    authenticatedUser?.siteAdmin && (
                        <Link to="/site-admin/code-graph/indexes">View indexes across all repositories</Link>
                    )
                }
                className="mb-3"
            />

            {!!location.state && <FlashMessage state={location.state.modal} message={location.state.message} />}

            {repo && authenticatedUser?.siteAdmin && (
                <Container className="mb-2">
                    <EnqueueForm repoId={repo.id} querySubject={querySubject} />
                </Container>
            )}

            {isErrorLike(deleteError) && <ErrorAlert prefix="Error deleting precise index" error={deleteError} />}
            {isErrorLike(deletesError) && <ErrorAlert prefix="Error deleting precise indexes" error={deletesError} />}
            {isErrorLike(reindexError) && (
                <ErrorAlert prefix="Error marking precise index as replaceable by auto-indexing" error={reindexError} />
            )}
            {isErrorLike(reindexesError) && (
                <ErrorAlert
                    prefix="Error marking precise indexes as replaceable by auto-indexing"
                    error={reindexesError}
                />
            )}

            <Container>
                <div className="list-group position-relative">
                    <FilteredConnection<PreciseIndexFields, Omit<IndexNodeProps, 'node'>>
                        listComponent="div"
                        inputClassName="ml-2 flex-1"
                        listClassName="mb-3"
                        formClassName={styles.form}
                        noun="precise index"
                        pluralNoun="precise indexes"
                        querySubject={querySubject}
                        nodeComponent={IndexNode}
                        nodeComponentProps={{ repo, selection, onCheckboxToggle, authenticatedUser }}
                        headComponent={
                            authenticatedUser?.siteAdmin
                                ? () => (
                                      <div className={styles.header}>
                                          <Label className={styles.checkbox}>
                                              <Checkbox
                                                  aria-label="Select all indexes"
                                                  id="checkAll"
                                                  checked={selection === 'all'}
                                                  wrapperClassName="d-flex align-items-center"
                                                  onChange={() =>
                                                      setSelection(selection =>
                                                          selection === 'all' ? new Set() : 'all'
                                                      )
                                                  }
                                              />
                                          </Label>

                                          <div className="text-right">
                                              {indexingEnabled && (
                                                  <Tooltip
                                                      content={`Allow Sourcegraph to re-index ${
                                                          selection === 'all' || selection.size > 1
                                                              ? 'these commits'
                                                              : 'this commit'
                                                      } in the future and replace this data.`}
                                                  >
                                                      <Button
                                                          className="mr-2"
                                                          variant="secondary"
                                                          disabled={selection !== 'all' && selection.size === 0}
                                                          onClick={onReindex}
                                                      >
                                                          <Icon aria-hidden={true} svgPath={mdiRedo} /> Mark{' '}
                                                          {(selection === 'all' ? totalCount : selection.size) === 0 ? (
                                                              ''
                                                          ) : (
                                                              <>
                                                                  {selection === 'all' ? totalCount : selection.size}{' '}
                                                                  {(selection === 'all'
                                                                      ? totalCount
                                                                      : selection.size) === 1
                                                                      ? 'index'
                                                                      : 'indexes'}
                                                              </>
                                                          )}{' '}
                                                          as replaceable by auto-indexing
                                                      </Button>
                                                  </Tooltip>
                                              )}
                                              <Button
                                                  className="mr-2"
                                                  variant="danger"
                                                  disabled={selection !== 'all' && selection.size === 0}
                                                  onClick={onDelete}
                                              >
                                                  <Icon aria-hidden={true} svgPath={mdiDelete} /> Delete{' '}
                                                  {(selection === 'all' ? totalCount : selection.size) === 0 ? (
                                                      ''
                                                  ) : (
                                                      <>
                                                          {selection === 'all' ? totalCount : selection.size}{' '}
                                                          {(selection === 'all' ? totalCount : selection.size) === 1
                                                              ? 'index'
                                                              : 'indexes'}
                                                      </>
                                                  )}
                                              </Button>
                                          </div>
                                      </div>
                                  )
                                : undefined
                        }
                        queryConnection={queryConnection}
                        cursorPaging={true}
                        filters={filters}
                        emptyElement={<EmptyIndex />}
                        updates={refresh}
                    />
                </div>
            </Container>
        </div>
    )
}

interface IndexNodeProps {
    authenticatedUser: AuthenticatedUser | null
    node: PreciseIndexFields
    repo?: { id: string }
    selection: Set<string> | 'all'
    onCheckboxToggle: (id: string, checked: boolean) => void
}

const IndexNode: FunctionComponent<IndexNodeProps> = ({
    node,
    repo,
    selection,
    onCheckboxToggle,
    authenticatedUser,
}) => (
    <div className={classNames(styles.grid, authenticatedUser?.siteAdmin && styles.gridControlled)}>
        {authenticatedUser?.siteAdmin && (
            <Label className={styles.checkbox}>
                <Checkbox
                    aria-label="Select index"
                    id="disabledFieldsetCheck"
                    disabled={selection === 'all'}
                    checked={selection === 'all' ? true : selection.has(node.id)}
                    onChange={input => onCheckboxToggle(node.id, input.target.checked)}
                    wrapperClassName="d-flex align-items-center"
                />
            </Label>
        )}
        <div className={styles.information}>
            {!repo && (
                <div>
                    <H3 className="m-0 mb-1">
                        {node.projectRoot ? (
                            <Link to={node.projectRoot.repository.url}>{node.projectRoot.repository.name}</Link>
                        ) : (
                            <span>Unknown repository</span>
                        )}
                    </H3>
                </div>
            )}

            <div>
                <span className="mr-2 d-block">
                    <ProjectDescription index={node} />
                </span>

                <small className="text-muted">
                    <PreciseIndexLastUpdated index={node} />{' '}
                    {node.shouldReindex && (
                        <Tooltip content="This index has been marked as replaceable by auto-indexing.">
                            <span className={classNames(styles.tag, 'ml-1 rounded')}>
                                (replaceable by auto-indexing)
                            </span>
                        </Tooltip>
                    )}
                </small>
            </div>
        </div>
        <span className={classNames(styles.state, 'd-none d-md-inline')}>
            <div className="d-flex flex-column align-items-center">
                <CodeIntelStateIcon state={node.state} autoIndexed={!!node.indexingFinishedAt} />
                <CodeIntelStateLabel
                    state={node.state}
                    autoIndexed={!!node.indexingFinishedAt}
                    placeInQueue={node.placeInQueue}
                    className="mt-2"
                />
            </div>
        </span>
        <Link to={`./${node.id}`} className="d-flex justify-content-end align-items-center align-self-stretch p-0">
            <Icon svgPath={mdiChevronRight} inline={false} aria-label="View details" />
        </Link>
    </div>
)

const EmptyIndex: React.FunctionComponent<{}> = () => (
    <Text alignment="center" className="text-muted w-100 mb-0 mt-1">
        <Icon className="mb-2" svgPath={mdiMapSearch} inline={false} aria-hidden={true} />
        <br />
        No indexes.
    </Text>
)
