import React, { useState, FunctionComponent, useCallback } from 'react'

import { requestGraphQL } from '../../../backend/graphql'
import { gql, dataOrThrowErrors } from '../../../../../shared/src/graphql/graphql'
import { SetUserEmailPrimaryResult, SetUserEmailPrimaryVariables, UserEmailsResult } from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'
import { asError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'

import { Form } from '../../../../../branded/src/components/Form'
import { LoaderButton } from '../../../components/LoaderButton'
import { ErrorAlert } from '../../../components/alerts'

type UserEmail = NonNullable<UserEmailsResult['node']>['emails'][number]

interface Props {
    user: string
    emails: UserEmail[]
    onDidSet: () => void

    className?: string
}

type Status = undefined | 'loading' | ErrorLike

// There is always exactly one primary email returned from the backend
// eslint-disable-next-line @typescript-eslint/no-non-null-assertion
const findPrimaryEmail = (emails: UserEmail[]): string => emails.find(email => email.isPrimary)!.email

export const SetUserPrimaryEmailForm: FunctionComponent<Props> = ({ user, emails, onDidSet, className }) => {
    const [primaryEmail, setPrimaryEmail] = useState<string>(findPrimaryEmail(emails))
    const [statusOrError, setStatusOrError] = useState<Status>()

    // options should include all verified emails + a primary one
    const options = emails.filter(email => email.verified || email.isPrimary).map(email => email.email)

    const onPrimaryEmailSelect: React.ChangeEventHandler<HTMLSelectElement> = event =>
        setPrimaryEmail(event.target.value)

    const onSubmit: React.FormEventHandler<HTMLFormElement> = useCallback(
        async event => {
            event.preventDefault()
            setStatusOrError('loading')

            try {
                dataOrThrowErrors(
                    await requestGraphQL<SetUserEmailPrimaryResult, SetUserEmailPrimaryVariables>(
                        gql`
                            mutation SetUserEmailPrimary($user: ID!, $email: String!) {
                                setUserEmailPrimary(user: $user, email: $email) {
                                    alwaysNil
                                }
                            }
                        `,
                        { user, email: primaryEmail }
                    ).toPromise()
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
        <div className={`add-user-email-form ${className || ''}`}>
            <p className="mb-2">Primary email address</p>
            <Form className="form-inline" onSubmit={onSubmit}>
                <label className="sr-only" htmlFor="setUserPrimaryEmailForm-email">
                    Email address
                </label>
                <select
                    id="setUserPrimaryEmailForm-email"
                    className="custom-select form-control-lg mr-sm-2"
                    value={primaryEmail}
                    onBlur={onPrimaryEmailSelect}
                    required={true}
                    disabled={options.length === 1 || statusOrError === 'loading'}
                >
                    {options.map(email => (
                        <option key={email} value={email}>
                            {email}
                        </option>
                    ))}
                </select>
                <LoaderButton
                    loading={statusOrError === 'loading'}
                    label="Save"
                    type="submit"
                    disabled={options.length === 1 || statusOrError === 'loading'}
                    className="btn btn-primary"
                />
            </Form>
            {isErrorLike(statusOrError) && <ErrorAlert className="mt-2" error={statusOrError} />}
        </div>
    )
}
