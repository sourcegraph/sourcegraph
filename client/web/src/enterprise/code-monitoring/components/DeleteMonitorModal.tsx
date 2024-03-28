import React, { useCallback } from 'react'

import { useNavigate } from 'react-router-dom'
import { type Observable, throwError } from 'rxjs'
import { mergeMap, startWith, tap, catchError } from 'rxjs/operators'

import { asError, isErrorLike } from '@sourcegraph/common'
import { Button, LoadingSpinner, useEventObservable, Modal, Alert, H3, Text } from '@sourcegraph/wildcard'

import type { CodeMonitorFormProps } from './CodeMonitorForm'

interface DeleteModalProps extends Pick<CodeMonitorFormProps, 'codeMonitor'> {
    isOpen: boolean
    toggleDeleteModal: () => void
    deleteCodeMonitor: (id: string) => Observable<void>
}

export const DeleteMonitorModal: React.FunctionComponent<React.PropsWithChildren<DeleteModalProps>> = ({
    isOpen,
    deleteCodeMonitor,
    toggleDeleteModal,
    codeMonitor,
}) => {
    const LOADING = 'loading' as const
    const navigate = useNavigate()

    const deleteLabelId = 'deleteCodeMonitor'

    const [onDelete, deleteCompletedOrError] = useEventObservable(
        useCallback(
            (click: Observable<React.MouseEvent<HTMLButtonElement>>) =>
                click.pipe(
                    mergeMap(() => {
                        if (codeMonitor) {
                            return deleteCodeMonitor(codeMonitor.id).pipe(
                                tap(() => {
                                    navigate('/code-monitoring')
                                }),
                                startWith(LOADING),
                                catchError(error => [asError(error)])
                            )
                        }

                        return throwError(() => new Error('Failed to delete: Code monitor ID not provided'))
                    })
                ),
            [deleteCodeMonitor, navigate, codeMonitor]
        )
    )

    return (
        <Modal
            position="center"
            isOpen={isOpen}
            onDismiss={toggleDeleteModal}
            aria-labelledby={deleteLabelId}
            data-testid="delete-modal"
        >
            <H3 className="text-danger" id={deleteLabelId}>
                Delete code monitor?
            </H3>

            <Text>
                <strong>This action cannot be undone.</strong> Code monitoring will no longer watch for trigger event
                and all actions will immediately be removed.
            </Text>
            {(!deleteCompletedOrError || isErrorLike(deleteCompletedOrError)) && (
                <div className="text-right">
                    <Button className="mr-2" onClick={toggleDeleteModal} outline={true} variant="secondary">
                        Cancel
                    </Button>
                    <Button onClick={onDelete} data-testid="confirm-delete-monitor" variant="danger">
                        Yes, delete code monitor
                    </Button>
                    {isErrorLike(deleteCompletedOrError) && (
                        <Alert variant="danger">Error deleting monitor: {deleteCompletedOrError.message}</Alert>
                    )}
                </div>
            )}
            {/*
             * Issue: This JSX tag's 'children' prop expects a single child of type 'ReactNode', but multiple children were provided
             * It seems that v18 requires explicit boolean value
             */}
            {!!deleteCompletedOrError && <div>{deleteCompletedOrError === 'loading' && <LoadingSpinner />}</div>}
        </Modal>
    )
}
