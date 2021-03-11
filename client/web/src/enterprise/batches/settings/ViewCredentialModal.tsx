import Dialog from '@reach/dialog'
import React from 'react'
import { BatchChangesCodeHostFields, BatchChangesCredentialFields } from '../../../graphql-operations'
import { CodeHostSshPublicKey } from './CodeHostSshPublicKey'
import { ModalHeader } from './ModalHeader'

interface ViewCredentialModalProps {
    codeHost: BatchChangesCodeHostFields
    credential: BatchChangesCredentialFields

    onClose: () => void
}

export const ViewCredentialModal: React.FunctionComponent<ViewCredentialModalProps> = ({
    credential,
    codeHost,
    onClose,
}) => {
    const labelId = 'viewCredential'
    return (
        <Dialog
            className="modal-body modal-body--top-third p-4 rounded border"
            onDismiss={onClose}
            aria-labelledby={labelId}
        >
            <div className="web-content">
                <ModalHeader
                    id={labelId}
                    externalServiceKind={codeHost.externalServiceKind}
                    externalServiceURL={codeHost.externalServiceURL}
                />

                <h4>Personal access token</h4>
                <div className="form-group">
                    <input
                        type="text"
                        value="PATs cannot be viewed after entering."
                        className="form-control"
                        disabled={true}
                    />
                </div>

                <hr className="mb-3" />

                <CodeHostSshPublicKey
                    externalServiceKind={codeHost.externalServiceKind}
                    sshPublicKey={credential.sshPublicKey!}
                />

                <div className="d-flex justify-content-end">
                    <button type="button" className="btn btn-outline-secondary" onClick={onClose}>
                        Close
                    </button>
                </div>
            </div>
        </Dialog>
    )
}
