import * as H from 'history'
import React, { useCallback, useState } from 'react'
import { Scalars } from '../../../graphql-operations'
import DeleteIcon from 'mdi-react/DeleteIcon'
import { Link } from '../../../../../shared/src/components/Link'
import { deleteBatchChange as _deleteBatchChange } from './backend'
import { isErrorLike, asError } from '../../../../../shared/src/util/errors'
import InformationIcon from 'mdi-react/InformationIcon'

export interface BatchChangeDetailsActionSectionProps {
    batchChangeID: Scalars['ID']
    batchChangeClosed: boolean
    batchChangeNamespaceURL: string
    history: H.History

    /** For testing only. */
    deleteBatchChange?: typeof _deleteBatchChange
}

export const BatchChangeDetailsActionSection: React.FunctionComponent<BatchChangeDetailsActionSectionProps> = ({
    batchChangeID,
    batchChangeClosed,
    batchChangeNamespaceURL,
    history,
    deleteBatchChange = _deleteBatchChange,
}) => {
    const [isDeleting, setIsDeleting] = useState<boolean | Error>(false)
    const onDeleteBatchChange = useCallback(async () => {
        if (!confirm('Do you really want to delete this batch change?')) {
            return
        }
        setIsDeleting(true)
        try {
            await deleteBatchChange(batchChangeID)
            history.push(batchChangeNamespaceURL + '/batch-changes')
        } catch (error) {
            setIsDeleting(asError(error))
        }
    }, [batchChangeID, deleteBatchChange, history, batchChangeNamespaceURL])
    if (batchChangeClosed) {
        return (
            <button
                type="button"
                className="btn btn-outline-danger test-batches-delete-btn"
                onClick={onDeleteBatchChange}
                data-tooltip="Deleting this batch change is a final action."
                disabled={isDeleting === true}
            >
                {isErrorLike(isDeleting) && <InformationIcon className="icon-inline" data-tooltip={isDeleting} />}
                <DeleteIcon className="icon-inline" /> Delete
            </button>
        )
    }
    return (
        <Link
            to={`${location.pathname}/close`}
            className="btn btn-outline-danger test-batches-close-btn"
            data-tooltip="View a preview of all changes that will happen when you close this batch change."
        >
            <DeleteIcon className="icon-inline" /> Close
        </Link>
    )
}
