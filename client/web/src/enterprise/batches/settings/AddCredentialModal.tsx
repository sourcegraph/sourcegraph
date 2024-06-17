import React, { useCallback, useState, type FC } from 'react'

import classNames from 'classnames'

import { logger } from '@sourcegraph/common'
import { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { Button, Modal, Link, Code, Label, Text, Input, ErrorAlert, Form, Select } from '@sourcegraph/wildcard'

import { LoaderButton } from '../../../components/LoaderButton'
import { ExternalServiceKind, GitHubAppKind, type UserAreaUserFields } from '../../../graphql-operations'

import { useCreateBatchChangesCredential } from './backend'
import { BatchChangesCreateGitHubAppPage } from './BatchChangesCreateGitHubAppPage'
import { CodeHostSshPublicKey } from './CodeHostSshPublicKey'
import { ModalHeader } from './ModalHeader'

import styles from './AddCredentialModal.module.scss'

export interface AddCredentialModalProps {
    onCancel: () => void
    afterCreate: () => void
    user: UserAreaUserFields | null
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

const AuthenticationStrategy = {
    PERSONAL_ACCESS_TOKEN: 'PERSONAL_ACCESS_TOKEN',
    GITHUB_APP: 'GITHUB_APP',
} as const

type AuthenticationStrategyType = typeof AuthenticationStrategy[keyof typeof AuthenticationStrategy]

export const AddCredentialModal: FC<React.PropsWithChildren<AddCredentialModalProps>> = ({
    onCancel,
    afterCreate,
    user,
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
    const [authStrategy, setAuthStrategy] = useState<AuthenticationStrategyType>(
        AuthenticationStrategy.PERSONAL_ACCESS_TOKEN
    )

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
                        user: user?.id || null,
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
            user?.id,
            credential,
            requiresUsername,
            username,
            externalServiceKind,
            externalServiceURL,
            requiresSSH,
            afterCreate,
        ]
    )

    const isTokenSection = step === 'add-token'
    const isGitHubKind = externalServiceKind === ExternalServiceKind.GITHUB

    // addCredentialModalStepRuler
    return (
        <Modal onDismiss={onCancel} aria-labelledby={labelId} position="center">
            <div className={classNames('test-add-credential-modal', styles.addCredentialModalContainer)}>
                <ModalHeader
                    id={labelId}
                    externalServiceKind={externalServiceKind}
                    externalServiceURL={externalServiceURL}
                />
                {isGitHubKind && isTokenSection && (
                    <Select
                        id="credential-kind"
                        selectSize="sm"
                        label="Select an Authentication strategy for your credential"
                        value={authStrategy}
                        onChange={event => setAuthStrategy(event.target.value as AuthenticationStrategyType)}
                    >
                        <option value={AuthenticationStrategy.PERSONAL_ACCESS_TOKEN} defaultChecked={true}>
                            Personal Access Token
                        </option>
                        <option value={AuthenticationStrategy.GITHUB_APP}>GitHub App</option>
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
                                    styles.addCredentialModalStepRuler,
                                    styles.addCredentialModalStepRulerPurple
                                )}
                            />
                        </div>
                        <div className="flex-grow-1 ml-2">
                            <Text className={classNames('mb-0 py-2', step === 'add-token' && 'text-muted')}>
                                2. Get SSH Key
                            </Text>
                            <div
                                className={classNames(
                                    styles.addCredentialModalStepRuler,
                                    step === 'add-token' && styles.addCredentialModalStepRulerGray,
                                    step === 'get-ssh-key' && styles.addCredentialModalStepRulerBlue
                                )}
                            />
                        </div>
                    </div>
                )}
                {step === 'add-token' && (
                    <AddToken
                        step={step}
                        error={error}
                        credential={credential}
                        onChangeCredential={onChangeCredential}
                        username={username}
                        onChangeUsername={onChangeUsername}
                        requiresUsername={requiresUsername}
                        externalServiceKind={externalServiceKind}
                        onSubmit={onSubmit}
                        requiresSSH={requiresSSH}
                        loading={loading}
                        onCancel={onCancel}
                        authStrategy={authStrategy}
                        externalServiceURL={externalServiceURL}
                        user={user}
                    />
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

const computeCredentialLabel = (
    externalServiceKind: ExternalServiceKind,
    authStrategy: AuthenticationStrategyType
): string => {
    if (externalServiceKind === ExternalServiceKind.PERFORCE) {
        return 'Ticket'
    }

    if (externalServiceKind === ExternalServiceKind.BITBUCKETCLOUD) {
        return 'App password'
    }

    if (externalServiceKind === ExternalServiceKind.GITHUB && authStrategy === AuthenticationStrategy.GITHUB_APP) {
        return 'Create GitHub App'
    }

    return 'Personal access token'
}

interface AddTokenProps {
    step: Step
    error: unknown
    onSubmit: React.FormEventHandler<Element>
    requiresUsername: boolean
    credential: string
    username: string
    onChangeUsername: React.ChangeEventHandler<HTMLInputElement>
    onChangeCredential: React.ChangeEventHandler<HTMLInputElement>
    externalServiceKind: ExternalServiceKind
    requiresSSH: boolean
    loading: boolean
    onCancel: () => void
    authStrategy: AuthenticationStrategyType
    externalServiceURL: string
    user: UserAreaUserFields | null
}

const AddToken: FC<AddTokenProps> = ({
    step,
    error,
    onSubmit,
    requiresUsername,
    credential,
    username,
    onChangeUsername,
    onChangeCredential,
    externalServiceKind,
    requiresSSH,
    loading,
    onCancel,
    authStrategy,
    externalServiceURL,
    user,
}) => {
    const patLabel = computeCredentialLabel(externalServiceKind, authStrategy)
    const isStrategyPAT = authStrategy === AuthenticationStrategy.PERSONAL_ACCESS_TOKEN
    const kind = user ? GitHubAppKind.USER_CREDENTIAL : GitHubAppKind.SITE_CREDENTIAL

    if (step === 'add-token') {
        return (
            <>
                {error && <ErrorAlert error={error} />}
                {isStrategyPAT ? (
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
                        <div className="d-flex justify-content-end align-items-center">
                            {isStrategyPAT && (
                                <>
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
                                </>
                            )}
                        </div>
                    </Form>
                ) : (
                    <BatchChangesCreateGitHubAppPage
                        authenticatedUser={user as unknown as AuthenticatedUser}
                        minimizedMode={true}
                        kind={kind}
                    />
                )}
            </>
        )
    }

    return null
}
