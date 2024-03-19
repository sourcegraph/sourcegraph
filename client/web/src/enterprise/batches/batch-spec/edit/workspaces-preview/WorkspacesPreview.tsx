import React, { useEffect, useMemo, useState } from 'react'

import { mdiAlert, mdiMagnify } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'
import { animated, useSpring } from 'react-spring'

import { CodeSnippet } from '@sourcegraph/branded/src/components/CodeSnippet'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Alert, Button, H4, Icon, Tooltip, useAccordion, useStopwatch, ErrorAlert } from '@sourcegraph/wildcard'

import type { Connection } from '../../../../../components/FilteredConnection'
import {
    BatchSpecWorkspaceResolutionState,
    type PreviewHiddenBatchSpecWorkspaceFields,
    type PreviewVisibleBatchSpecWorkspaceFields,
} from '../../../../../graphql-operations'
import { eventLogger } from '../../../../../tracking/eventLogger'
import { useBatchChangesLicense } from '../../../useBatchChangesLicense'
import { Header as WorkspacesListHeader } from '../../../workspaces-list'
import { type BatchSpecContextState, useBatchSpecContext } from '../../BatchSpecContext'

import { ImportingChangesetsPreviewList } from './ImportingChangesetsPreviewList'
import { PreviewLoadingSpinner } from './PreviewLoadingSpinner'
import { PreviewPromptIcon } from './PreviewPromptIcon'
import { useImportingChangesets } from './useImportingChangesets'
import { useWorkspaces } from './useWorkspaces'
import { WorkspacePreviewFilterRow } from './WorkspacesPreviewFilterRow'
import { WorkspacesPreviewList } from './WorkspacesPreviewList'

import styles from './WorkspacesPreview.module.scss'

/** Example snippet show in preview prompt if user has not yet added an on: statement. */
const ON_STATEMENT = `on:
  - repositoriesMatchingQuery: repo:my-org/.*
`

const WAITING_MESSAGES = [
    'Hang tight while we look for matching workspaces...',
    'Still searching, this should just take a moment or two...',
    '*elevator music* (Still looking for matching workspaces...)',
    'The search continues...',
    'Reticulating splines... (Still looking for matching workspaces...)',
    "So, how's your day? (Still looking for matching workspaces...)",
    'Are you staying hydrated? (Still looking for matching workspaces...)',
    "Hold your horses, we're still not done yet...",
    "A Go developer walks into a bar and tries to defer their bill, but they can't start a tab. (Sorry.)",
]

/* The time to wait until we display the next waiting message, in seconds. */
const WAITING_MESSAGE_INTERVAL = 10

/** The minimum number of resolved workspaces at which we'll show a warning
 * about Batch Changes performance. */
const WORKSPACE_WARNING_MIN_TOTAL_COUNT = 2000

interface WorkspacesPreviewProps extends TelemetryV2Props {
    isReadOnly?: boolean
}

export const WorkspacesPreview: React.FunctionComponent<React.PropsWithChildren<WorkspacesPreviewProps>> = ({
    isReadOnly = false,
    telemetryRecorder,
}) => {
    const { batchSpec, editor, workspacesPreview } = useBatchSpecContext()

    return (
        <MemoizedWorkspacesPreview
            batchSpec={batchSpec}
            editor={editor}
            workspacesPreview={workspacesPreview}
            isReadOnly={isReadOnly}
            telemetryRecorder={telemetryRecorder}
        />
    )
}

type MemoizedWorkspacesPreviewProps = WorkspacesPreviewProps &
    Pick<BatchSpecContextState, 'batchSpec' | 'editor' | 'workspacesPreview'>

const MemoizedWorkspacesPreview: React.FunctionComponent<React.PropsWithChildren<MemoizedWorkspacesPreviewProps>> =
    React.memo(function MemoizedWorkspacesPreview({
        isReadOnly,
        batchSpec,
        editor,
        workspacesPreview,
        telemetryRecorder,
    }) {
        const { debouncedCode, excludeRepo, isServerStale } = editor
        const {
            resolutionState,
            error,
            filters,
            setFilters,
            cancel,
            isInProgress: isWorkspacesPreviewInProgress,
            hasPreviewed,
            preview,
            isPreviewDisabled,
            noCache: cacheDisabled,
        } = workspacesPreview

        const workspacesConnection = useWorkspaces(batchSpec.id, filters)
        const importingChangesetsConnection = useImportingChangesets(batchSpec.id)
        const connection = workspacesConnection.connection

        // Before we've ever previewed workspaces for this batch change, there's no reason to
        // show the list.
        const shouldShowConnection = hasPreviewed || !!connection?.nodes.length

        // We "cache" the last results of the workspaces preview so that we can continue to
        // show them in the list while the next workspaces resolution is still in progress. We
        // have to do this outside of Apollo Client because we continue to requery the
        // workspaces preview while the resolution job is still in progress, and so the
        // results will come up empty and overwrite the previous results in the Apollo Client
        // cache while this is happening.
        const [cachedWorkspacesPreview, setCachedWorkspacesPreview] =
            useState<Connection<PreviewHiddenBatchSpecWorkspaceFields | PreviewVisibleBatchSpecWorkspaceFields>>()

        // We copy the results from `connection` to `cachedWorkspacesPreview` whenever a
        // resolution job completes.
        useEffect(() => {
            if (resolutionState === BatchSpecWorkspaceResolutionState.COMPLETED && connection?.nodes.length) {
                setCachedWorkspacesPreview(connection)
            }
        }, [resolutionState, connection])

        // We will instruct `WorkspacesPreviewList` to show the cached results instead of
        // whatever is in `connection` if we know the workspaces preview resolution is
        // currently in progress.
        const showCached = useMemo(
            () =>
                Boolean(
                    cachedWorkspacesPreview?.nodes.length &&
                        (isWorkspacesPreviewInProgress || resolutionState === 'CANCELED')
                ),
            [cachedWorkspacesPreview, isWorkspacesPreviewInProgress, resolutionState]
        )

        // We time the preview so that we can show a changing message to the user the longer
        // they have to wait.
        const { time, start, stop, isRunning } = useStopwatch(false)
        useEffect(() => {
            if (isWorkspacesPreviewInProgress) {
                start()
            } else {
                stop()
            }
        }, [isWorkspacesPreviewInProgress, start, stop])

        // We use the same `<Button />` and just swap props so that we keep the same element
        // hierarchy when the preview is in progress as when it is not. We do this in order to
        // maintain focus on the button between state changes.
        const ctaButton = useMemo(
            () => (
                <Tooltip
                    content={
                        !isWorkspacesPreviewInProgress && typeof isPreviewDisabled === 'string'
                            ? isPreviewDisabled
                            : undefined
                    }
                >
                    <Button
                        variant={isWorkspacesPreviewInProgress ? 'secondary' : 'success'}
                        onClick={
                            isWorkspacesPreviewInProgress
                                ? cancel
                                : () => {
                                      eventLogger.log('batch_change_editor:preview_workspaces:clicked')
                                      telemetryRecorder.recordEvent('batchChange.editor.previewWorkspaces', 'click')
                                      return preview(debouncedCode)
                                  }
                        }
                        // The "Cancel" button is always enabled while the preview is in progress
                        disabled={!isWorkspacesPreviewInProgress && !!isPreviewDisabled}
                    >
                        {!isWorkspacesPreviewInProgress && (
                            <Icon aria-hidden={true} className="mr-1" svgPath={mdiMagnify} />
                        )}
                        {isWorkspacesPreviewInProgress ? 'Cancel' : error ? 'Retry preview' : 'Preview workspaces'}
                    </Button>
                </Tooltip>
            ),
            [isWorkspacesPreviewInProgress, isPreviewDisabled, cancel, preview, debouncedCode, error, telemetryRecorder]
        )

        const [exampleReference, exampleOpen, setExampleOpen, exampleStyle] = useAccordion()

        const ctaInstructions = isWorkspacesPreviewInProgress ? (
            // We render all of the waiting messages at once on top of each other so that we
            // can animate from one to the next.
            <div className={styles.waitingMessageContainer}>
                {WAITING_MESSAGES.map((message, index) => {
                    const active =
                        Math.floor((isRunning ? time.seconds : 0) / WAITING_MESSAGE_INTERVAL) %
                            WAITING_MESSAGES.length ===
                        index
                    return (
                        <CTAInstruction active={active} key={message}>
                            {message}
                        </CTAInstruction>
                    )
                })}
            </div>
        ) : isServerStale ? (
            <H4 className={styles.instruction}>Finish editing your batch spec, then manually preview repositories.</H4>
        ) : (
            <>
                <H4 className={classNames(styles.instruction, styles.exampleOnStatement)}>
                    {hasPreviewed ? 'Modify your' : 'Add an'}
                    <span className="text-monospace mx-1">on:</span> statement to preview repositories.
                    {!hasPreviewed && (
                        <div className={styles.toggleExampleButtonContainer}>
                            <Button className={styles.toggleExampleButton} onClick={() => setExampleOpen(!exampleOpen)}>
                                {exampleOpen ? 'Close example' : 'See example'}
                            </Button>
                        </div>
                    )}
                </H4>
                <animated.div style={exampleStyle} className={styles.onExample} aria-hidden={!exampleOpen}>
                    <div ref={exampleReference} className="pt-2 pb-3">
                        {/* Hide the copy button while the example is closed so that it's not focusable. */}
                        <CodeSnippet
                            className="w-100 m-0"
                            code={ON_STATEMENT}
                            language="yaml"
                            withCopyButton={exampleOpen}
                        />
                    </div>
                </animated.div>
            </>
        )

        const [visibleCount, totalCount] = useMemo<[number, number] | [null, null]>(() => {
            if (shouldShowConnection) {
                // Show cached count when showCached is true AND the connection has no data. Else, the count will not match
                // the actual.
                if (cachedWorkspacesPreview && showCached && !connection?.nodes.length) {
                    return [cachedWorkspacesPreview.nodes.length, cachedWorkspacesPreview.totalCount ?? 0]
                }
                if (connection) {
                    return [connection.nodes.length, connection.totalCount ?? 0]
                }
            }
            return [null, null]
        }, [shouldShowConnection, showCached, cachedWorkspacesPreview, connection])

        const totalCountDisplay = useMemo(
            () =>
                visibleCount !== null && totalCount !== null ? (
                    <span className={styles.totalCount}>
                        Displaying {visibleCount} of {totalCount}
                    </span>
                ) : null,
            [visibleCount, totalCount]
        )

        const { maxUnlicensedChangesets, exceedsLicense } = useBatchChangesLicense()

        return (
            <div className={styles.container}>
                <WorkspacesListHeader>
                    <span>Workspaces {isReadOnly ? '' : 'preview '}</span>
                    {(isServerStale || resolutionState === 'CANCELED' || !hasPreviewed) &&
                        shouldShowConnection &&
                        !isWorkspacesPreviewInProgress &&
                        !isReadOnly && (
                            <Tooltip content="The workspaces previewed below may not be up-to-date.">
                                <Icon
                                    aria-label="The workspaces previewed below may not be up-to-date."
                                    className={classNames('text-muted ml-1', styles.warningIcon)}
                                    svgPath={mdiAlert}
                                />
                            </Tooltip>
                        )}
                    {totalCountDisplay}
                </WorkspacesListHeader>
                {/* We wrap this section in its own div to prevent margin collapsing within the flex column */}
                {!isReadOnly && (
                    <div className="d-flex flex-column align-items-center w-100 mb-3">
                        {error && <ErrorAlert error={error} className="w-100 mb-0" />}
                        <VisuallyHidden role="status">
                            {isWorkspacesPreviewInProgress ? 'Preview loading' : ''}
                        </VisuallyHidden>
                        <div className={styles.iconContainer} aria-hidden={true}>
                            <PreviewLoadingSpinner
                                className={classNames({ [styles.hidden]: !isWorkspacesPreviewInProgress })}
                            />
                            <PreviewPromptIcon
                                className={classNames({ [styles.hidden]: isWorkspacesPreviewInProgress })}
                            />
                        </div>
                        {ctaInstructions}
                        {ctaButton}
                    </div>
                )}
                {totalCount !== null && totalCount >= WORKSPACE_WARNING_MIN_TOTAL_COUNT && (
                    <div className="d-flex flex-column align-items-center w-100">
                        <CTASizeWarning />
                    </div>
                )}
                {totalCount !== null &&
                    exceedsLicense(totalCount + (importingChangesetsConnection?.connection?.totalCount ?? 0)) && (
                        <div className="d-flex flex-column align-items-center w-100">
                            <CTALicenseWarning maxCount={maxUnlicensedChangesets} />
                        </div>
                    )}
                {(hasPreviewed || isReadOnly) && (
                    <WorkspacePreviewFilterRow onFiltersChange={setFilters} disabled={isWorkspacesPreviewInProgress} />
                )}
                {shouldShowConnection && (
                    <div className="overflow-auto w-100">
                        <WorkspacesPreviewList
                            isStale={
                                isServerStale ||
                                isWorkspacesPreviewInProgress ||
                                resolutionState === 'CANCELED' ||
                                !hasPreviewed
                            }
                            excludeRepo={excludeRepo}
                            workspacesConnection={workspacesConnection}
                            showCached={showCached}
                            cached={cachedWorkspacesPreview}
                            isReadOnly={isReadOnly}
                            cacheDisabled={cacheDisabled}
                        />
                        <ImportingChangesetsPreviewList
                            isStale={
                                isServerStale ||
                                isWorkspacesPreviewInProgress ||
                                resolutionState === 'CANCELED' ||
                                !hasPreviewed
                            }
                            importingChangesetsConnection={importingChangesetsConnection}
                        />
                    </div>
                )}
            </div>
        )
    })

const CTAInstruction: React.FunctionComponent<React.PropsWithChildren<{ active: boolean }>> = ({
    active,
    children,
}) => {
    // We use 3rem for the height, which is intentionally bigger than the parent (2rem) so
    // that if text is forced to wrap, it isn't cut off.
    const style = useSpring({ height: active ? '3rem' : '0rem', opacity: active ? 1 : 0 })
    return (
        <animated.div style={style}>
            <H4 className={classNames(styles.instruction, styles.waitingText)}>{children}</H4>
        </animated.div>
    )
}

const CTASizeWarning: React.FunctionComponent = () => (
    <Alert variant="note" className="mb-2">
        The experience is currently optimized for batch changes targeting up to {WORKSPACE_WARNING_MIN_TOTAL_COUNT}{' '}
        workspaces. Break your batch change down into several smaller batch changes for a better experience.
    </Alert>
)

const CTALicenseWarning: React.FunctionComponent<React.PropsWithChildren<{ maxCount: number }>> = ({ maxCount }) => (
    <Alert variant="note" className="mb-2">
        Your license only allows for {maxCount} changesets per batch change. If more than {maxCount} changesets are
        generated, you won't be able to apply the batch change and actually publish the changesets to the code host.
    </Alert>
)
