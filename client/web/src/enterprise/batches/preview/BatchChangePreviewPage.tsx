import * as H from 'history'
import { isEqual, reduce } from 'lodash'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import FileDocumentIcon from 'mdi-react/FileDocumentIcon'
import SourceBranchIcon from 'mdi-react/SourceBranchIcon'
import React, { useEffect, useMemo } from 'react'
import { delay, distinctUntilChanged, repeatWhen } from 'rxjs/operators'

import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'
import { useObservable } from '@sourcegraph/shared/src/util/useObservable'
import { PageHeader } from '@sourcegraph/wildcard'

import { AuthenticatedUser } from '../../../auth'
import { BatchChangesIcon } from '../../../batches/icons'
import { HeroPage } from '../../../components/HeroPage'
import { PageTitle } from '../../../components/PageTitle'
import {
    BatchChangeTab,
    BatchChangeTabPanel,
    BatchChangeTabPanels,
    BatchChangeTabs,
    BatchChangeTabsList,
} from '../BatchChangeTabs'
import { Description } from '../Description'
import { BatchSpecTab } from '../detail/BatchSpecTab'
import { SupersedingBatchSpecAlert } from '../detail/SupersedingBatchSpecAlert'

import { fetchBatchSpecById as _fetchBatchSpecById } from './backend'
import { BatchChangePreviewStatsBar } from './BatchChangePreviewStatsBar'
import { BatchSpecInfoByline } from './BatchSpecInfoByline'
import { CreateUpdateBatchChangeAlert } from './CreateUpdateBatchChangeAlert'
import { queryChangesetSpecFileDiffs, queryChangesetApplyPreview } from './list/backend'
import { PreviewList } from './list/PreviewList'
import { MissingCredentialsAlert } from './MissingCredentialsAlert'

export type PreviewPageAuthenticatedUser = Pick<AuthenticatedUser, 'url' | 'displayName' | 'username' | 'email'>

export interface BatchChangePreviewPageProps extends ThemeProps, TelemetryProps {
    batchSpecID: string
    history: H.History
    location: H.Location
    authenticatedUser: PreviewPageAuthenticatedUser

    /** Used for testing. */
    fetchBatchSpecById?: typeof _fetchBatchSpecById
    /** Used for testing. */
    queryChangesetApplyPreview?: typeof queryChangesetApplyPreview
    /** Used for testing. */
    queryChangesetSpecFileDiffs?: typeof queryChangesetSpecFileDiffs
    /** Expand changeset descriptions, for testing only. */
    expandChangesetDescriptions?: boolean
}

export const BatchChangePreviewPage: React.FunctionComponent<BatchChangePreviewPageProps> = ({
    batchSpecID: specID,
    history,
    location,
    authenticatedUser,
    isLightTheme,
    telemetryService,
    fetchBatchSpecById = _fetchBatchSpecById,
    queryChangesetApplyPreview,
    queryChangesetSpecFileDiffs,
    expandChangesetDescriptions,
}) => {
    const spec = useObservable(
        useMemo(
            () =>
                fetchBatchSpecById(specID).pipe(
                    repeatWhen(notifier => notifier.pipe(delay(5000))),
                    distinctUntilChanged((a, b) => isEqual(a, b))
                ),
            [specID, fetchBatchSpecById]
        )
    )

    useEffect(() => {
        telemetryService.logViewEvent('BatchChangeApplyPage')
    }, [telemetryService])

    if (spec === undefined) {
        return (
            <div className="text-center">
                <LoadingSpinner className="icon-inline mx-auto my-4" />
            </div>
        )
    }
    if (spec === null) {
        return <HeroPage icon={AlertCircleIcon} title="Batch spec not found" />
    }

    return (
        <div className="pb-5">
            <PageTitle title="Apply batch spec" />
            <PageHeader
                path={[
                    {
                        icon: BatchChangesIcon,
                        to: '/batch-changes',
                    },
                    { to: `${spec.namespace.url}/batch-changes`, text: spec.namespace.namespaceName },
                    { text: spec.description.name },
                ]}
                byline={<BatchSpecInfoByline createdAt={spec.createdAt} creator={spec.creator} />}
                className="test-batch-change-apply-page mb-3"
            />
            <MissingCredentialsAlert
                authenticatedUser={authenticatedUser}
                viewerBatchChangesCodeHosts={spec.viewerBatchChangesCodeHosts}
            />
            <SupersedingBatchSpecAlert spec={spec.supersedingBatchSpec} />
            <BatchChangePreviewStatsBar batchSpec={spec} />
            <CreateUpdateBatchChangeAlert
                history={history}
                specID={spec.id}
                toBeArchived={spec.applyPreview.stats.archive}
                batchChange={spec.appliesToBatchChange}
                viewerCanAdminister={spec.viewerCanAdminister}
                telemetryService={telemetryService}
            />
            <Description description={spec.description.description} />
            <BatchChangeTabs history={history} location={location}>
                <BatchChangeTabsList>
                    <BatchChangeTab index={0}>
                        <SourceBranchIcon className="icon-inline text-muted mr-1" />
                        Changesets{' '}
                        <span className="badge badge-pill badge-secondary ml-1">
                            {/* TODO: This doesn't seem to yield the right sum (saw 2 when it should be 1), also should this be added as a GraphQL field like it is for BatchChange stats? */}
                            {reduce(
                                spec.applyPreview.stats,
                                (sum, statValue, statKey) => (statKey === 'archive' ? sum : sum + statValue),
                                0
                            )}
                        </span>
                    </BatchChangeTab>
                    <BatchChangeTab index={1}>
                        <FileDocumentIcon className="icon-inline text-muted mr-1" /> Spec
                    </BatchChangeTab>
                </BatchChangeTabsList>
                <BatchChangeTabPanels>
                    <BatchChangeTabPanel>
                        <PreviewList
                            batchSpecID={specID}
                            history={history}
                            location={location}
                            authenticatedUser={authenticatedUser}
                            isLightTheme={isLightTheme}
                            queryChangesetApplyPreview={queryChangesetApplyPreview}
                            queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
                            expandChangesetDescriptions={expandChangesetDescriptions}
                        />
                    </BatchChangeTabPanel>
                    <BatchChangeTabPanel>
                        <BatchSpecTab originalInput={spec.originalInput} />
                    </BatchChangeTabPanel>
                </BatchChangeTabPanels>
            </BatchChangeTabs>
        </div>
    )
}
