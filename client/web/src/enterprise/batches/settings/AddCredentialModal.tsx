import React, { useCallback, useState } from 'react'

import classNames from 'classnames'

import { logger } from '@sourcegraph/common'
import { Button, Modal, Link, Code, Label, Text, Input, ErrorAlert, Form, Select } from '@sourcegraph/wildcard'

import { LoaderButton } from '../../../components/LoaderButton'
import { ExternalServiceKind, type Scalars } from '../../../graphql-operations'

import { useCreateBatchChangesCredential } from './backend'
import { CodeHostSshPublicKey } from './CodeHostSshPublicKey'
import { ModalHeader } from './ModalHeader'

import styles from './AddCredentialModal.module.scss'

export interface AddCredentialModalProps {
    onCancel: () => void
    afterCreate: () => void
    userID: Scalars['ID'] | null
    externalServiceKind: ExternalServiceKind
    externalServiceURL: string
    requiresSSH: boolean
    requiresUsername: boolean

    /** For testing only. */
    initialStep?: Step
}

const HELP_TEXT_LINK_URL = '/help/batch_changes/quickstart#configure-code-host-credentials'

const scopeRequirements: Record<ExternalServiceKind, JSX.Element> = {
    [ExternalServiceKind.GITHUB]: (
        <span>
            with the <Code>repo</Code>, <Code>read:org</Code>, <Code>user:email</Code>, <Code>read:discussion</Code>,
            and <Code>workflow</Code> scopes.
        </span>
    ),
    [ExternalServiceKind.GITLAB]: (
        <span>
            with <Code>api</Code>, <Code>read_repository</Code>, and <Code>write_repository</Code> scopes.
        </span>
    ),
    [ExternalServiceKind.BITBUCKETSERVER]: (
        <span>
            with <Code>write</Code> permissions on the project and repository level.
        </span>
    ),
    [ExternalServiceKind.BITBUCKETCLOUD]: (
        <span>
            with <Code>account:read</Code>, <Code>repo:write</Code>, <Code>pr:write</Code>, and{' '}
            <Code>pipeline:read</Code> permissions.
        </span>
    ),
    [ExternalServiceKind.AZUREDEVOPS]: (
        <span>
            with <Code>Organization:All accessible organizations</Code>, <Code>Code:Full</Code>,{' '}
            <Code>Code:Status</Code>, <Code>Pull Request Threads:Read & Write</Code>, and <Code>User Profile:Read</Code>{' '}
            permissions.
        </span>
    ),
    [ExternalServiceKind.GERRIT]: <span />,
    [ExternalServiceKind.PERFORCE]: <span>with the ability to shelve changelists.</span>,
    // These are just for type completeness and serve as placeholders for a bright future.
    [ExternalServiceKind.GITOLITE]: <span>Unsupported</span>,
    [ExternalServiceKind.GOMODULES]: <span>Unsupported</span>,
    [ExternalServiceKind.PYTHONPACKAGES]: <span>Unsupported</span>,
    [ExternalServiceKind.RUSTPACKAGES]: <span>Unsupported</span>,
    [ExternalServiceKind.RUBYPACKAGES]: <span>Unsupported</span>,
    [ExternalServiceKind.JVMPACKAGES]: <span>Unsupported</span>,
    [ExternalServiceKind.NPMPACKAGES]: <span>Unsupported</span>,
    [ExternalServiceKind.PHABRICATOR]: <span>Unsupported</span>,
    [ExternalServiceKind.AWSCODECOMMIT]: <span>Unsupported</span>,
    [ExternalServiceKind.PAGURE]: <span>Unsupported</span>,
    [ExternalServiceKind.OTHER]: <span>Unsupported</span>,
}

type Step = 'add-token' | 'get-ssh-key'

export const AddCredentialModal: React.FunctionComponent<React.PropsWithChildren<AddCredentialModalProps>> = ({
    onCancel,
    afterCreate,
    userID,
    externalServiceKind,
    externalServiceURL,
    requiresSSH,
    requiresUsername,
    initialStep = 'add-token',
}) => {
    const labelId = 'addCredential'
    const [credential, setCredential] = useState<string>('')
    const [sshPublicKey, setSSHPublicKey] = useState<string>()
    const [username, setUsername] = useState<string>('')
    const [step, setStep] = useState<Step>(initialStep)

    const onChangeCredential = useCallback<React.ChangeEventHandler<HTMLInputElement>>(event => {
        setCredential(event.target.value)
    }, [])

    const onChangeUsername = useCallback<React.ChangeEventHandler<HTMLInputElement>>(event => {
        setUsername(event.target.value)
    }, [])

    const [createBatchChangesCredential, { loading, error }] = useCreateBatchChangesCredential()

    const onSubmit = useCallback<React.FormEventHandler>(
        async event => {
            event.preventDefault()

            try {
                const { data } = await createBatchChangesCredential({
                    variables: {
                        user: userID,
                        credential,
                        username: requiresUsername ? username : null,
                        externalServiceKind,
                        externalServiceURL,
                    },
                })

                if (requiresSSH && data?.createBatchChangesCredential.sshPublicKey) {
                    setSSHPublicKey(data?.createBatchChangesCredential.sshPublicKey)
                    setStep('get-ssh-key')
                } else {
                    afterCreate()
                }
            } catch (error) {
                logger.error(error)
            }
        },
        [
            createBatchChangesCredential,
            userID,
            credential,
            requiresUsername,
            username,
            externalServiceKind,
            externalServiceURL,
            requiresSSH,
            afterCreate,
        ]
    )

    const patLabel =
        externalServiceKind === ExternalServiceKind.PERFORCE
            ? 'Ticket'
            : externalServiceKind === ExternalServiceKind.BITBUCKETCLOUD
            ? 'App password'
            : 'Personal access token'

    const isGitHubKind = externalServiceKind == ExternalServiceKind.GITHUB

    return (
        <Modal onDismiss={onCancel} aria-labelledby={labelId}>
            <div className="test-add-credential-modal">
                <ModalHeader
                    id={labelId}
                    externalServiceKind={externalServiceKind}
                    externalServiceURL={externalServiceURL}
                />
                {isGitHubKind && (
                    <Select
                        id="credential-kind"
                        selectSize="sm"
                        label="Select an Authentication scheme for your credential"
                    >
                        <option>Personal Access Token</option>
                        <option>GitHub App</option>
                    </Select>
                )}
                {requiresSSH && (
                    <div className="d-flex w-100 justify-content-between mb-4">
                        <div className="flex-grow-1 mr-2">
                            <Text className={classNames('mb-0 py-2', step === 'get-ssh-key' && 'text-muted')}>
                                1. Add token
                            </Text>
                            <div
                                className={classNames(
                                    styles.addCredentialModalModalStepRuler,
                                    styles.addCredentialModalModalStepRulerPurple
                                )}
                            />
                        </div>
                        <div className="flex-grow-1 ml-2">
                            <Text className={classNames('mb-0 py-2', step === 'add-token' && 'text-muted')}>
                                2. Get SSH Key
                            </Text>
                            <div
                                className={classNames(
                                    styles.addCredentialModalModalStepRuler,
                                    step === 'add-token' && styles.addCredentialModalModalStepRulerGray,
                                    step === 'get-ssh-key' && styles.addCredentialModalModalStepRulerBlue
                                )}
                            />
                        </div>
                    </div>
                )}
                {step === 'add-token' && (
                    <>
                        {error && <ErrorAlert error={error} />}
                        <Form onSubmit={onSubmit}>
                            <div className="form-group">
                                {requiresUsername && (
                                    <>
                                        <Input
                                            id="username"
                                            name="username"
                                            autoComplete="off"
                                            inputClassName="mb-2"
                                            className="mb-0"
                                            required={true}
                                            spellCheck="false"
                                            minLength={1}
                                            value={username}
                                            onChange={onChangeUsername}
                                            label="Username"
                                        />
                                    </>
                                )}
                                <Label htmlFor="token">{patLabel}</Label>
                                <Input
                                    id="token"
                                    name="token"
                                    type="password"
                                    autoComplete="off"
                                    data-testid="test-add-credential-modal-input"
                                    required={true}
                                    spellCheck="false"
                                    minLength={1}
                                    value={credential}
                                    onChange={onChangeCredential}
                                />
                                <Text className="form-text">
                                    <Link
                                        to={HELP_TEXT_LINK_URL}
                                        rel="noreferrer noopener"
                                        target="_blank"
                                        aria-label={`Follow our docs to learn how to create a new ${patLabel.toLocaleLowerCase()} on this code host`}
                                    >
                                        Create a new {patLabel.toLocaleLowerCase()}
                                    </Link>{' '}
                                    {scopeRequirements[externalServiceKind]}
                                </Text>
                            </div>
                            <div className="d-flex justify-content-end">
                                <Button
                                    disabled={loading}
                                    className="mr-2"
                                    onClick={onCancel}
                                    outline={true}
                                    variant="secondary"
                                >
                                    Cancel
                                </Button>
                                <LoaderButton
                                    type="submit"
                                    disabled={loading || credential.length === 0}
                                    className="test-add-credential-modal-submit"
                                    variant="primary"
                                    loading={loading}
                                    alwaysShowLabel={true}
                                    label={requiresSSH ? 'Next' : 'Add credential'}
                                />
                            </div>
                        </Form>
                    </>
                )}
                {step === 'get-ssh-key' && (
                    <>
                        <Text>
                            An SSH key has been generated for your batch changes code host connection. Copy the public
                            key below and enter it on your code host.
                        </Text>
                        <CodeHostSshPublicKey externalServiceKind={externalServiceKind} sshPublicKey={sshPublicKey!} />
                        <Button
                            className="test-add-credential-modal-submit float-right"
                            onClick={afterCreate}
                            variant="primary"
                        >
                            Finish
                        </Button>
                    </>
                )}
            </div>
        </Modal>
    )
}
