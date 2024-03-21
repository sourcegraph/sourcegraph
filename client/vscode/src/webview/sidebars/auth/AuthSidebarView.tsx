import React, { useCallback, useEffect, useMemo, useState } from 'react'

import { VSCodeButton, VSCodeLink } from '@vscode/webview-ui-toolkit/react'
import classNames from 'classnames'

import { currentAuthStateQuery } from '@sourcegraph/shared/src/auth'
import type { CurrentAuthStateResult, CurrentAuthStateVariables } from '@sourcegraph/shared/src/graphql-operations'
import { Alert, Text, Link, Input, H5, Form } from '@sourcegraph/wildcard'

import {
    VSCE_LINK_DOTCOM,
    VSCE_LINK_MARKETPLACE,
    VSCE_LINK_AUTH,
    VSCE_LINK_TOKEN_CALLBACK,
    VSCE_LINK_TOKEN_CALLBACK_TEST,
    VSCE_LINK_USER_DOCS,
    VSCE_SIDEBAR_PARAMS,
} from '../../../common/links'
import type { WebviewPageProps } from '../../platform/context'

import styles from './AuthSidebarView.module.scss'

interface AuthSidebarViewProps
    extends Pick<WebviewPageProps, 'extensionCoreAPI' | 'platformContext' | 'instanceURL' | 'authenticatedUser'> {}

interface AuthSidebarCtaProps extends Pick<WebviewPageProps, 'platformContext'> {}

/**
 * Rendered by sidebar in search-home state when user doesn't have a valid access token.
 */
export const AuthSidebarView: React.FunctionComponent<React.PropsWithChildren<AuthSidebarViewProps>> = ({
    instanceURL,
    extensionCoreAPI,
    platformContext,
    authenticatedUser,
}) => {
    const [state, setState] = useState<'initial' | 'validating' | 'success' | 'failure'>('initial')
    const [hasAccount, setHasAccount] = useState(authenticatedUser?.username !== undefined)
    const [usePrivateInstance, setUsePrivateInstance] = useState(true)
    const signUpURL = VSCE_LINK_AUTH('sign-up')
    const instanceHostname = useMemo(() => new URL(instanceURL).hostname, [instanceURL])
    const [hostname, setHostname] = useState(instanceHostname)
    const [accessToken, setAccessToken] = useState<string | undefined>('initial')
    const [endpointUrl, setEndpointUrl] = useState(instanceURL)
    const sourcegraphDotCom = 'https://www.sourcegraph.com'
    const isSourcegraphDotCom = useMemo(() => {
        const hostname = new URL(instanceURL).hostname
        if (hostname === 'sourcegraph.com' || hostname === 'www.sourcegraph.com') {
            return VSCE_LINK_TOKEN_CALLBACK
        }
        if (hostname === 'sourcegraph.test') {
            return VSCE_LINK_TOKEN_CALLBACK_TEST
        }
        return null
    }, [instanceURL])

    useEffect(() => {
        // Get access token from setting
        if (accessToken === 'initial') {
            extensionCoreAPI.getAccessToken
                .then(token => {
                    setAccessToken(token)
                    // If an access token and endpoint url exist at initial load,
                    // assumes the extension was started with a bad token because
                    // user should be autheticated automatically if token is valid
                    if (endpointUrl && token) {
                        extensionCoreAPI
                            .removeAccessToken()
                            .then(() => {
                                setAccessToken('REMOVED')
                                setState('failure')
                            })
                            .catch(() => {})
                    }
                })
                .catch(error => console.error(error))
        }
    }, [
        accessToken,
        endpointUrl,
        extensionCoreAPI,
        extensionCoreAPI.getAccessToken,
        extensionCoreAPI.removeAccessToken,
    ])

    const onTokenInputChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
        setAccessToken(event.target.value)
    }, [])

    const onInstanceURLInputChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
        setEndpointUrl(event.target.value)
    }, [])

    const onInstanceTypeChange = useCallback(() => {
        setUsePrivateInstance(!usePrivateInstance)
        if (!usePrivateInstance) {
            setEndpointUrl(sourcegraphDotCom)
        }
    }, [usePrivateInstance])

    const validateAccessToken: React.FormEventHandler<HTMLFormElement> = (event): void => {
        event.preventDefault()
        if (state !== 'validating' && accessToken) {
            const authStateVariables = {
                request: currentAuthStateQuery,
                variables: {},
                mightContainPrivateInfo: true,
                overrideAccessToken: accessToken,
                overrideSourcegraphURL: endpointUrl,
            }
            if (usePrivateInstance) {
                setHostname(new URL(endpointUrl).hostname)
                authStateVariables.overrideSourcegraphURL = endpointUrl
            }
            try {
                setState('validating')
                const currentAuthStateResult = platformContext
                    .requestGraphQL<CurrentAuthStateResult, CurrentAuthStateVariables>(authStateVariables)
                    .toPromise()
                currentAuthStateResult
                    .then(async result => {
                        if (!result) {
                            return
                        }

                        const { data } = result
                        if (data?.currentUser) {
                            await extensionCoreAPI.setEndpointUri(accessToken, endpointUrl)
                            setState('success')
                            return
                        }
                        setState('failure')
                        return
                    })
                    // v2/debt: Disambiguate network vs auth errors like we do in the browser extension.
                    .catch(() => {
                        setState('failure')
                    })
            } catch (error) {
                setState('failure')
                console.error(error)
            }
        }
        // If successful, update setting. This form will no longer be rendered
    }

    const onSignUpClick = (): void => {
        setHasAccount(true)
        platformContext.telemetryService.log('VSCESidebarCreateAccount')
    }

    if (state === 'success') {
        // This form should no longer be rendered as the extension context
        // will be invalidated. We should show a notification that the accessToken
        // has successfully been updated.
        return null
    }

    const renderCommon = (content: JSX.Element): JSX.Element => (
        <div className={classNames(styles.ctaContainer)}>
            <Form onSubmit={validateAccessToken}>
                <H5 className={styles.ctaTitle}>Search your private code</H5>
                {content}
            </Form>
        </div>
    )

    if (!hasAccount && !accessToken) {
        return renderCommon(
            <>
                <Text className={classNames(styles.ctaParagraph)}>
                    Create an account to search across your private repositories and access advanced features: search
                    multiple repositories & commit history, monitor code changes, save searches, and more.
                </Text>
                <div>
                    <Link to={signUpURL} className={styles.ctaLink}>
                        <VSCodeButton
                            onClick={onSignUpClick}
                            className={classNames(
                                'my-1 p-0',
                                styles.ctaButton,
                                styles.ctaButtonWrapperWithContextBelow
                            )}
                            autofocus={false}
                        >
                            Create an account
                        </VSCodeButton>
                    </Link>
                </div>
                <VSCodeLink className="my-0" onClick={() => setHasAccount(true)}>
                    Have an account?
                </VSCodeLink>
            </>
        )
    }

    enum InputStates {
        initial = 'initial',
        validating = 'loading',
        success = 'valid',
        failure = 'error',
    }

    return renderCommon(
        <>
            <Text className={classNames(styles.ctaParagraph)}>
                Sign in by entering an access token created through your user settings on Sourcegraph.
            </Text>
            <Text className={classNames(styles.ctaParagraph)}>
                See our {/* eslint-disable-next-line react/forbid-elements */}{' '}
                <a
                    href={VSCE_LINK_USER_DOCS}
                    onClick={() => platformContext.telemetryService.log('VSCESidebarCreateToken')}
                >
                    user docs
                </a>{' '}
                for a video guide on how to create an access token.
            </Text>
            {/* ---------- UNRELEASED FEATURE ---------- */}
            {isSourcegraphDotCom && authenticatedUser?.displayName === 'sourcegraph' && (
                <Text className={classNames(styles.ctaParagraph)}>
                    <Link to={isSourcegraphDotCom} className={styles.ctaLink}>
                        <VSCodeButton
                            onClick={onSignUpClick}
                            className={classNames(
                                'my-1 p-0',
                                styles.ctaButton,
                                styles.ctaButtonWrapperWithContextBelow
                            )}
                            autofocus={false}
                        >
                            Continue in browser
                        </VSCodeButton>
                    </Link>
                </Text>
            )}
            <Text className={classNames(styles.ctaButtonWrapperWithContextBelow)}>
                <Input
                    inputClassName={classNames('input', styles.ctaInput)}
                    id="access-token-input"
                    value={accessToken}
                    onChange={onTokenInputChange}
                    name="token"
                    required={true}
                    autoFocus={true}
                    spellCheck={false}
                    disabled={state === 'validating'}
                    placeholder="ex 6dfc880b320dff712d9f6cfcac5cbd13ebfad1d8"
                    label="Access Token"
                    className="mb-0"
                    status={InputStates[state]}
                />
            </Text>
            {usePrivateInstance && (
                <Text className={classNames(styles.ctaButtonWrapperWithContextBelow)}>
                    <Input
                        inputClassName={classNames('input', styles.ctaInput)}
                        id="instance-url-input"
                        value={endpointUrl}
                        type="url"
                        name="instance-url"
                        onChange={onInstanceURLInputChange}
                        required={true}
                        autoFocus={true}
                        spellCheck={false}
                        disabled={state === 'validating'}
                        placeholder="ex https://sourcegraph.example.com"
                        label="Sourcegraph Instance URL"
                        className="mb-0"
                        status={InputStates[state]}
                    />
                </Text>
            )}
            <VSCodeButton
                type="submit"
                disabled={state === 'validating'}
                className={classNames('my-1 p-0', styles.ctaButton, styles.ctaButtonWrapperWithContextBelow)}
            >
                Authenticate account
            </VSCodeButton>
            {state === 'failure' && (
                <Alert variant="danger" className={classNames(styles.ctaParagraph, 'my-1')}>
                    Unable to verify your access token for {hostname}. Please try again with a new access token or
                    restart VS Code if the instance URL has been updated.
                </Alert>
            )}
            <Text className="my-0">
                <VSCodeLink onClick={() => onInstanceTypeChange()}>
                    {!usePrivateInstance ? 'Need to connect to a private instance?' : 'Not a private instance user?'}
                </VSCodeLink>
            </Text>
            <Text className="my-0">
                <VSCodeLink href={signUpURL} onClick={onSignUpClick}>
                    Create an account
                </VSCodeLink>
            </Text>
        </>
    )
}

export const AuthSidebarCta: React.FunctionComponent<React.PropsWithChildren<AuthSidebarCtaProps>> = ({
    platformContext,
}) => {
    const onLinkClick = (type: 'Sourcegraph' | 'Extension'): void =>
        platformContext.telemetryService.log(`VSCESidebarLearn${type}Click`)

    return (
        <div>
            <H5 className={styles.ctaTitle}>Welcome</H5>
            <Text className={classNames(styles.ctaParagraph)}>
                The Sourcegraph extension allows you to search millions of open source repositories without cloning them
                to your local machine.
            </Text>
            <Text className={classNames(styles.ctaParagraph)}>
                Developers use Sourcegraph every day to onboard to new code bases, find code to reuse, resolve
                incidents, fix security vulnerabilities, and more.
            </Text>
            <div className={classNames(styles.ctaParagraph)}>
                <Text className="mb-0">Learn more:</Text>
                <VSCodeLink href={VSCE_LINK_DOTCOM + VSCE_SIDEBAR_PARAMS} onClick={() => onLinkClick('Sourcegraph')}>
                    Sourcegraph.com
                </VSCodeLink>
                <br />
                <VSCodeLink href={VSCE_LINK_MARKETPLACE} onClick={() => onLinkClick('Extension')}>
                    Sourcegraph VS Code extension
                </VSCodeLink>
            </div>
        </div>
    )
}
