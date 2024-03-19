import React, { useCallback, useContext, useState } from 'react'

import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { isErrorLike } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Alert, Button, Code, Link, Tooltip, ErrorAlert } from '@sourcegraph/wildcard'

import type { BatchSpecFields } from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'
import { MultiSelectContext } from '../MultiSelectContext'

import { applyBatchChange, createBatchChange } from './backend'
import { BatchChangePreviewContext } from './BatchChangePreviewContext'

import styles from './CreateUpdateBatchChangeAlert.module.scss'

export interface CreateUpdateBatchChangeAlertProps extends TelemetryProps, TelemetryV2Props {
    specID: string
    toBeArchived: number
    batchChange: BatchSpecFields['appliesToBatchChange']
    viewerCanAdminister: boolean
}

export const CreateUpdateBatchChangeAlert: React.FunctionComponent<
    React.PropsWithChildren<CreateUpdateBatchChangeAlertProps>
> = ({ specID, toBeArchived, batchChange, viewerCanAdminister, telemetryService, telemetryRecorder }) => {
    const navigate = useNavigate()

    const batchChangeID = batchChange?.id

    // `BatchChangePreviewContext` is responsible for managing the overrideable
    // publication states for preview changesets on the clientside.
    const { publicationStates } = useContext(BatchChangePreviewContext)
    const { selected } = useContext(MultiSelectContext)

    const [isLoading, setIsLoading] = useState<boolean | Error>(false)

    const canApply = selected !== 'all' && selected.size === 0 && !isLoading && viewerCanAdminister

    // Returns a tooltip error message appropriate for the given circumstances preventing
    // the user from applying the preview.
    const disabledTooltip = useCallback((): string | undefined => {
        if (canApply) {
            return undefined
        }
        if (!viewerCanAdminister) {
            return "You don't have permission to apply this batch change."
        }
        if (selected === 'all' || selected.size > 0) {
            return 'You have selected changesets. Choose the action to be performed on apply or deselect to continue.'
        }
        return undefined
    }, [canApply, selected, viewerCanAdminister])

    const onApply = useCallback(async () => {
        if (!canApply) {
            return
        }
        if (!confirm(`Are you sure you want to ${batchChangeID ? 'update' : 'create'} this batch change?`)) {
            return
        }
        setIsLoading(true)
        try {
            const batchChange = batchChangeID
                ? await applyBatchChange({ batchSpec: specID, batchChange: batchChangeID, publicationStates })
                : await createBatchChange({ batchSpec: specID, publicationStates })

            if (toBeArchived > 0) {
                navigate(`${batchChange.url}?archivedCount=${toBeArchived}&archivedBy=${specID}`)
            } else {
                navigate(batchChange.url)
            }
            telemetryService.logViewEvent(`BatchChangeDetailsPageAfter${batchChangeID ? 'Create' : 'Update'}`)
            if (batchChangeID) {
                telemetryRecorder.recordEvent('batchChange.detailsAfterUpdate', 'view')
            } else {
                telemetryRecorder.recordEvent('batchChange.detailsAfterCreate', 'view')
            }
        } catch (error) {
            setIsLoading(error)
        }
    }, [
        canApply,
        specID,
        setIsLoading,
        navigate,
        batchChangeID,
        telemetryService,
        telemetryRecorder,
        toBeArchived,
        publicationStates,
    ])

    return (
        <>
            <Alert className="mb-3 d-block d-md-flex align-items-center body-lead" variant="info" aria-live="off">
                <div className={classNames(styles.createUpdateBatchChangeAlertCopy, 'flex-grow-1 mr-3')}>
                    {batchChange ? (
                        <>
                            This operation will update the existing batch change{' '}
                            <Link to={batchChange.url}>{batchChange.name}</Link>.
                        </>
                    ) : (
                        'Review the proposed changesets below.'
                    )}{' '}
                    Click 'Apply' or run <Code>src batch apply</Code> against your batch spec to{' '}
                    {batchChange ? 'update' : 'create'} the batch change and perform the indicated action on each
                    changeset. Select a changeset and modify the action to customize the publication state of each or
                    all changesets.
                </div>
                <div className={styles.createUpdateBatchChangeAlertBtn}>
                    <Tooltip content={disabledTooltip()}>
                        <Button
                            variant="primary"
                            className={classNames(
                                'test-batches-confirm-apply-btn text-nowrap',
                                isLoading === true || (!viewerCanAdminister && 'disabled')
                            )}
                            onClick={() => {
                                if (batchChange) {
                                    eventLogger.log('batch_change_execution_preview:apply_update:clicked')
                                    telemetryRecorder.recordEvent('batchChange.execution.updateAndApply', 'click')
                                } else {
                                    eventLogger.log('batch_change_execution_preview:apply:clicked')
                                    telemetryRecorder.recordEvent('batchChange.execution.createAndApply', 'click')
                                }
                                return onApply()
                            }}
                            disabled={!canApply}
                        >
                            Apply
                        </Button>
                    </Tooltip>
                </div>
            </Alert>
            {isErrorLike(isLoading) && <ErrorAlert error={isLoading} />}
        </>
    )
}
