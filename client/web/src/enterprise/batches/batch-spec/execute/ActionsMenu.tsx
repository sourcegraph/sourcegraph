import React, { useCallback, useState } from 'react'

import { mdiChevronDown, mdiAlertCircle, mdiPencil, mdiClose, mdiSync, mdiCloseCircleOutline } from '@mdi/js'
import { noop } from 'lodash'
import { useNavigate, useLocation } from 'react-router-dom'

import { useMutation } from '@sourcegraph/http-client'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Button,
    Icon,
    Link,
    Menu,
    MenuButton,
    MenuItem,
    MenuLink,
    MenuList,
    Position,
    Text,
    useMeasure,
} from '@sourcegraph/wildcard'

import {
    type BatchSpecExecutionFields,
    BatchSpecState,
    type CancelBatchSpecExecutionResult,
    type CancelBatchSpecExecutionVariables,
    type RetryBatchSpecExecutionResult,
    type RetryBatchSpecExecutionVariables,
} from '../../../../graphql-operations'
import { eventLogger } from '../../../../tracking/eventLogger'
import { type BatchSpecContextState, useBatchSpecContext } from '../BatchSpecContext'

import { CANCEL_BATCH_SPEC_EXECUTION, RETRY_BATCH_SPEC_EXECUTION } from './backend'
import { CancelExecutionModal } from './CancelExecutionModal'

import styles from './ActionsMenu.module.scss'

export enum ActionsMenuMode {
    Preview = 'PREVIEW',
    Actions = 'ACTIONS',
    ActionsOnlyClose = 'CLOSE',
    ActionsWithPreview = 'ACTIONS_WITH_PREVIEW',
}

export interface ActionsMenuProps extends TelemetryV2Props {
    defaultMode?: ActionsMenuMode
}

export const ActionsMenu: React.FunctionComponent<React.PropsWithChildren<ActionsMenuProps>> = ({
    defaultMode,
    telemetryRecorder,
}) => {
    const { batchChange, batchSpec, setActionsError } = useBatchSpecContext<BatchSpecExecutionFields>()

    return (
        <MemoizedActionsMenu
            batchChange={batchChange}
            batchSpec={batchSpec}
            setActionsError={setActionsError}
            defaultMode={defaultMode}
            telemetryRecorder={telemetryRecorder}
        />
    )
}

const MemoizedActionsMenu: React.FunctionComponent<
    React.PropsWithChildren<
        Pick<BatchSpecContextState, 'batchChange' | 'batchSpec' | 'setActionsError'> & ActionsMenuProps
    >
> = React.memo(function MemoizedActionsMenu({
    batchChange,
    batchSpec,
    setActionsError,
    defaultMode,
    telemetryRecorder,
}) {
    const navigate = useNavigate()
    const location = useLocation()

    const { url } = batchChange
    const { isExecuting, state } = batchSpec

    const [showCancelModal, setShowCancelModal] = useState(false)
    const [cancelModalType, setCancelModalType] = useState<'cancel' | 'edit'>('cancel')
    const [cancelBatchSpecExecution, { loading: isCancelLoading }] = useMutation<
        CancelBatchSpecExecutionResult,
        CancelBatchSpecExecutionVariables
    >(CANCEL_BATCH_SPEC_EXECUTION, {
        variables: { id: batchSpec.id },
        onError: setActionsError,
        onCompleted: () => setShowCancelModal(false),
    })

    const cancelAndEdit = useCallback(() => {
        cancelBatchSpecExecution()
            .then(() => navigate(`${url}/edit`))
            .catch(noop)
    }, [cancelBatchSpecExecution, navigate, url])

    const [retryBatchSpecExecution, { loading: isRetryLoading }] = useMutation<
        RetryBatchSpecExecutionResult,
        RetryBatchSpecExecutionVariables
    >(RETRY_BATCH_SPEC_EXECUTION, { variables: { id: batchSpec.id }, onError: setActionsError })

    const onSelectEdit = useCallback(() => {
        if (isExecuting) {
            setCancelModalType('edit')
            setShowCancelModal(true)
        } else {
            navigate(`${url}/edit`)
        }
    }, [isExecuting, url, navigate])

    const onSelectCancel = useCallback(() => {
        setCancelModalType('cancel')
        setShowCancelModal(true)
    }, [])

    // We have to figure out exactly what menu to show. Our options are:
    //
    // Actions:            the default actions menu.
    // ActionsOnlyClose:   a visually identical actions menu, but with _only_
    //                     an option to close the batch change.
    // ActionsWithPreview: the default actions menu, _plus_ an item to preview
    //                     with errors.
    // Preview:            a single Preview CTA primary button, with no dropdown
    //                     menu.
    //
    // Most of the time, this component is used within the ExecuteBatchSpecPage,
    // in which case we can parse location.pathname to figure out what page
    // we're on, and combine that with the batch spec state to decide what to
    // display. Here's that set of possible states:
    //
    // | Page           | State     | Mode               |
    // |----------------|-----------|--------------------|
    // | configuration  | pending   | ActionsOnlyClose   |
    // | configuration  | any       | Actions            |
    // | spec           | completed | Preview            |
    // | spec           | failed    | ActionsWithPreview |
    // | spec           | other     | Actions            |
    // | execution      | completed | Preview            |
    // | execution      | failed    | ActionsWithPreview |
    // | execution      | other     | Actions            |
    // | preview        | any       | Actions            |
    //
    // If none of these match, then the optional defaultMode property will be
    // used. If that isn't provided, the we'll just fall back to Actions.
    let mode = defaultMode ?? ActionsMenuMode.Actions
    if (location.pathname.endsWith('configuration') && state === BatchSpecState.PENDING) {
        mode = ActionsMenuMode.ActionsOnlyClose
    } else if (location.pathname.endsWith('preview')) {
        mode = ActionsMenuMode.Actions
    } else if (state === BatchSpecState.COMPLETED) {
        mode = ActionsMenuMode.Preview
    } else if (state === BatchSpecState.FAILED) {
        mode = ActionsMenuMode.ActionsWithPreview
    }

    // The actions menu button is wider than the "Preview" button, so to prevent layout
    // shift, we apply the width of the actions menu button to the "Preview" button
    // instead.
    const [menuReference, { width: menuWidth }] = useMeasure()

    return (
        <div className="position-relative">
            {mode === ActionsMenuMode.Preview && (
                <Button
                    to={`${batchSpec.executionURL}/preview`}
                    variant="primary"
                    as={Link}
                    className={styles.previewButton}
                    style={{ width: menuWidth }}
                    onClick={() => {
                        eventLogger.log('batch_change_execution:preview:clicked')
                        telemetryRecorder.recordEvent('batchChange.execution', 'preview')
                    }}
                >
                    Preview
                </Button>
            )}
            <Menu>
                <div className="d-inline-block" ref={menuReference} aria-hidden={mode === ActionsMenuMode.Preview}>
                    <MenuButton
                        variant="secondary"
                        className={mode === ActionsMenuMode.Preview ? styles.menuButtonHidden : undefined}
                        // If an element with aria-hidden={true} contains a focusable
                        // element, assistive technologies won't read the focusable
                        // element, but keyboard users will still be able to navigate to
                        // it, which can cause confusion. We pair this with negative tab
                        // index to take the menu button out of the tab order when it is
                        // hidden. See: https://web.dev/aria-hidden-focus/
                        tabIndex={mode === ActionsMenuMode.Preview ? -1 : undefined}
                    >
                        Actions
                        <Icon aria-hidden={true} className={styles.chevronIcon} svgPath={mdiChevronDown} />
                    </MenuButton>
                </div>
                <MenuList position={Position.bottomEnd}>
                    {mode !== ActionsMenuMode.ActionsOnlyClose && (
                        <>
                            {mode === ActionsMenuMode.ActionsWithPreview && (
                                <MenuItem onSelect={() => navigate(`${batchSpec.executionURL}/preview`)}>
                                    <Icon aria-hidden={true} svgPath={mdiAlertCircle} /> Preview with errors
                                </MenuItem>
                            )}
                            <MenuItem onSelect={onSelectEdit}>
                                <Icon aria-hidden={true} svgPath={mdiPencil} /> Edit spec{isExecuting ? '...' : ''}
                            </MenuItem>
                            {isExecuting && (
                                <MenuItem onSelect={onSelectCancel}>
                                    <Icon aria-hidden={true} className={styles.cancelIcon} svgPath={mdiClose} /> Cancel
                                    execution...
                                </MenuItem>
                            )}
                            {batchSpec.viewerCanRetry && (
                                <MenuItem onSelect={retryBatchSpecExecution} disabled={isRetryLoading}>
                                    <Icon aria-hidden={true} svgPath={mdiSync} /> Retry failed workspaces
                                </MenuItem>
                            )}
                        </>
                    )}
                    <MenuLink as={Link} to={`${batchChange.url}/close`}>
                        <Icon aria-hidden={true} className={styles.cancelIcon} svgPath={mdiCloseCircleOutline} /> Close
                        batch change
                    </MenuLink>
                </MenuList>
            </Menu>
            <CancelExecutionModal
                isOpen={showCancelModal}
                onCancel={() => setShowCancelModal(false)}
                onConfirm={
                    cancelModalType === 'cancel'
                        ? () => {
                              eventLogger.log('batch_change_execution:actions_execution_cancel:clicked')
                              telemetryRecorder.recordEvent('batchChange.execution', 'cancel')
                              return cancelBatchSpecExecution()
                          }
                        : () => {
                              eventLogger.log('batch_change_execution:actions_execution_cancel_and_edit:clicked')
                              telemetryRecorder.recordEvent('batchChange.execution', 'edit')
                              return cancelAndEdit()
                          }
                }
                modalHeader={cancelModalType === 'cancel' ? 'Cancel execution' : 'The execution is still running'}
                modalBody={
                    <Text>
                        {cancelModalType === 'cancel'
                            ? 'Are you sure you want to cancel the current execution?'
                            : 'You are unable to edit the spec when an execution is running.'}
                    </Text>
                }
                isLoading={isCancelLoading}
            />
        </div>
    )
})
