import React, { useState, useCallback, type FC } from 'react'

import type { ErrorLike } from '@sourcegraph/common'
import { Button, Link, H3 } from '@sourcegraph/wildcard'

import { LoaderButton } from '../../../components/LoaderButton'
import type { AuthProvider } from '../../../jscontext'

import { AddGerritAccountModal } from './AddGerritAccountModal'
import type { NormalizedExternalAccount } from './ExternalAccountsSignIn'
import { RemoveExternalAccountModal } from './RemoveExternalAccountModal'

interface Props {
    account: NormalizedExternalAccount
    authProvider: AuthProvider
    onDidRemove: (id: string, name: string) => void
    onDidError: (error: ErrorLike) => void
    onDidAdd: () => void
}

export const ExternalAccount: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    account,
    authProvider,
    onDidRemove,
    onDidError,
    onDidAdd,
}) => {
    const [isLoading, setIsLoading] = useState(false)
    const [isRemoveAccountModalOpen, setIsRemoveAccountModalOpen] = useState(false)
    const [isAddGerritAccountModalOpen, setIsGerritAccountModalOpen] = useState(false)

    const navigateToAuthProvider = useCallback((): void => {
        if (authProvider.serviceType === 'gerrit') {
            setIsGerritAccountModalOpen(true)
            return
        }
        setIsLoading(true)

        const authURL = new URL(authProvider.authenticationURL, window.location.origin)
        authURL.searchParams.set('connect', 'true')

        if (authProvider.serviceType === 'saml') {
            window.location.assign(authURL.toString())
        } else {
            authURL.searchParams.set('redirect', window.location.href)
            window.location.assign(authURL.toString())
        }
    }, [authProvider.serviceType, authProvider.authenticationURL])

    const { icon: AccountIcon } = account

    return (
        <div className="d-flex align-items-start">
            {account.external && (
                <RemoveExternalAccountModal
                    id={account.external.id}
                    name={account.name}
                    onDidCancel={() => setIsRemoveAccountModalOpen(false)}
                    onDidRemove={(id: string, name: string) => {
                        onDidRemove(id, name)
                        setIsRemoveAccountModalOpen(false)
                    }}
                    onDidError={onDidError}
                    isOpen={isRemoveAccountModalOpen}
                />
            )}
            {isAddGerritAccountModalOpen && (
                <AddGerritAccountModal
                    serviceID={authProvider.serviceID}
                    onDidAdd={() => {
                        onDidAdd()
                        setIsGerritAccountModalOpen(false)
                    }}
                    onDismiss={() => setIsGerritAccountModalOpen(false)}
                    isOpen={isAddGerritAccountModalOpen}
                />
            )}
            <div className="align-self-center">
                <AccountIcon className="mb-0 mr-2" />
            </div>
            <div className="flex-1 flex-column">
                <H3 className="m-0">{authProvider.displayName}</H3>
                <div className="text-muted">
                    <ExternalAccountConnectionDetails account={account} serviceType={authProvider.serviceType} />
                </div>
            </div>
            <div className="align-self-center">
                {account.external ? (
                    <Button
                        className="text-danger px-0"
                        onClick={() => setIsRemoveAccountModalOpen(true)}
                        variant="link"
                    >
                        Remove
                    </Button>
                ) : (
                    <LoaderButton
                        loading={isLoading}
                        label="Add"
                        display="block"
                        onClick={navigateToAuthProvider}
                        variant="success"
                    />
                )}
            </div>
        </div>
    )
}

interface ExternalAccountConnectionDetailsProps {
    account: NormalizedExternalAccount
    serviceType: AuthProvider['serviceType']
}

export const ExternalAccountConnectionDetails: FC<ExternalAccountConnectionDetailsProps> = ({
    account,
    serviceType,
}) => {
    switch (serviceType) {
        case 'openidconnect':
        case 'saml':
        case 'gerrit': {
            return <span>{account.external?.displayName || 'Not connected'}</span>
        }
        case 'azuredevops': {
            return (
                <>
                    {account.external?.displayName ? (
                        <>
                            {account.external.displayName} (@{account.external?.login})
                        </>
                    ) : (
                        'Not connected'
                    )}
                </>
            )
        }
        // Bitbucket Server doesn't have a URL because there are no profile pages
        // as far as I can tell, so using a slightly modified version here.
        case 'bitbucketServer':
            return (
                <>
                    {account.external?.login ? (
                        <>
                            {account.external.displayName} (@{account.external.login})
                        </>
                    ) : (
                        'Not connected'
                    )}
                </>
            )

        default: {
            return (
                <>
                    {account.external?.url ? (
                        <>
                            {account.external.displayName} (
                            <Link to={account.external.url} target="_blank" rel="noopener noreferrer">
                                @{account.external.login}
                            </Link>
                            )
                        </>
                    ) : (
                        'Not connected'
                    )}
                </>
            )
        }
    }
}
