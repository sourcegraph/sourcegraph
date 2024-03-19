import React, { useCallback, useState } from 'react'

import { lastValueFrom } from 'rxjs'

import { asError, type ErrorLike } from '@sourcegraph/common'
import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'
import { Button, Modal, H3, Form } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../../backend/graphql'
import type { Scalars, DeleteExternalAccountResult, DeleteExternalAccountVariables } from '../../../graphql-operations'

const deleteUserExternalAccount = async (externalAccount: Scalars['ID']): Promise<void> => {
    dataOrThrowErrors(
        await lastValueFrom(
            requestGraphQL<DeleteExternalAccountResult, DeleteExternalAccountVariables>(
                gql`
                    mutation DeleteExternalAccount($externalAccount: ID!) {
                        deleteExternalAccount(externalAccount: $externalAccount) {
                            alwaysNil
                        }
                    }
                `,
                { externalAccount }
            )
        )
    )
}

export const RemoveExternalAccountModal: React.FunctionComponent<
    React.PropsWithChildren<{
        id: Scalars['ID']
        name: string

        onDidRemove: (id: string, name: string) => void
        onDidCancel: () => void
        onDidError: (error: ErrorLike) => void
        isOpen: boolean
    }>
> = ({ id, name, onDidRemove, onDidCancel, onDidError, isOpen }) => {
    const [isLoading, setIsLoading] = useState(false)

    const onAccountRemove = useCallback<React.FormEventHandler<HTMLFormElement>>(
        async event => {
            event.preventDefault()
            setIsLoading(true)

            try {
                await deleteUserExternalAccount(id)
                onDidRemove(id, name)
            } catch (error) {
                setIsLoading(false)
                onDidError(asError(error))
                onDidCancel()
            }
        },
        [id, name, onDidRemove, onDidError, onDidCancel]
    )

    return (
        <Modal
            aria-labelledby={`heading--disconnect-${name}`}
            aria-describedby={`description--disconnect-${name}`}
            onDismiss={onDidCancel}
            isOpen={isOpen}
        >
            <H3 id={`heading--disconnect-${name}`} className="text-danger mb-4">
                Disconnect {name}?
            </H3>
            <Form onSubmit={onAccountRemove}>
                <div id={`description--disconnect-${name}`} className="form-group mb-4">
                    You are about to remove the sign in connection with {name}. After removing it, you won’t be able to
                    use {name} to sign in to Sourcegraph.
                </div>
                <div className="d-flex justify-content-end">
                    <Button
                        disabled={isLoading}
                        className="mr-2"
                        onClick={onDidCancel}
                        outline={true}
                        variant="secondary"
                    >
                        Cancel
                    </Button>
                    <Button type="submit" disabled={isLoading} variant="danger">
                        Yes, disconnect {name}
                    </Button>
                </div>
            </Form>
        </Modal>
    )
}
