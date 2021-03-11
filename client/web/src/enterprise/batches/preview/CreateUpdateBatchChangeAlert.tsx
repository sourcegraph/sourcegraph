import * as H from 'history'
import React, { useCallback, useState } from 'react'
import { BatchSpecFields } from '../../../graphql-operations'
import { createBatchChange, applyBatchChange } from './backend'
import { Link } from '../../../../../shared/src/components/Link'
import classNames from 'classnames'
import { isErrorLike } from '../../../../../shared/src/util/errors'
import { ErrorAlert } from '../../../components/alerts'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import InfoCircleOutlineIcon from 'mdi-react/InfoCircleOutlineIcon'

export interface CreateUpdateBatchChangeAlertProps extends TelemetryProps {
    specID: string
    batchChange: BatchSpecFields['appliesToBatchChange']
    viewerCanAdminister: boolean
    history: H.History
}

export const CreateUpdateBatchChangeAlert: React.FunctionComponent<CreateUpdateBatchChangeAlertProps> = ({
    specID,
    batchChange,
    viewerCanAdminister,
    history,
    telemetryService,
}) => {
    const batchChangeID = batchChange?.id
    const [isLoading, setIsLoading] = useState<boolean | Error>(false)
    const onApply = useCallback(async () => {
        if (!confirm(`Are you sure you want to ${batchChangeID ? 'update' : 'create'} this batch change?`)) {
            return
        }
        setIsLoading(true)
        try {
            const batchChange = batchChangeID
                ? await applyBatchChange({ batchSpec: specID, batchChange: batchChangeID })
                : await createBatchChange({ batchSpec: specID })
            history.push(batchChange.url)
            telemetryService.logViewEvent(`CampaignDetailsPageAfter${batchChangeID ? 'Create' : 'Update'}`)
        } catch (error) {
            setIsLoading(error)
        }
    }, [specID, setIsLoading, history, batchChangeID, telemetryService])
    return (
        <>
            <div className="alert alert-info p-3 mb-3 d-block d-md-flex align-items-center body-lead">
                <div className="flex-grow-1 mr-3 create-update-batch-change-alert__copy">
                    <InfoCircleOutlineIcon className="icon-inline mr-2" />
                    {!batchChange && (
                        <>
                            Review the proposed changesets below. Click 'Apply spec' or run <code>src batch apply</code>{' '}
                            against your batch spec to create the batch change and perform the indicated action on each
                            changeset.
                        </>
                    )}
                    {batchChange && (
                        <>
                            This operation will update the existing batch change{' '}
                            <Link to={batchChange.url}>{batchChange.name}</Link>. Click 'Apply spec' or run{' '}
                            <code>src batch apply</code> against your batch spec to update the batch change and perform
                            the indicated action on each changeset.
                        </>
                    )}
                </div>
                <div className="create-update-batch-change-alert__btn">
                    <button
                        type="button"
                        className={classNames(
                            'btn btn-primary test-batches-confirm-apply-btn text-nowrap',
                            isLoading === true || (!viewerCanAdminister && 'disabled')
                        )}
                        onClick={onApply}
                        disabled={isLoading === true || !viewerCanAdminister}
                        data-tooltip={
                            !viewerCanAdminister ? 'You have no permission to apply this batch change.' : undefined
                        }
                    >
                        Apply spec
                    </button>
                </div>
            </div>
            {isErrorLike(isLoading) && <ErrorAlert error={isLoading} />}
        </>
    )
}
