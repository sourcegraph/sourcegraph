import * as H from 'history'
import React, { useEffect, useMemo } from 'react'
import { useObservable } from '../../../../../shared/src/util/useObservable'
import { delay, distinctUntilChanged, repeatWhen } from 'rxjs/operators'
import { isEqual } from 'lodash'
import { PageTitle } from '../../../components/PageTitle'
import { fetchBatchSpecById as _fetchBatchSpecById } from './backend'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { PreviewList } from './list/PreviewList'
import { ThemeProps } from '../../../../../shared/src/theme'
import { CreateUpdateBatchChangeAlert } from './CreateUpdateBatchChangeAlert'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import { HeroPage } from '../../../components/HeroPage'
import { Description } from '../Description'
import { BatchSpecInfoByline } from './BatchSpecInfoByline'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import { AuthenticatedUser } from '../../../auth'
import { MissingCredentialsAlert } from './MissingCredentialsAlert'
import { SupersedingBatchSpecAlert } from '../detail/SupersedingBatchSpecAlert'
import { queryChangesetSpecFileDiffs, queryChangesetApplyPreview } from './list/backend'
import { BatchChangePreviewStatsBar } from './BatchChangePreviewStatsBar'
import { PageHeader } from '../../../components/PageHeader'
import { BatchChangesIcon } from '../icons'

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
        telemetryService.logViewEvent('CampaignApplyPage')
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
                batchChange={spec.appliesToBatchChange}
                viewerCanAdminister={spec.viewerCanAdminister}
                telemetryService={telemetryService}
            />
            <Description history={history} description={spec.description.description} />
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
        </div>
    )
}
