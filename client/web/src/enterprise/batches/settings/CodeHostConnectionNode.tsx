import React, { useCallback, useState } from 'react'
import CheckboxBlankCircleOutlineIcon from 'mdi-react/CheckboxBlankCircleOutlineIcon'
import CheckCircleOutlineIcon from 'mdi-react/CheckCircleOutlineIcon'
import { defaultExternalServices } from '../../../components/externalServices/externalServices'
import { BatchChangesCodeHostFields, Scalars } from '../../../graphql-operations'
import { AddCredentialModal } from './AddCredentialModal'
import { RemoveCredentialModal } from './RemoveCredentialModal'
import { Subject } from 'rxjs'
import { ViewCredentialModal } from './ViewCredentialModal'

export interface CodeHostConnectionNodeProps {
    node: BatchChangesCodeHostFields
    userID: Scalars['ID']
    updateList: Subject<void>
}

type OpenModal = 'add' | 'view' | 'delete'

export const CodeHostConnectionNode: React.FunctionComponent<CodeHostConnectionNodeProps> = ({
    node,
    userID,
    updateList,
}) => {
    const Icon = defaultExternalServices[node.externalServiceKind].icon

    const [openModal, setOpenModal] = useState<OpenModal | undefined>()
    const onClickAdd = useCallback(() => {
        setOpenModal('add')
    }, [])
    const onClickRemove = useCallback<React.MouseEventHandler>(event => {
        event.preventDefault()
        setOpenModal('delete')
    }, [])
    const onClickView = useCallback<React.MouseEventHandler>(event => {
        event.preventDefault()
        setOpenModal('view')
    }, [])
    const closeModal = useCallback(() => {
        setOpenModal(undefined)
    }, [])
    const afterAction = useCallback(() => {
        setOpenModal(undefined)
        updateList.next()
    }, [updateList])

    const isEnabled = node.credential !== null

    return (
        <>
            <li className="list-group-item p-3 test-code-host-connection-node">
                <div className="d-flex justify-content-between align-items-center mb-0">
                    <h3 className="mb-0">
                        {isEnabled && (
                            <CheckCircleOutlineIcon
                                className="text-success icon-inline test-code-host-connection-node-enabled"
                                data-tooltip="Connected"
                            />
                        )}
                        {!isEnabled && (
                            <CheckboxBlankCircleOutlineIcon
                                className="text-danger icon-inline test-code-host-connection-node-disabled"
                                data-tooltip="No token set"
                            />
                        )}
                        <Icon className="icon-inline mx-2" /> {node.externalServiceURL}
                    </h3>
                    <div className="mb-0">
                        {isEnabled && (
                            <>
                                <a
                                    href=""
                                    className="btn btn-link text-danger test-code-host-connection-node-btn-remove"
                                    onClick={onClickRemove}
                                >
                                    Remove
                                </a>
                                {node.requiresSSH && (
                                    <button type="button" onClick={onClickView} className="btn btn-secondary ml-2">
                                        View public key
                                    </button>
                                )}
                            </>
                        )}
                        {!isEnabled && (
                            <button
                                type="button"
                                className="btn btn-success test-code-host-connection-node-btn-add"
                                onClick={onClickAdd}
                            >
                                Add credentials
                            </button>
                        )}
                    </div>
                </div>
            </li>
            {openModal === 'delete' && (
                <RemoveCredentialModal
                    onCancel={closeModal}
                    afterDelete={afterAction}
                    codeHost={node}
                    credential={node.credential!}
                />
            )}
            {openModal === 'view' && (
                <ViewCredentialModal onClose={closeModal} codeHost={node} credential={node.credential!} />
            )}
            {openModal === 'add' && (
                <AddCredentialModal
                    onCancel={closeModal}
                    afterCreate={afterAction}
                    userID={userID}
                    externalServiceKind={node.externalServiceKind}
                    externalServiceURL={node.externalServiceURL}
                    requiresSSH={node.requiresSSH}
                />
            )}
        </>
    )
}
