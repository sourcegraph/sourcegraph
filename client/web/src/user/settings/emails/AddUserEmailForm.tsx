import React, { FunctionComponent, useMemo, useState } from 'react'
import classNames from 'classnames'

import { AddUserEmailResult, AddUserEmailVariables } from '../../../graphql-operations'
import { gql, dataOrThrowErrors } from '../../../../../shared/src/graphql/graphql'
import { requestGraphQL } from '../../../backend/graphql'
import { asError, isErrorLike, ErrorLike } from '../../../../../shared/src/util/errors'
import { useInputValidation, deriveInputClassName } from '../../../../../shared/src/util/useInputValidation'

import { eventLogger } from '../../../tracking/eventLogger'
import { ErrorAlert } from '../../../components/alerts'
import { LoaderButton } from '../../../components/LoaderButton'
import { LoaderInput } from '../../../../../branded/src/components/LoaderInput'

interface Props {
    user: string
    onDidAdd: () => void

    className?: string
}

type Status = undefined | 'loading' | ErrorLike

export const AddUserEmailForm: FunctionComponent<Props> = ({ user, className, onDidAdd }) => {
    const [statusOrError, setStatusOrError] = useState<Status>()

    const [emailState, nextEmailFieldChange, emailInputReference, overrideEmailState] = useInputValidation(
        useMemo(
            () => ({
                synchronousValidators: [],
                asynchronousValidators: [],
            }),
            []
        )
    )

    const onSubmit: React.FormEventHandler<HTMLFormElement> = async event => {
        event.preventDefault()

        if (emailState.kind === 'VALID') {
            setStatusOrError('loading')

            try {
                dataOrThrowErrors(
                    await requestGraphQL<AddUserEmailResult, AddUserEmailVariables>(
                        gql`
                            mutation AddUserEmail($user: ID!, $email: String!) {
                                addUserEmail(user: $user, email: $email) {
                                    alwaysNil
                                }
                            }
                        `,
                        { user, email: emailState.value }
                    ).toPromise()
                )

                eventLogger.log('NewUserEmailAddressAdded')
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
        <div className={`add-user-email-form ${className || ''}`}>
            <label
                htmlFor="AddUserEmailForm-email"
                className={classNames('align-self-start', {
                    'text-danger font-weight-bold': emailState.kind === 'INVALID',
                })}
            >
                Email address
            </label>
            {/* eslint-disable-next-line react/forbid-elements */}
            <form className="form-inline" onSubmit={onSubmit} noValidate={true}>
                <LoaderInput
                    className={classNames(deriveInputClassName(emailState), 'mr-sm-2')}
                    loading={emailState.kind === 'LOADING'}
                >
                    <input
                        id="AddUserEmailForm-email"
                        type="email"
                        name="email"
                        className={classNames(
                            'form-control test-user-email-add-input',
                            deriveInputClassName(emailState)
                        )}
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
                    />
                </LoaderInput>
                <LoaderButton
                    loading={statusOrError === 'loading'}
                    label="Add"
                    type="submit"
                    disabled={statusOrError === 'loading' || emailState.kind !== 'VALID'}
                    className="btn btn-primary"
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
