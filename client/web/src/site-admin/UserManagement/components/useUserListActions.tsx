import React, { useState, useCallback } from 'react'

import { lastValueFrom } from 'rxjs'

import { logger } from '@sourcegraph/common'
import { useMutation } from '@sourcegraph/http-client'
import { Text } from '@sourcegraph/wildcard'

import { CopyableText } from '../../../components/CopyableText'
import { randomizeUserPassword, setUserIsSiteAdmin } from '../../backend'
import { DELETE_USERS, DELETE_USERS_FOREVER, FORCE_SIGN_OUT_USERS, RECOVER_USERS } from '../queries'

import { type UseUserListActionReturnType, type SiteUser, getUsernames } from './UsersList'

export function useUserListActions(onEnd: (error?: any) => void): UseUserListActionReturnType {
    const [forceSignOutUsers] = useMutation(FORCE_SIGN_OUT_USERS)
    const [deleteUsers] = useMutation(DELETE_USERS)
    const [deleteUsersForever] = useMutation(DELETE_USERS_FOREVER)

    const [recoverUsers] = useMutation(RECOVER_USERS)

    const [notification, setNotification] = useState<UseUserListActionReturnType['notification']>()

    const handleDismissNotification = useCallback(() => setNotification(undefined), [])
    const handleDisplayNotification = useCallback((text: React.ReactNode) => setNotification({ text }), [])

    const onError = useCallback(
        (error: any) => {
            setNotification({
                text: (
                    <Text as="span">
                        Something went wrong :(!
                        <Text as="pre" className="m-1" size="small">
                            {error?.message}
                        </Text>
                    </Text>
                ),
                isError: true,
            })
            logger.error(error)
            onEnd(error)
        },
        [onEnd]
    )

    const createOnSuccess = useCallback(
        (text: React.ReactNode, shouldReload = false) =>
            () => {
                setNotification({ text })
                if (shouldReload) {
                    onEnd()
                }
            },
        [onEnd]
    )

    const handleForceSignOutUsers = useCallback(
        (users: SiteUser[]) => {
            if (confirm('Are you sure you want to force sign out the selected user(s)?')) {
                forceSignOutUsers({ variables: { userIDs: users.map(user => user.id) } })
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully force signed out following {users.length} user(s):{' '}
                                <strong>{getUsernames(users)}</strong>
                            </Text>
                        )
                    )
                    .catch(onError)
            }
        },
        [forceSignOutUsers, onError, createOnSuccess]
    )

    const handleDeleteUsers = useCallback(
        (users: SiteUser[]) => {
            if (confirm('Are you sure you want to delete the selected user(s)?')) {
                deleteUsers({ variables: { userIDs: users.map(user => user.id) } })
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully deleted following {users.length} user(s):{' '}
                                <strong>{getUsernames(users)}</strong>
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [deleteUsers, onError, createOnSuccess]
    )
    const handleDeleteUsersForever = useCallback(
        (users: SiteUser[]) => {
            if (confirm('Are you sure you want to delete the selected user(s)?')) {
                deleteUsersForever({ variables: { userIDs: users.map(user => user.id) } })
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully deleted forever following {users.length} user(s):{' '}
                                <strong>{getUsernames(users)}</strong>
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [deleteUsersForever, onError, createOnSuccess]
    )

    const handleRecoverUsers = useCallback(
        (users: SiteUser[]) => {
            if (confirm('Are you sure you want to recover the selected user(s)?')) {
                recoverUsers({ variables: { userIDs: users.map(user => user.id) } })
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully recovered following {users.length} user(s):{' '}
                                <strong>{getUsernames(users)}</strong>
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [recoverUsers, onError, createOnSuccess]
    )

    const handlePromoteToSiteAdmin = useCallback(
        ([user]: SiteUser[]) => {
            if (confirm('Are you sure you want to promote the selected user to site admin?')) {
                setUserIsSiteAdmin(user.id, true)
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully promoted user <strong>{user.username}</strong> to site admin.
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [onError, createOnSuccess]
    )

    const handleUnlockUser = useCallback(
        ([user]: SiteUser[]) => {
            if (confirm("Are you sure you want to unlock this user's account?")) {
                fetch('/-/unlock-user-account', {
                    method: 'POST',
                    headers: {
                        Accept: 'application/json',
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ username: user.username }),
                })
                    .then(response => {
                        if (response.status === 200) {
                            createOnSuccess(
                                <Text as="span">
                                    Successfully unlocked user <strong>{user.username}</strong>{' '}
                                </Text>,
                                true
                            )()
                            return
                        }

                        response
                            .text()
                            .then(text => {
                                onError(new Error('Failed to unlock user: ' + text))
                            })
                            .catch(onError)
                    })
                    .catch(onError)
            }
        },
        [onError, createOnSuccess]
    )

    const handleRevokeSiteAdmin = useCallback(
        ([user]: SiteUser[]) => {
            if (confirm('Are you sure you want to revoke the selected user from site admin?')) {
                setUserIsSiteAdmin(user.id, false)
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully revoked site admin from <strong>{user.username}</strong> user.
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [onError, createOnSuccess]
    )

    const handleResetUserPassword = useCallback(
        ([user]: SiteUser[]) => {
            if (confirm('Are you sure you want to reset the selected user password?')) {
                lastValueFrom(randomizeUserPassword(user.id))
                    .then(({ resetPasswordURL, emailSent }) => {
                        if (resetPasswordURL === null || emailSent) {
                            createOnSuccess(
                                <Text className="mb-0">
                                    Password was reset. The reset link was sent to the primary email of the user:{' '}
                                    <strong>{user.username}</strong>
                                </Text>
                            )()
                        } else {
                            createOnSuccess(
                                <div>
                                    <Text className="mb-2">
                                        Password was reset. You must manually send <strong>{user.username}</strong> this
                                        reset link:
                                    </Text>
                                    <CopyableText text={resetPasswordURL} size={40} />
                                </div>
                            )()
                        }
                    })
                    .catch(onError)
            }
        },
        [onError, createOnSuccess]
    )

    return {
        notification,
        handleForceSignOutUsers,
        handleDeleteUsers,
        handleDeleteUsersForever,
        handlePromoteToSiteAdmin,
        handleUnlockUser,
        handleRecoverUsers,
        handleRevokeSiteAdmin,
        handleResetUserPassword,
        handleDismissNotification,
        handleDisplayNotification,
    }
}
