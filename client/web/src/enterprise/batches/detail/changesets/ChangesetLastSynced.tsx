import React, { useState, useEffect, useCallback } from 'react'
import classNames from 'classnames'
import { formatDistance, parseISO } from 'date-fns'
import { syncChangeset } from '../backend'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import SyncIcon from 'mdi-react/SyncIcon'
import ErrorIcon from 'mdi-react/ErrorIcon'
import { isErrorLike } from '../../../../../../shared/src/util/errors'
import InfoCircleOutlineIcon from 'mdi-react/InfoCircleOutlineIcon'
import { ExternalChangesetFields, HiddenExternalChangesetFields } from '../../../../graphql-operations'

interface Props {
    changeset:
        | Pick<HiddenExternalChangesetFields, 'id' | 'nextSyncAt' | 'updatedAt' | '__typename'>
        | Pick<ExternalChangesetFields, 'id' | 'nextSyncAt' | 'updatedAt' | '__typename' | 'syncerError'>
    viewerCanAdminister: boolean
    /** For testing purposes only */
    _now?: Date
}

export const ChangesetLastSynced: React.FunctionComponent<Props> = ({ changeset, viewerCanAdminister, _now }) => {
    // initially, the changeset was never last updated
    const [lastUpdatedAt, setLastUpdatedAt] = useState<string | Error | null>(null)
    // .. if it was, and the changesets current updatedAt doesn't match the previous updated at, we know that it has been synced
    const lastUpdatedAtChanged = lastUpdatedAt && !isErrorLike(lastUpdatedAt) && changeset.updatedAt !== lastUpdatedAt
    useEffect(() => {
        if (lastUpdatedAtChanged) {
            setLastUpdatedAt(null)
        }
    }, [lastUpdatedAtChanged, changeset.updatedAt])
    const enqueueChangeset = useCallback<React.MouseEventHandler>(async () => {
        if (!viewerCanAdminister) {
            return
        }
        // already enqueued
        if (typeof lastUpdatedAt === 'string') {
            return
        }
        setLastUpdatedAt(changeset.updatedAt)
        try {
            await syncChangeset(changeset.id)
        } catch (error) {
            setLastUpdatedAt(error)
        }
    }, [changeset.id, changeset.updatedAt, lastUpdatedAt, viewerCanAdminister])

    let tooltipText = ''
    if (changeset.updatedAt === lastUpdatedAt) {
        tooltipText = 'Currently refreshing'
    } else {
        if (!changeset.nextSyncAt) {
            tooltipText = 'Not scheduled for syncing.'
        } else {
            tooltipText = `Next refresh in ${formatDistance(parseISO(changeset.nextSyncAt), _now ?? new Date())}.`
        }
        if (viewerCanAdminister) {
            tooltipText += ' Click to prioritize refresh'
        }
    }

    const UpdateLoaderIcon =
        typeof lastUpdatedAt === 'string' && changeset.updatedAt === lastUpdatedAt
            ? LoadingSpinner
            : viewerCanAdminister
            ? SyncIcon
            : InfoCircleOutlineIcon

    return (
        <small className="text-muted">
            {changeset.__typename === 'ExternalChangeset' && changeset.syncerError ? (
                <span data-tooltip="Expand to see details.">
                    <ErrorIcon className="icon-inline text-danger" /> Syncing from code host failed.
                </span>
            ) : (
                <>Last synced {formatDistance(parseISO(changeset.updatedAt), _now ?? new Date())} ago.</>
            )}{' '}
            {isErrorLike(lastUpdatedAt) && (
                <ErrorIcon data-tooltip={lastUpdatedAt.message} className="ml-2 icon-inline small" />
            )}
            <span data-tooltip={tooltipText}>
                <UpdateLoaderIcon
                    className={classNames(
                        'icon-inline',
                        typeof lastUpdatedAt !== 'string' && viewerCanAdminister && 'cursor-pointer'
                    )}
                    onClick={enqueueChangeset}
                />
            </span>
        </small>
    )
}
