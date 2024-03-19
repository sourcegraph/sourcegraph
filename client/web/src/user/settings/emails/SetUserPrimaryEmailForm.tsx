import React, { useState, type FunctionComponent, useCallback } from 'react'

import classNames from 'classnames'
import { lastValueFrom } from 'rxjs'

import { asError, type ErrorLike, isErrorLike } from '@sourcegraph/common'
import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'
import { Select, ErrorAlert, Form } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../../backend/graphql'
import { LoaderButton } from '../../../components/LoaderButton'
import type {
    SetUserEmailPrimaryResult,
    SetUserEmailPrimaryVariables,
    UserEmailsResult,
    UserSettingsAreaUserFields,
} from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'

import styles from './SetUserPrimaryEmailForm.module.scss'

type UserEmail = (NonNullable<UserEmailsResult['node']> & { __typename: 'User' })['emails'][number]

interface Props {
    user: Pick<UserSettingsAreaUserFields, 'id' | 'scimControlled'>
    emails: UserEmail[]
    onDidSet: () => void

    className?: string
}

type Status = undefined | 'loading' | ErrorLike

const findPrimaryEmail = (emails: UserEmail[]): string | undefined => emails.find(email => email.isPrimary)?.email

export const SetUserPrimaryEmailForm: FunctionComponent<React.PropsWithChildren<Props>> = ({
    user,
    emails,
    onDidSet,
    className,
}) => {
    const currentPrimaryEmail = findPrimaryEmail(emails)
    const [primaryEmail, setPrimaryEmail] = useState<string | undefined>(currentPrimaryEmail)
    const [statusOrError, setStatusOrError] = useState<Status>()

    // options should include all verified emails + a primary one
    const options = emails.filter(email => email.verified || email.isPrimary).map(email => email.email)

    const onPrimaryEmailSelect: React.ChangeEventHandler<HTMLSelectElement> = event =>
        setPrimaryEmail(event.target.value)

    const onSubmit: React.FormEventHandler<HTMLFormElement> = useCallback(
        async event => {
            event.preventDefault()
            if (primaryEmail === undefined) {
                return
            }
            setStatusOrError('loading')

            try {
                dataOrThrowErrors(
                    await lastValueFrom(
                        requestGraphQL<SetUserEmailPrimaryResult, SetUserEmailPrimaryVariables>(
                            gql`
                                mutation SetUserEmailPrimary($user: ID!, $email: String!) {
                                    setUserEmailPrimary(user: $user, email: $email) {
                                        alwaysNil
                                    }
                                }
                            `,
                            { user: user.id, email: primaryEmail }
                        )
                    )
                )

                eventLogger.log('UserEmailAddressSetAsPrimary')
                setStatusOrError(undefined)

                if (onDidSet) {
                    onDidSet()
                }
            } catch (error) {
                setStatusOrError(asError(error))
            }
        },
        [user, primaryEmail, onDidSet]
    )

    return (
        <div className={classNames('add-user-email-form', className)}>
            <Form onSubmit={onSubmit}>
                <div className={styles.formLine}>
                    <div className={styles.formSelect}>
                        <Select
                            label="Primary email address"
                            labelVariant="block"
                            id="setUserPrimaryEmailForm-email"
                            isCustomStyle={true}
                            selectClassName="mr-sm-2"
                            value={primaryEmail}
                            onChange={onPrimaryEmailSelect}
                            required={true}
                            disabled={
                                (options.length === 1 && !!currentPrimaryEmail) ||
                                statusOrError === 'loading' ||
                                user.scimControlled
                            }
                        >
                            {/* If no primary email is selected yet, we add an empty option to indicate nothing was selected. */}
                            {!currentPrimaryEmail && <option key="" />}
                            {options.map(email => (
                                <option key={email} value={email}>
                                    {email}
                                </option>
                            ))}
                        </Select>
                    </div>
                    <div className={styles.formButton}>
                        <LoaderButton
                            loading={statusOrError === 'loading'}
                            label="Save"
                            type="submit"
                            disabled={
                                // In case no email is marked primary yet, and none
                                // has been selected from the dropdown yet.
                                !primaryEmail ||
                                (options.length === 1 && !!currentPrimaryEmail) ||
                                statusOrError === 'loading' ||
                                user.scimControlled
                            }
                            variant="primary"
                        />
                    </div>
                </div>
            </Form>
            {isErrorLike(statusOrError) && <ErrorAlert className="mt-2" error={statusOrError} />}
        </div>
    )
}
