import React, { useCallback, useEffect, useState } from 'react'

import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryService } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import { Alert, Form, Input, LoadingSpinner, Text, Badge, Link, useSessionStorage } from '@sourcegraph/wildcard'

import { BrandLogo } from '../../components/branding/BrandLogo'
import { useFeatureFlag } from '../../featureFlags/useFeatureFlag'
import { useURLSyncedString } from '../../hooks/useUrlSyncedString'
import { eventLogger } from '../../tracking/eventLogger'
import { DOTCOM_URL } from '../../tracking/util'
import { CodyIcon } from '../components/CodyIcon'
import { isEmailVerificationNeededForCody } from '../isCodyEnabled'

import { translateToQuery } from './translateToQuery'

import searchPageStyles from '../../storm/pages/SearchPage/SearchPageContent.module.scss'
import styles from './CodySearchPage.module.scss'

interface CodeSearchPageProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    telemetryService: TelemetryService
}

// Mapping for telemetry
const failureReasons = {
    untranslateable: 0,
    unreachable: 1,
}

export const CodySearchPage: React.FunctionComponent<CodeSearchPageProps> = ({
    authenticatedUser,
    telemetryRecorder,
}) => {
    useEffect(() => {
        eventLogger.logPageView('CodySearch')
        telemetryRecorder.recordEvent('cody.search', 'view')
    }, [telemetryRecorder])

    const navigate = useNavigate()

    /** The value entered by the user in the query input */
    // const [input, setInput] = useState('')
    const [input, setInput] = useURLSyncedString('cody-search', '')
    const codySearchStorage = useSessionStorage<string>('cody-search-input', '')
    const setCodySearchInput = codySearchStorage[1]

    const [inputError, setInputError] = useState<string | null>(null)

    const onInputChange = (newInput: string): void => {
        setInput(newInput)
        setInputError(null)
    }

    const [loading, setLoading] = useState(false)

    const onSubmit = useCallback(() => {
        const sanitizedInput = input.trim()
        const dotcomHost = DOTCOM_URL.href
        const isPrivateInstance = window.location.host !== dotcomHost

        if (!sanitizedInput) {
            return
        }

        eventLogger.log(
            'web:codySearch:submit',
            !isPrivateInstance ? { input: sanitizedInput } : null,
            !isPrivateInstance ? { input: sanitizedInput } : null
        )
        telemetryRecorder.recordEvent('cody.search', 'submit')
        setLoading(true)
        translateToQuery(sanitizedInput, authenticatedUser).then(
            query => {
                setLoading(false)

                if (query) {
                    eventLogger.log(
                        'web:codySearch:submitSucceeded',
                        !isPrivateInstance ? { input: sanitizedInput, translatedQuery: query } : null,
                        !isPrivateInstance ? { input: sanitizedInput, translatedQuery: query } : null
                    )
                    telemetryRecorder.recordEvent('cody.search', 'success')
                    setCodySearchInput(JSON.stringify({ input: sanitizedInput, translatedQuery: query }))
                    navigate({
                        pathname: '/search',
                        search: buildSearchURLQuery(query, SearchPatternType.regexp, false) + '&ref=cody-search',
                    })
                } else {
                    eventLogger.log(
                        'web:codySearch:submitFailed',
                        !isPrivateInstance ? { input: sanitizedInput, reason: 'untranslatable' } : null,
                        !isPrivateInstance ? { input: sanitizedInput, reason: 'untranslatable' } : null
                    )
                    telemetryRecorder.recordEvent('cody.search', 'fail', {
                        metadata: { reason: failureReasons.untranslateable },
                    })
                    setInputError('Cody does not understand this query. Try rephrasing it.')
                }
            },
            error => {
                telemetryRecorder.recordEvent('cody.search', 'fail')
                eventLogger.log(
                    'web:codySearch:submitFailed',
                    !isPrivateInstance
                        ? {
                              input: sanitizedInput,
                              reason: 'unreachable',
                              error: error?.message,
                          }
                        : null,
                    !isPrivateInstance
                        ? {
                              input: sanitizedInput,
                              reason: 'unreachable',
                              error: error?.message,
                          }
                        : null
                )
                telemetryRecorder.recordEvent('cody.search', 'fail', {
                    metadata: { reason: failureReasons.unreachable },
                })
                setLoading(false)
                setInputError(`Unable to reach Cody. Error: ${error?.message}`)
            }
        )
    }, [navigate, input, authenticatedUser, setCodySearchInput, telemetryRecorder])

    const isLightTheme = useIsLightTheme()

    return (
        <div className={classNames('d-flex flex-column align-items-center px-3', searchPageStyles.searchPage)}>
            <BrandLogo className={searchPageStyles.logo} isLightTheme={isLightTheme} variant="logo" />
            <div className="text-muted mt-3 mr-sm-2 pr-2 text-center">Searching millions of public repositories</div>
            <SearchInput
                value={input}
                onChange={onInputChange}
                onSubmit={onSubmit}
                loading={loading}
                error={inputError}
                className={classNames('mt-5 w-100', styles.inputContainer)}
            />
        </div>
    )
}

const SearchInput: React.FunctionComponent<{
    value: string
    loading: boolean
    error: string | null
    onChange: (value: string) => void
    onSubmit: () => void
    className?: string
}> = ({ value, loading, error, onChange, onSubmit: parentOnSubmit, className }) => {
    const onInput = useCallback<React.FormEventHandler<HTMLInputElement>>(
        event => {
            onChange(event.currentTarget.value)
        },
        [onChange]
    )

    const onSubmit = useCallback<React.FormEventHandler<HTMLFormElement>>(
        event => {
            event.preventDefault()
            parentOnSubmit()
        },
        [parentOnSubmit]
    )

    const [codySearchEnabled] = useFeatureFlag('cody-web-search')

    return codySearchEnabled ? (
        <Form onSubmit={onSubmit} className={className}>
            {isEmailVerificationNeededForCody() && (
                <Alert variant="warning">
                    <Text className="mb-0">Verify email</Text>
                    <Text className="mb-0">
                        Using Cody requires a verified email.{' '}
                        <Link to={`${window.context.currentUser?.settingsURL}/emails`} target="_blank" rel="noreferrer">
                            Resend email verification
                        </Link>
                        .
                    </Text>
                </Alert>
            )}
            <Input
                inputClassName={styles.input}
                value={value}
                onInput={onInput}
                disabled={loading || isEmailVerificationNeededForCody()}
                autoFocus={true}
                placeholder="Search for code or files in natural language..."
            />
            <div className="align-items-center d-flex mt-4 justify-content-center">
                <Text className="text-muted mb-0 mr-2" size="small">
                    Powered by Cody <CodyIcon />
                </Text>
                <Badge variant="warning">Experimental</Badge>
            </div>
            {error ? (
                <Alert variant="danger" className="mt-2 w-100">
                    {error}
                </Alert>
            ) : loading ? (
                <LoadingSpinner className="mt-2 d-block mx-auto" />
            ) : null}
        </Form>
    ) : (
        <Alert variant="info" className="mt-5">
            Cody is not enabled on this Sourcegraph instance.
        </Alert>
    )
}
