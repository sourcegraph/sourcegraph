import React, { useCallback } from 'react'

import { ErrorAlert } from '@sourcegraph/branded/src/components/alerts'
import { Button, LoadingSpinner, Modal } from '@sourcegraph/wildcard'

import { BatchChangesCodeHostFields, BatchChangesCredentialFields } from '../../../graphql-operations'

import { useDeleteBatchChangesCredential } from './backend'
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
    const [deleteBatchChangesCredential, { loading, error }] = useDeleteBatchChangesCredential()
    const onDelete = useCallback<React.MouseEventHandler>(async () => {
        await deleteBatchChangesCredential({ variables: { id: credential.id } })
        afterDelete()
    }, [afterDelete, credential.id])
    return (
        <Modal onDismiss={onCancel} aria-labelledby={labelId}>
            <div className="test-remove-credential-modal">
                <ModalHeader
                    id={labelId}
                    externalServiceKind={codeHost.externalServiceKind}
                    externalServiceURL={codeHost.externalServiceURL}
                />

                <h3 className="text-danger mb-4">Removing credentials is irreversible</h3>

                {error && <ErrorAlert error={error} />}

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
                    <Button disabled={loading} className="mr-2" onClick={onCancel} outline={true} variant="secondary">
                        Cancel
                    </Button>
                    <Button
                        disabled={loading}
                        className="test-remove-credential-modal-submit"
                        onClick={onDelete}
                        variant="danger"
                    >
                        {loading && <LoadingSpinner />}
                        Remove credentials
                    </Button>
                </div>
            </div>
        </Modal>
    )
}
