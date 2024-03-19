import { type FC, useMemo } from 'react'

import VisuallyHidden from '@reach/visually-hidden'

import { pluralize } from '@sourcegraph/common'
import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { PageHeader, H2, useObservable, Text, H4 } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../auth'
import { BatchChangesIcon } from '../../../batches/icons'
import { canWriteBatchChanges, NO_ACCESS_BATCH_CHANGES_WRITE, NO_ACCESS_SOURCEGRAPH_COM } from '../../../batches/utils'
import { DiffStat } from '../../../components/diff/DiffStat'
import { Page } from '../../../components/Page'
import { PageTitle } from '../../../components/PageTitle'
import type { RepositoryFields, RepoBatchChangeStats } from '../../../graphql-operations'
import type { queryExternalChangesetWithFileDiffs as _queryExternalChangesetWithFileDiffs } from '../detail/backend'
import { BatchChangeStatsTotalAction } from '../detail/BatchChangeStatsCard'
import {
    ChangesetStatusUnpublished,
    ChangesetStatusOpen,
    ChangesetStatusClosed,
    ChangesetStatusMerged,
} from '../detail/changesets/ChangesetStatusCell'
import { NewBatchChangeButton } from '../list/NewBatchChangeButton'

import {
    type queryRepoBatchChanges as _queryRepoBatchChanges,
    queryRepoBatchChangeStats as _queryRepoBatchChangeStats,
} from './backend'
import { RepoBatchChanges } from './RepoBatchChanges'

interface BatchChangeRepoPageProps extends TelemetryV2Props {
    repo: RepositoryFields
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    /** For testing only. */
    queryRepoBatchChangeStats?: typeof _queryRepoBatchChangeStats
    /** For testing only. */
    queryRepoBatchChanges?: typeof _queryRepoBatchChanges
    /** For testing only. */
    queryExternalChangesetWithFileDiffs?: typeof _queryExternalChangesetWithFileDiffs
}

export const BatchChangeRepoPage: FC<BatchChangeRepoPageProps> = ({
    repo,
    isSourcegraphDotCom,
    authenticatedUser,
    queryRepoBatchChangeStats = _queryRepoBatchChangeStats,
    ...props
}) => {
    const repoDisplayName = displayRepoName(repo.name)

    const stats: RepoBatchChangeStats | undefined = useObservable(
        useMemo(() => queryRepoBatchChangeStats({ name: repo.name }), [queryRepoBatchChangeStats, repo.name])
    )
    const hasChangesets = stats?.changesetsStats.total

    const canCreate: true | string = useMemo(() => {
        if (isSourcegraphDotCom) {
            return NO_ACCESS_SOURCEGRAPH_COM
        }
        if (!canWriteBatchChanges(authenticatedUser)) {
            return NO_ACCESS_BATCH_CHANGES_WRITE
        }
        return true
    }, [isSourcegraphDotCom, authenticatedUser])

    return (
        <Page>
            <PageTitle title="Batch Changes" />
            <PageHeader
                path={[{ icon: BatchChangesIcon, text: 'Batch Changes' }]}
                headingElement="h1"
                actions={
                    <NewBatchChangeButton
                        to="/batch-changes/create"
                        canCreate={canCreate}
                        telemetryRecorder={props.telemetryRecorder}
                    />
                }
                description={
                    hasChangesets
                        ? undefined
                        : 'Run custom code over this repository and others, and manage the resulting changesets.'
                }
            />
            {hasChangesets && stats?.batchChangesDiffStat && stats?.changesetsStats ? (
                <div className="d-flex align-items-center mt-4 mb-3">
                    <H2 className="mb-0 pb-1">{repoDisplayName}</H2>
                    <DiffStat className="d-flex flex-1 ml-2" expandedCounts={true} {...stats.batchChangesDiffStat} />
                    <StatsBar stats={stats.changesetsStats} />
                </div>
            ) : null}
            {hasChangesets ? (
                <Text>
                    Batch changes has created {stats?.changesetsStats.total} changesets on {repoDisplayName}
                </Text>
            ) : (
                <div className="mb-3" />
            )}
            <RepoBatchChanges
                isSourcegraphDotCom={isSourcegraphDotCom}
                viewerCanAdminister={true}
                repo={repo}
                canCreate={canCreate}
                {...props}
            />
        </Page>
    )
}

const ACTION_CLASSNAMES = 'd-flex flex-column text-muted justify-content-center align-items-center mx-2'

interface StatsBarProps {
    stats: RepoBatchChangeStats['changesetsStats']
}

const StatsBar: React.FunctionComponent<React.PropsWithChildren<StatsBarProps>> = ({
    stats: { total, draft, open, unpublished, closed, merged },
}) => (
    <div className="d-flex flex-wrap align-items-center">
        <BatchChangeStatsTotalAction count={total} />
        <ChangesetStatusOpen
            className={ACTION_CLASSNAMES}
            label={
                <H4 className="font-weight-normal text-muted m-0">
                    {draft + open} <VisuallyHidden>{pluralize('changeset', draft + open)}</VisuallyHidden> open
                </H4>
            }
        />
        <ChangesetStatusUnpublished
            className={ACTION_CLASSNAMES}
            label={
                <H4 className="font-weight-normal text-muted m-0">
                    {unpublished} <VisuallyHidden>{pluralize('changeset', unpublished)}</VisuallyHidden> unpublished
                </H4>
            }
        />
        <ChangesetStatusClosed
            className={ACTION_CLASSNAMES}
            label={
                <H4 className="font-weight-normal text-muted m-0">
                    {closed} <VisuallyHidden>{pluralize('changeset', closed)}</VisuallyHidden> closed
                </H4>
            }
        />
        <ChangesetStatusMerged
            className={ACTION_CLASSNAMES}
            label={
                <H4 className="font-weight-normal text-muted m-0">
                    {merged} <VisuallyHidden>{pluralize('changeset', merged)}</VisuallyHidden> merged
                </H4>
            }
        />
    </div>
)
