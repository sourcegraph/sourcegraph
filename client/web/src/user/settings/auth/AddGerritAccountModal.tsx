import React from 'react'

import { gql, useMutation } from '@sourcegraph/http-client'
import { Alert, Button, Form, H3, Input, Modal, Text } from '@sourcegraph/wildcard'

import type { AddExternalAccountResult, AddExternalAccountVariables } from '../../../graphql-operations'

export const ADD_EXTERNAL_ACCOUNT = gql`
    mutation AddExternalAccount($serviceType: String!, $serviceID: String!, $accountDetails: String!) {
        addExternalAccount(serviceType: $serviceType, serviceID: $serviceID, accountDetails: $accountDetails) {
            alwaysNil
        }
    }
`

export const AddGerritAccountModal: React.FunctionComponent<
    React.PropsWithChildren<{
        onDidAdd: () => void
        onDismiss: () => void
        serviceID: string
        isOpen: boolean
    }>
> = ({ onDidAdd, serviceID, isOpen, onDismiss }) => {
    const [addExternalAccount, { error, loading }] = useMutation<AddExternalAccountResult, AddExternalAccountVariables>(
        ADD_EXTERNAL_ACCOUNT,
        {
            onCompleted: () => {
                onDidAdd()
            },
        }
    )

    const onAccountAdd: React.FormEventHandler<HTMLFormElement> = event => {
        event.preventDefault()
        const target = event.target as typeof event.target & {
            username: { value: string }
            password: { value: string }
        }

        addExternalAccount({
            variables: {
                serviceType: 'gerrit',
                serviceID,
                accountDetails: JSON.stringify({
                    username: target.username.value,
                    password: target.password.value,
                }),
            },
        }).catch(() => {})
    }

    return (
        <Modal
            aria-labelledby="heading--add-gerrit-account"
            aria-describedby="description--add-gerrit-account"
            isOpen={isOpen}
            onDismiss={onDismiss}
        >
            <H3 id="heading--add-gerrit-account" className="mb-4">
                Add Gerrit account
            </H3>
            <Form onSubmit={onAccountAdd}>
                {error && <Alert variant="danger">{error.message}</Alert>}
                <Text id="description--add-gerrit-account">
                    You are about to add a Gerrit account. Please enter your Gerrit HTTP credentials.
                </Text>
                <Input type="text" name="username" placeholder="Username" className="mb-4" />
                <Input type="password" name="password" placeholder="Password" className="mb-4" />
                <div className="d-flex justify-content-end">
                    <Button className="mr-2" disabled={loading} onClick={onDismiss} outline={true} variant="secondary">
                        Cancel
                    </Button>
                    <Button type="submit" disabled={loading} variant="primary">
                        Add account
                    </Button>
                </div>
            </Form>
        </Modal>
    )
}
