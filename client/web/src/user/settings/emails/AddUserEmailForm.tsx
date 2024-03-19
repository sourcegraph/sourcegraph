import React, { type FunctionComponent, useMemo, useState } from 'react'

import classNames from 'classnames'
import { lastValueFrom } from 'rxjs'

import { asError, isErrorLike, type ErrorLike } from '@sourcegraph/common'
import { gql, dataOrThrowErrors } from '@sourcegraph/http-client'
import { deriveInputClassName, useInputValidation } from '@sourcegraph/shared/src/util/useInputValidation'
import { screenReaderAnnounce, Input, Label, ErrorAlert } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../../backend/graphql'
import { LoaderButton } from '../../../components/LoaderButton'
import type { AddUserEmailResult, AddUserEmailVariables, UserSettingsAreaUserFields } from '../../../graphql-operations'
import { eventLogger } from '../../../tracking/eventLogger'

interface Props {
    user: Pick<UserSettingsAreaUserFields, 'id' | 'scimControlled'>
    onDidAdd: () => void
    emails: Set<string>

    className?: string
}

type Status = undefined | 'loading' | ErrorLike

enum InputState {
    NOT_VALIDATED = 'initial',
    LOADING = 'loading',
    VALID = 'valid',
    INVALID = 'error',
}

export const AddUserEmailForm: FunctionComponent<React.PropsWithChildren<Props>> = ({
    user,
    className,
    onDidAdd,
    emails,
}) => {
    const [statusOrError, setStatusOrError] = useState<Status>()

    const [emailState, nextEmailFieldChange, emailInputReference, overrideEmailState] = useInputValidation(
        useMemo(
            () => ({
                synchronousValidators: [email => validateEmail(email, emails)],
                asynchronousValidators: [],
            }),
            [emails]
        )
    )

    const onSubmit: React.FormEventHandler<HTMLFormElement> = async event => {
        event.preventDefault()

        if (emailState.kind === 'VALID') {
            setStatusOrError('loading')

            try {
                dataOrThrowErrors(
                    await lastValueFrom(
                        requestGraphQL<AddUserEmailResult, AddUserEmailVariables>(
                            gql`
                                mutation AddUserEmail($user: ID!, $email: String!) {
                                    addUserEmail(user: $user, email: $email) {
                                        alwaysNil
                                    }
                                }
                            `,
                            { user: user.id, email: emailState.value }
                        )
                    )
                )

                eventLogger.log('NewUserEmailAddressAdded')
                screenReaderAnnounce('Email address added')

                overrideEmailState({ value: '' })
                setStatusOrError(undefined)

                if (onDidAdd) {
                    onDidAdd()
                }
            } catch (error) {
                setStatusOrError(asError(error))
            }
        }
    }

    return (
        <div className={classNames('add-user-email-form', className)}>
            <Label
                htmlFor="AddUserEmailForm-email"
                className={classNames('align-self-start', {
                    'text-danger font-weight-bold': emailState.kind === 'INVALID',
                })}
            >
                Add email address
            </Label>
            {/* eslint-disable-next-line react/forbid-elements */}
            <form className="form-inline" onSubmit={onSubmit} noValidate={true}>
                <Input
                    id="AddUserEmailForm-email"
                    type="email"
                    name="email"
                    inputClassName={deriveInputClassName(emailState)}
                    onChange={nextEmailFieldChange}
                    size={32}
                    value={emailState.value}
                    ref={emailInputReference}
                    required={true}
                    autoComplete="email"
                    autoCorrect="off"
                    autoCapitalize="off"
                    spellCheck={false}
                    readOnly={false}
                    status={InputState[emailState.kind]}
                    disabled={user.scimControlled}
                    className={classNames(deriveInputClassName(emailState), 'mr-sm-2')}
                />
                <LoaderButton
                    loading={statusOrError === 'loading'}
                    label="Add"
                    type="submit"
                    disabled={statusOrError === 'loading' || emailState.kind !== 'VALID' || user.scimControlled}
                    variant="primary"
                />
                {emailState.kind === 'INVALID' && (
                    <small className="invalid-feedback" role="alert">
                        {emailState.reason}
                    </small>
                )}
            </form>
            {isErrorLike(statusOrError) && <ErrorAlert className="mt-2" error={statusOrError} />}
        </div>
    )
}

const validateEmail = (email: string, existingEmails: Set<string>): string | undefined =>
    existingEmails.has(email) ? 'Email already exists' : undefined
