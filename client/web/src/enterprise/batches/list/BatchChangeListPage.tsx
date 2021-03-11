import React, { useEffect, useCallback, useState, useMemo } from 'react'
import {
    areBatchChangesLicensed as _areBatchChangesLicensed,
    queryBatchChanges as _queryBatchChanges,
    queryBatchChangesByNamespace,
} from './backend'
import { RouteComponentProps } from 'react-router'
import { FilteredConnection, FilteredConnectionFilter } from '../../../components/FilteredConnection'
import { BatchChangeNode, BatchChangeNodeProps } from './BatchChangeNode'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import {
    ListBatchChange,
    Scalars,
    BatchChangeState,
    BatchChangesVariables,
    BatchChangesResult,
    BatchChangesByNamespaceVariables,
} from '../../../graphql-operations'
import PlusIcon from 'mdi-react/PlusIcon'
import { Link } from '../../../../../shared/src/components/Link'
import { PageHeader } from '../../../components/PageHeader'
import { BatchChangesIcon } from '../icons'
import { BatchChangesListEmpty } from './BatchChangesListEmpty'
import { BatchChangesListIntro } from './BatchChangesListIntro'
import { filter, map, tap, withLatestFrom } from 'rxjs/operators'
import { Observable, ReplaySubject } from 'rxjs'
import classNames from 'classnames'
import { useObservable } from '../../../../../shared/src/util/useObservable'

export interface BatchChangeListPageProps extends TelemetryProps, Pick<RouteComponentProps, 'history' | 'location'> {
    displayNamespace?: boolean
    /** For testing only. */
    queryBatchChanges?: typeof _queryBatchChanges
    /** For testing only. */
    areBatchChangesLicensed?: typeof _areBatchChangesLicensed
    /** For testing only. */
    openTab?: SelectedTab
}

const FILTERS: FilteredConnectionFilter[] = [
    {
        id: 'status',
        label: 'Status',
        type: 'radio',
        values: [
            {
                label: 'Open',
                value: 'open',
                tooltip: 'Show only batch changes that are open',
                args: { state: BatchChangeState.OPEN },
            },
            {
                label: 'Closed',
                value: 'closed',
                tooltip: 'Show only batch changes that are closed',
                args: { state: BatchChangeState.CLOSED },
            },
            {
                label: 'All',
                value: 'all',
                tooltip: 'Show all batch changes',
                args: {},
            },
        ],
    },
]

type SelectedTab = 'batchChanges' | 'gettingStarted'

/**
 * A list of all batch changes on the Sourcegraph instance.
 */
export const BatchChangeListPage: React.FunctionComponent<BatchChangeListPageProps> = ({
    queryBatchChanges = _queryBatchChanges,
    areBatchChangesLicensed = _areBatchChangesLicensed,
    displayNamespace = true,
    location,
    openTab,
    ...props
}) => {
    useEffect(() => props.telemetryService.logViewEvent('CampaignsListPage'), [props.telemetryService])

    /*
     * Tracks whether this is the first fetch since this page has been rendered the first time.
     * Used to only switch to the "Getting started" tab if the user didn't select the tab manually.
     */
    const isFirstFetch = useMemo(() => {
        const subject = new ReplaySubject(1)
        subject.next(true)
        return subject
    }, [])
    const [selectedTab, setSelectedTab] = useState<SelectedTab>(openTab ?? 'batchChanges')
    const query = useCallback<(args: Partial<BatchChangesVariables>) => Observable<BatchChangesResult['batchChanges']>>(
        args =>
            queryBatchChanges(args).pipe(
                withLatestFrom(isFirstFetch),
                tap(([response, isFirst]) => {
                    if (isFirst) {
                        isFirstFetch.next(false)
                        if (!openTab && response.totalCount === 0) {
                            setSelectedTab('gettingStarted')
                        }
                    }
                }),
                // Don't emit when we are switching to the getting started tab right away to prevent a costly render.
                // Only if:
                //  - We don't fetch for the first time (the user clicked a tab) OR
                //  - There are more than 0 changesets in the namespace OR
                //  - A test forces us to display a specific tab
                filter(([response, isFirst]) => !isFirst || openTab !== undefined || response.totalCount > 0),
                map(([response]) => response.batchChanges)
            ),
        [queryBatchChanges, isFirstFetch, openTab]
    )
    const licensed: boolean | undefined = useObservable(
        useMemo(() => areBatchChangesLicensed(), [areBatchChangesLicensed])
    )

    return (
        <>
            <PageHeader
                path={[{ icon: BatchChangesIcon, text: 'Batch Changes' }]}
                className="test-batches-list-page mb-3"
                actions={<NewBatchChangeButton location={location} />}
                byline="Run custom code over hundreds of repositories and manage the resulting changesets"
            />
            <BatchChangesListIntro licensed={licensed} />
            <BatchChangeListTabHeader selectedTab={selectedTab} setSelectedTab={setSelectedTab} />
            {selectedTab === 'gettingStarted' && <BatchChangesListEmpty />}
            {selectedTab === 'batchChanges' && (
                <FilteredConnection<ListBatchChange, Omit<BatchChangeNodeProps, 'node'>>
                    {...props}
                    location={location}
                    nodeComponent={BatchChangeNode}
                    nodeComponentProps={{ history: props.history, displayNamespace }}
                    queryConnection={query}
                    hideSearch={true}
                    defaultFirst={15}
                    filters={FILTERS}
                    noun="batch change"
                    pluralNoun="batch changes"
                    listComponent="div"
                    listClassName="batch-change-list-page__grid mb-3"
                    className="mb-3"
                    cursorPaging={true}
                    noSummaryIfAllNodesVisible={true}
                    emptyElement={<BatchChangeListEmptyElement location={location} />}
                />
            )}
        </>
    )
}

export interface NamespaceBatchChangeListPageProps extends BatchChangeListPageProps {
    namespaceID: Scalars['ID']
}

/**
 * A list of all batch changes in a namespace.
 */
export const NamespaceBatchChangeListPage: React.FunctionComponent<NamespaceBatchChangeListPageProps> = ({
    namespaceID,
    ...props
}) => {
    const queryConnection = useCallback(
        (args: Partial<BatchChangesByNamespaceVariables>) =>
            queryBatchChangesByNamespace({
                namespaceID,
                first: args.first ?? null,
                after: args.after ?? null,
                // The types for FilteredConnectionQueryArguments don't allow access to the filter arguments.
                state: (args as { state: BatchChangeState | undefined }).state ?? null,
                viewerCanAdminister: null,
            }),
        [namespaceID]
    )
    return <BatchChangeListPage {...props} displayNamespace={false} queryBatchChanges={queryConnection} />
}

interface BatchChangeListEmptyElementProps extends Pick<RouteComponentProps, 'location'> {}

const BatchChangeListEmptyElement: React.FunctionComponent<BatchChangeListEmptyElementProps> = ({ location }) => (
    <div className="w-100 py-5 text-center">
        <p>
            <strong>No batch changes have been created</strong>
        </p>
        <NewBatchChangeButton location={location} />
    </div>
)

interface NewBatchChangeButtonProps extends Pick<RouteComponentProps, 'location'> {}

const NewBatchChangeButton: React.FunctionComponent<NewBatchChangeButtonProps> = ({ location }) => (
    <Link to={`${location.pathname}/create`} className="btn btn-secondary">
        <PlusIcon className="icon-inline" /> Create batch change
    </Link>
)

const BatchChangeListTabHeader: React.FunctionComponent<{
    selectedTab: SelectedTab
    setSelectedTab: (selectedTab: SelectedTab) => void
}> = ({ selectedTab, setSelectedTab }) => {
    const onSelectBatchChanges = useCallback<React.MouseEventHandler>(
        event => {
            event.preventDefault()
            setSelectedTab('batchChanges')
        },
        [setSelectedTab]
    )
    const onSelectGettingStarted = useCallback<React.MouseEventHandler>(
        event => {
            event.preventDefault()
            setSelectedTab('gettingStarted')
        },
        [setSelectedTab]
    )
    return (
        <div className="overflow-auto mb-4">
            <ul className="nav nav-tabs d-inline-flex d-sm-flex flex-nowrap text-nowrap">
                <li className="nav-item">
                    <a
                        href=""
                        onClick={onSelectBatchChanges}
                        className={classNames('nav-link', selectedTab === 'batchChanges' && 'active')}
                    >
                        All batch changes
                    </a>
                </li>
                <li className="nav-item">
                    <a
                        href=""
                        onClick={onSelectGettingStarted}
                        className={classNames('nav-link', selectedTab === 'gettingStarted' && 'active')}
                    >
                        Getting started
                    </a>
                </li>
            </ul>
        </div>
    )
}
