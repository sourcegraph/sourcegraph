import React, { useCallback, useState } from 'react'
import Dialog from '@reach/dialog'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { asError, isErrorLike } from '../../../../../shared/src/util/errors'
import { ErrorAlert } from '../../../components/alerts'
import { deleteBatchChangesCredential } from './backend'
import { BatchChangesCodeHostFields, BatchChangesCredentialFields } from '../../../graphql-operations'
import { CodeHostSshPublicKey } from './CodeHostSshPublicKey'
import { ModalHeader } from './ModalHeader'

export interface RemoveCredentialModalProps {
    codeHost: BatchChangesCodeHostFields
    credential: BatchChangesCredentialFields

    onCancel: () => void
    afterDelete: () => void
}

export const RemoveCredentialModal: React.FunctionComponent<RemoveCredentialModalProps> = ({
    codeHost,
    credential,
    onCancel,
    afterDelete,
}) => {
    const labelId = 'removeCredential'
    const [isLoading, setIsLoading] = useState<boolean | Error>(false)
    const onDelete = useCallback<React.MouseEventHandler>(async () => {
        setIsLoading(true)
        try {
            await deleteBatchChangesCredential(credential.id)
            afterDelete()
        } catch (error) {
            setIsLoading(asError(error))
        }
    }, [afterDelete, credential.id])
    return (
        <Dialog
            className="modal-body modal-body--top-third p-4 rounded border"
            onDismiss={onCancel}
            aria-labelledby={labelId}
        >
            <div className="web-content test-remove-credential-modal">
                <ModalHeader
                    id={labelId}
                    externalServiceKind={codeHost.externalServiceKind}
                    externalServiceURL={codeHost.externalServiceURL}
                />

                <h3 className="text-danger mb-4">Removing credentials is irreversible</h3>

                {isErrorLike(isLoading) && <ErrorAlert error={isLoading} />}

                <p>
                    To create changesets on this code host after removing credentials, you will need to repeat the 'Add
                    credentials' process.
                </p>

                {codeHost.requiresSSH && (
                    <CodeHostSshPublicKey
                        externalServiceKind={codeHost.externalServiceKind}
                        sshPublicKey={credential.sshPublicKey!}
                        showInstructionsLink={false}
                        showCopyButton={false}
                        label="Public key to remove"
                    />
                )}

                <div className="d-flex justify-content-end pt-1">
                    <button
                        type="button"
                        disabled={isLoading === true}
                        className="btn btn-outline-secondary mr-2"
                        onClick={onCancel}
                    >
                        Cancel
                    </button>
                    <button
                        type="button"
                        disabled={isLoading === true}
                        className="btn btn-danger test-remove-credential-modal-submit"
                        onClick={onDelete}
                    >
                        {isLoading === true && <LoadingSpinner className="icon-inline" />}
                        Remove credentials
                    </button>
                </div>
            </div>
        </Dialog>
    )
}
