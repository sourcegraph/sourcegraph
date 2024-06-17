import React, { useCallback, useRef, useState } from 'react'

import type { ApolloError } from '@apollo/client'
import { mdiCheckCircleOutline, mdiCheckboxBlankCircleOutline, mdiDelete, mdiEye } from '@mdi/js'
import classNames from 'classnames'

import { logger } from '@sourcegraph/common'
import { useLazyQuery } from '@sourcegraph/http-client'
import { Badge, Button, Icon, H3, Tooltip } from '@sourcegraph/wildcard'

import { defaultExternalServices } from '../../../components/externalServices/externalServices'
import type {
    BatchChangesCodeHostFields,
    CheckBatchChangesCredentialResult,
    CheckBatchChangesCredentialVariables,
    UserAreaUserFields,
} from '../../../graphql-operations'

import { AddCredentialModal } from './AddCredentialModal'
import { CHECK_BATCH_CHANGES_CREDENTIAL } from './backend'
import { CheckButton } from './CheckButton'
import { RemoveCredentialModal } from './RemoveCredentialModal'
import { ViewCredentialModal } from './ViewCredentialModal'

import styles from './CodeHostConnectionNode.module.scss'

export interface CodeHostConnectionNodeProps {
    node: BatchChangesCodeHostFields
    refetchAll: () => void
    user: UserAreaUserFields | null
}

type OpenModal = 'add' | 'view' | 'delete'

export const CodeHostConnectionNode: React.FunctionComponent<React.PropsWithChildren<CodeHostConnectionNodeProps>> = ({
    node,
    refetchAll,
    user,
}) => {
    const [checkCredError, setCheckCredError] = useState<ApolloError | undefined>()
    const ExternalServiceIcon = defaultExternalServices[node.externalServiceKind].icon
    const codeHostDisplayName = defaultExternalServices[node.externalServiceKind].defaultDisplayName

    const [checkCred, { data: checkCredData, loading: checkCredLoading }] = useLazyQuery<
        CheckBatchChangesCredentialResult,
        CheckBatchChangesCredentialVariables
    >(CHECK_BATCH_CHANGES_CREDENTIAL, {
        onError: err => setCheckCredError(err),
    })

    const buttonReference = useRef<HTMLButtonElement | null>(null)

    const [openModal, setOpenModal] = useState<OpenModal | undefined>()
    const onClickAdd = useCallback(() => {
        setOpenModal('add')
    }, [])
    const onClickCheck = useCallback<React.MouseEventHandler>(async () => {
        await checkCred({ variables: { id: node?.credential?.id ?? '', isGitHubApp: false } })
    }, [node, checkCred])
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
        setCheckCredError(undefined)
        buttonReference.current?.focus()
        refetchAll()
    }, [refetchAll, buttonReference])

    const isEnabled = node.credential !== null && (user === null || !node.credential.isSiteCredential)

    const headingAriaLabel = `Sourcegraph ${
        isEnabled ? 'has credentials configured' : 'does not have credentials configured'
    } for ${codeHostDisplayName} (${node.externalServiceURL}).${
        !isEnabled && node.credential?.isSiteCredential
            ? ' Changesets on this code host will be created with a global token until a personal access token is added.'
            : ''
    }`

    // At the moment, log the error since it is not being displayed on the page.
    if (checkCredError) {
        logger.error(checkCredError.message)
    }

    return (
        <>
            <li
                className={classNames(
                    styles.codeHostConnectionNodeContainer,
                    'list-group-item test-code-host-connection-node'
                )}
            >
                <div
                    className={classNames(
                        styles.wrapper,
                        'd-flex justify-content-between align-items-center flex-wrap mb-0'
                    )}
                >
                    <H3 className="text-nowrap mb-0" aria-label={headingAriaLabel}>
                        {isEnabled && (
                            <Tooltip content="This code host has credentials connected.">
                                <Icon
                                    aria-label="This code host has credentials connected."
                                    className="text-success test-code-host-connection-node-enabled"
                                    svgPath={mdiCheckCircleOutline}
                                />
                            </Tooltip>
                        )}
                        {!isEnabled && (
                            <Tooltip content="This code host does not have credentials configured.">
                                <Icon
                                    aria-label="This code host does not have credentials configured."
                                    className="text-danger test-code-host-connection-node-disabled"
                                    svgPath={mdiCheckboxBlankCircleOutline}
                                />
                            </Tooltip>
                        )}
                        <Icon className="mx-2" aria-hidden={true} as={ExternalServiceIcon} /> {node.externalServiceURL}{' '}
                        {!isEnabled && node.credential?.isSiteCredential && (
                            <Badge
                                variant="secondary"
                                tooltip="Changesets on this code host will
                            be created with a global token until a personal access token is added."
                                aria-label="Changesets on this code host will
                            be created with a global token until a personal access token is added."
                            >
                                Global token
                            </Badge>
                        )}
                        {isEnabled && node.credential?.isGitHubApp && (
                            <Badge
                                variant="outlineSecondary"
                                tooltip="This code host is using a GitHub App"
                                className="ml-2"
                                role="button"
                            >
                                GitHub App
                            </Badge>
                        )}
                    </H3>
                    <div className="mb-0 d-flex justify-content-end flex-grow-1 align-items-baseline">
                        {isEnabled ? (
                            <>
                                <CheckButton
                                    label={`Check credentials for ${codeHostDisplayName}`}
                                    onClick={onClickCheck}
                                    loading={checkCredLoading}
                                    successMessage={checkCredData ? 'Credential is valid' : undefined}
                                    failedMessage={checkCredError ? 'Credential is not authorized' : undefined}
                                />

                                <Button
                                    className="ml-2 text-nowrap test-code-host-connection-node-btn-remove"
                                    aria-label={`Remove credentials for ${codeHostDisplayName}`}
                                    onClick={onClickRemove}
                                    variant="danger"
                                    size="sm"
                                    ref={buttonReference}
                                >
                                    <Icon aria-hidden={true} svgPath={mdiDelete} /> Remove
                                </Button>
                                {node.requiresSSH && (
                                    <Button
                                        onClick={onClickView}
                                        className="text-nowrap ml-2"
                                        variant="secondary"
                                        size="sm"
                                    >
                                        <Icon aria-hidden={true} svgPath={mdiEye} /> View public key
                                    </Button>
                                )}
                            </>
                        ) : (
                            /*
                                a11y-ignore
                                Rule: "color-contrast" (Elements must have sufficient color contrast)
                                GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/33343
                            */
                            <Button
                                className="a11y-ignore text-nowrap test-code-host-connection-node-btn-add"
                                onClick={onClickAdd}
                                aria-label={`Add credentials for ${codeHostDisplayName}`}
                                variant="success"
                                ref={buttonReference}
                                size="sm"
                            >
                                Add credentials
                            </Button>
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
                    user={user}
                    externalServiceKind={node.externalServiceKind}
                    externalServiceURL={node.externalServiceURL}
                    requiresSSH={node.requiresSSH}
                    requiresUsername={node.requiresUsername}
                />
            )}
        </>
    )
}
