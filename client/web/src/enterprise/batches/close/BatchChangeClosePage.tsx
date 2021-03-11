import React, { useState, useMemo, useCallback } from 'react'
import * as H from 'history'
import { PageTitle } from '../../../components/PageTitle'
import { BatchChangeCloseAlert } from './BatchChangeCloseAlert'
import { BatchChangeChangesetsResult, BatchChangeFields, Scalars } from '../../../graphql-operations'
import {
    queryExternalChangesetWithFileDiffs as _queryExternalChangesetWithFileDiffs,
    queryChangesets as _queryChangesets,
    fetchBatchChangeByNamespace as _fetchBatchChangeByNamespace,
} from '../detail/backend'
import { ThemeProps } from '../../../../../shared/src/theme'
import { PlatformContextProps } from '../../../../../shared/src/platform/context'
import { ExtensionsControllerProps } from '../../../../../shared/src/extensions/controller'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import { closeBatchChange as _closeBatchChange } from './backend'
import { BatchChangeCloseChangesetsList } from './BatchChangeCloseChangesetsList'
import { useObservable } from '../../../../../shared/src/util/useObservable'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { HeroPage } from '../../../components/HeroPage'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import { BatchChangeInfoByline } from '../detail/BatchChangeInfoByline'
import { ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { BatchChangesIcon } from '../icons'
import { PageHeader } from '../../../components/PageHeader'

export interface BatchChangeClosePageProps
    extends ThemeProps,
        TelemetryProps,
        PlatformContextProps,
        ExtensionsControllerProps {
    /**
     * The namespace ID.
     */
    namespaceID: Scalars['ID']
    /**
     * The batch change name.
     */
    batchChangeName: BatchChangeFields['name']
    history: H.History
    location: H.Location

    /** For testing only. */
    fetchBatchChangeByNamespace?: typeof _fetchBatchChangeByNamespace
    /** For testing only. */
    queryChangesets?: typeof _queryChangesets
    /** For testing only. */
    queryExternalChangesetWithFileDiffs?: typeof _queryExternalChangesetWithFileDiffs
    /** For testing only. */
    closeBatchChange?: typeof _closeBatchChange
}

export const BatchChangeClosePage: React.FunctionComponent<BatchChangeClosePageProps> = ({
    namespaceID,
    batchChangeName,
    history,
    location,
    extensionsController,
    isLightTheme,
    platformContext,
    telemetryService,
    fetchBatchChangeByNamespace = _fetchBatchChangeByNamespace,
    queryChangesets,
    queryExternalChangesetWithFileDiffs,
    closeBatchChange,
}) => {
    const [closeChangesets, setCloseChangesets] = useState<boolean>(false)
    const batchChange = useObservable(
        useMemo(() => fetchBatchChangeByNamespace(namespaceID, batchChangeName), [
            namespaceID,
            batchChangeName,
            fetchBatchChangeByNamespace,
        ])
    )

    const [totalCount, setTotalCount] = useState<number>()

    const onFetchChangesets = useCallback(
        (
            connection?: (BatchChangeChangesetsResult['node'] & { __typename: 'BatchChange' })['changesets'] | ErrorLike
        ) => {
            if (!connection || isErrorLike(connection)) {
                return
            }
            setTotalCount(connection.totalCount)
        },
        []
    )

    // Is loading.
    if (batchChange === undefined) {
        return (
            <div className="text-center">
                <LoadingSpinner className="icon-inline mx-auto my-4" />
            </div>
        )
    }

    // Batch change not found.
    if (batchChange === null) {
        return <HeroPage icon={AlertCircleIcon} title="Batch change not found" />
    }

    return (
        <>
            <PageTitle title="Preview close" />
            <PageHeader
                path={[
                    {
                        icon: BatchChangesIcon,
                        to: '/batch-changes',
                    },
                    { to: `${batchChange.namespace.url}/batch-changes`, text: batchChange.namespace.namespaceName },
                    { text: batchChange.name },
                ]}
                byline={
                    <BatchChangeInfoByline
                        createdAt={batchChange.createdAt}
                        initialApplier={batchChange.initialApplier}
                        lastAppliedAt={batchChange.lastAppliedAt}
                        lastApplier={batchChange.lastApplier}
                    />
                }
                className="test-batch-change-close-page mb-3"
            />
            {totalCount !== undefined && (
                <BatchChangeCloseAlert
                    batchChangeID={batchChange.id}
                    batchChangeURL={batchChange.url}
                    closeChangesets={closeChangesets}
                    setCloseChangesets={setCloseChangesets}
                    history={history}
                    closeBatchChange={closeBatchChange}
                    viewerCanAdminister={batchChange.viewerCanAdminister}
                    totalCount={totalCount}
                />
            )}
            <BatchChangeCloseChangesetsList
                batchChangeID={batchChange.id}
                history={history}
                location={location}
                viewerCanAdminister={batchChange.viewerCanAdminister}
                extensionsController={extensionsController}
                isLightTheme={isLightTheme}
                platformContext={platformContext}
                telemetryService={telemetryService}
                queryChangesets={queryChangesets}
                queryExternalChangesetWithFileDiffs={queryExternalChangesetWithFileDiffs}
                willClose={closeChangesets}
                onUpdate={onFetchChangesets}
            />
        </>
    )
}
