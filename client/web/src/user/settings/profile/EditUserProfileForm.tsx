import React, { useCallback, useState } from 'react'

import { useNavigate } from 'react-router-dom'
import { lastValueFrom } from 'rxjs'

import { gql, useMutation } from '@sourcegraph/http-client'
import { Container, Button, Alert, Form } from '@sourcegraph/wildcard'

import { refreshAuthenticatedUser } from '../../../auth'
import type { EditUserProfilePage, UpdateUserResult, UpdateUserVariables } from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'

import { UserProfileFormFields, type UserProfileFormFieldsValue } from './UserProfileFormFields'

export const UPDATE_USER = gql`
    mutation UpdateUser($user: ID!, $username: String!, $displayName: String, $avatarURL: String) {
        updateUser(user: $user, username: $username, displayName: $displayName, avatarURL: $avatarURL) {
            id
            username
            displayName
            avatarURL
        }
    }
`

interface Props {
    user: Pick<EditUserProfilePage, 'id' | 'viewerCanChangeUsername' | 'scimControlled'>
    initialValue: UserProfileFormFieldsValue
    after?: React.ReactNode
}

/**
 * A form to edit a user's profile.
 */
export const EditUserProfileForm: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    user,
    initialValue,
    after,
}) => {
    const navigate = useNavigate()
    const [updateUser, { data, loading, error }] = useMutation<UpdateUserResult, UpdateUserVariables>(UPDATE_USER, {
        onCompleted: ({ updateUser }) => {
            eventLogger.log('UserProfileUpdated')
            navigate(`/users/${updateUser.username}/settings/profile`, { replace: true })

            // In case the edited user is the current user, immediately reflect the changes in the
            // UI.
            // TODO: Migrate this to use the Apollo cache
            lastValueFrom(refreshAuthenticatedUser(), { defaultValue: undefined }).finally(() => {})
        },
        onError: () => eventLogger.log('UpdateUserFailed'),
    })

    const [userFields, setUserFields] = useState<UserProfileFormFieldsValue>(initialValue)
    const onChange = useCallback<React.ComponentProps<typeof UserProfileFormFields>['onChange']>(
        newValue => setUserFields(previous => ({ ...previous, ...newValue })),
        []
    )

    const onSubmit = useCallback<React.FormEventHandler>(
        event => {
            event.preventDefault()
            eventLogger.log('UpdateUserClicked')
            return updateUser({
                variables: {
                    user: user.id,
                    username: userFields.username,
                    displayName: userFields.displayName,
                    avatarURL: userFields.avatarURL,
                },
            })
        },
        [updateUser, user.id, userFields]
    )

    const isUserScimControlled = user.scimControlled

    return (
        <Container>
            <Form className="w-100" onSubmit={onSubmit}>
                <UserProfileFormFields
                    value={userFields}
                    onChange={onChange}
                    usernameFieldDisabled={!user.viewerCanChangeUsername || isUserScimControlled}
                    displayNameFieldDisabled={isUserScimControlled}
                    disabled={loading}
                />
                <Button type="submit" disabled={loading} id="test-EditUserProfileForm__save" variant="primary">
                    Save
                </Button>
                {error && (
                    <Alert className="mt-3" variant="danger">
                        {error.message}
                    </Alert>
                )}
                {data?.updateUser && (
                    <Alert className="mt-3 mb-0 test-EditUserProfileForm__success" variant="success">
                        User profile updated.
                    </Alert>
                )}
                {after && (
                    <>
                        <hr className="my-4" />
                        {after}
                    </>
                )}
            </Form>
        </Container>
    )
}
