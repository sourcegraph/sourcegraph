import React, { useEffect, useState } from 'react'

import { mdiBitbucket, mdiChevronLeft, mdiGithub, mdiGitlab, mdiKeyVariant, mdiMicrosoftAzureDevops } from '@mdi/js'
import classNames from 'classnames'
import { partition } from 'lodash'
import { Navigate, useLocation, useSearchParams } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import { Alert, Icon, Text, Link, Button, ErrorAlert, AnchorLink, Container } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { PageTitle } from '../components/PageTitle'
import type { AuthProvider, SourcegraphContext } from '../jscontext'
import { checkRequestAccessAllowed } from '../util/checkRequestAccessAllowed'

import { AuthPageWrapper } from './AuthPageWrapper'
import { OrDivider } from './OrDivider'
import { getReturnTo } from './SignInSignUpCommon'
import { UsernamePasswordSignInForm } from './UsernamePasswordSignInForm'

import styles from './SignInPage.module.scss'

export interface SignInPageProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    context: Pick<
        SourcegraphContext,
        | 'allowSignup'
        | 'authProviders'
        | 'sourcegraphDotComMode'
        | 'primaryLoginProvidersCount'
        // needed for checkRequestAccessAllowed
        | 'authAccessRequest'
        // needed for UsernamePasswordSignInForm
        | 'xhrHeaders'
        | 'resetPasswordEnabled'
    >
}

export const SignInPage: React.FunctionComponent<React.PropsWithChildren<SignInPageProps>> = props => {
    const { context, authenticatedUser } = props
    useEffect(() => {
        EVENT_LOGGER.logViewEvent('SignIn', null, false)
        props.telemetryRecorder.recordEvent('auth.signIn', 'view')
    }, [props.telemetryRecorder])

    const location = useLocation()
    const email = new URLSearchParams(location.search).get('email')
    const [error, setError] = useState<Error | null>(null)
    const [searchParams, setSearchParams] = useSearchParams()
    const isRequestAccessAllowed = checkRequestAccessAllowed(props.context)

    const returnTo = getReturnTo(location)
    if (authenticatedUser) {
        return <Navigate to={returnTo} replace={true} />
    }

    const [[builtInAuthProvider], nonBuiltinAuthProviders] = partition(
        context.authProviders,
        provider => provider.isBuiltin
    )

    const shouldShowProvider = function (provider: AuthProvider): boolean {
        if (provider.noSignIn) {
            return false
        }

        // Hide the Sourcegraph Operator authentication provider by default because it is
        // not useful to customer users and may even cause confusion.
        if (provider.serviceType === 'sourcegraph-operator') {
            return searchParams.has('sourcegraph-operator')
        }
        if (provider.serviceType === 'gerrit') {
            return false
        }
        return true
    }

    const toggleMoreProviders = (showMore: boolean): void => {
        const param = 'showMore'
        if (showMore) {
            searchParams.set(param, '')
        } else {
            searchParams.delete(param)
        }
        setSearchParams(searchParams)
    }

    const thirdPartyAuthProviders = nonBuiltinAuthProviders.filter(provider => shouldShowProvider(provider))
    // If there is only one auth provider that is going to be displayed, and request access is disabled, we want to redirect to the auth provider directly.
    if (context.sourcegraphDotComMode && thirdPartyAuthProviders.length === 1) {
        // Add '?returnTo=' + encodeURIComponent(returnTo) to thirdPartyAuthProviders[0].authenticationURL in a safe way.
        const redirectUrl = new URL(thirdPartyAuthProviders[0].authenticationURL, window.location.href)
        if (returnTo) {
            redirectUrl.searchParams.set('returnTo', new URL(returnTo, window.location.href).toString())
        }
        window.location.replace(redirectUrl)

        return (
            <>
                <PageTitle title="Signing in..." />
                <AuthPageWrapper
                    title="Redirecting to sign in..."
                    sourcegraphDotComMode={context.sourcegraphDotComMode}
                    className={styles.wrapper}
                >
                    <Alert className="mt-3" variant="info">
                        You are being redirected to sign in with {thirdPartyAuthProviders[0].displayName}.
                    </Alert>
                </AuthPageWrapper>
            </>
        )
    }
    const primaryProviders = thirdPartyAuthProviders.slice(0, context.primaryLoginProvidersCount)
    const moreProviders = thirdPartyAuthProviders.slice(context.primaryLoginProvidersCount)

    const showMoreProviders = searchParams.has('showMore') && (builtInAuthProvider || moreProviders.length > 0)
    const hasProviders = builtInAuthProvider || thirdPartyAuthProviders.length > 0
    const showMoreWaysToLogin =
        !showMoreProviders && (moreProviders.length > 0 || (primaryProviders.length > 0 && builtInAuthProvider))

    const providers = showMoreProviders ? moreProviders : primaryProviders

    const body = !hasProviders ? (
        <Alert className="mt-3" variant="info">
            No authentication providers are available. Contact a site administrator for help.
        </Alert>
    ) : (
        <>
            {error && <ErrorAlert error={error} />}
            <Container className="test-signin-form">
                {showMoreProviders && (
                    <div className="mb-3 text-left">
                        <Button
                            variant="link"
                            className="p-0 border-0 font-weight-normal"
                            onClick={() => toggleMoreProviders(false)}
                        >
                            <Icon aria-hidden={true} svgPath={mdiChevronLeft} />
                            Back
                        </Button>
                    </div>
                )}
                {builtInAuthProvider && (showMoreProviders || thirdPartyAuthProviders.length === 0) && (
                    <UsernamePasswordSignInForm
                        {...props}
                        email={email}
                        onAuthError={setError}
                        className={classNames({ 'mb-3': providers.length > 0 })}
                    />
                )}
                {builtInAuthProvider && showMoreProviders && providers.length > 0 && (
                    <OrDivider className="mb-3 py-1" />
                )}
                {providers.map((provider, index) => {
                    // Use index as key because display name may not be unique. This is OK
                    // here because this list will not be updated during this component's lifetime.
                    /* eslint-disable react/no-array-index-key */
                    const authURL = new URL(provider.authenticationURL, window.location.href)
                    if (returnTo) {
                        // propagate return to callback parameter
                        authURL.searchParams.set('returnTo', returnTo)
                    }

                    return (
                        // Only add botton margin to every but the last providers.
                        <div className={classNames(index !== providers.length - 1 && 'mb-2')} key={index}>
                            <Button
                                to={authURL.toString()}
                                display="block"
                                variant={showMoreProviders ? 'secondary' : 'primary'}
                                as={AnchorLink}
                            >
                                <ProviderIcon serviceType={provider.serviceType} />{' '}
                                {provider.displayPrefix ?? 'Continue with'} {provider.displayName}
                            </Button>
                        </div>
                    )
                })}
                {showMoreWaysToLogin && (
                    <Button
                        display="block"
                        className="mt-2"
                        variant="secondary"
                        onClick={() => toggleMoreProviders(true)}
                    >
                        <Icon aria-hidden={true} svgPath={mdiKeyVariant} /> Other login methods
                    </Button>
                )}
            </Container>
            <SignUpNotice
                allowSignup={context.allowSignup}
                isRequestAccessAllowed={isRequestAccessAllowed}
                sourcegraphDotComMode={context.sourcegraphDotComMode}
                telemetryRecorder={props.telemetryRecorder}
            />
        </>
    )

    return (
        <>
            <PageTitle title="Sign in" />
            <AuthPageWrapper
                title="Sign in to Sourcegraph"
                sourcegraphDotComMode={context.sourcegraphDotComMode}
                className={styles.wrapper}
            >
                {body}
            </AuthPageWrapper>
        </>
    )
}

const ProviderIcon: React.FunctionComponent<{ serviceType: AuthProvider['serviceType'] }> = ({ serviceType }) => {
    switch (serviceType) {
        case 'github': {
            return <Icon aria-hidden={true} svgPath={mdiGithub} />
        }
        case 'gitlab': {
            return <Icon aria-hidden={true} svgPath={mdiGitlab} />
        }
        case 'bitbucketCloud': {
            return <Icon aria-hidden={true} svgPath={mdiBitbucket} />
        }
        case 'bitbucketServer': {
            return <Icon aria-hidden={true} svgPath={mdiBitbucket} />
        }
        case 'azuredevops': {
            return <Icon aria-hidden={true} svgPath={mdiMicrosoftAzureDevops} />
        }
        default: {
            return null
        }
    }
}

interface SignUpNoticeProps extends TelemetryV2Props {
    allowSignup: boolean
    sourcegraphDotComMode: boolean
    isRequestAccessAllowed: boolean
}

const SignUpNotice: React.FunctionComponent<SignUpNoticeProps> = ({
    allowSignup,
    sourcegraphDotComMode,
    isRequestAccessAllowed,
    telemetryRecorder,
}) => {
    const dotcomCTAs = (
        <>
            <Link
                to="https://sourcegraph.com/get-started?t=enterprise"
                onClick={() => {
                    EVENT_LOGGER.log('ClickedOnEnterpriseCTA', { location: 'SignInPage' })
                    telemetryRecorder.recordEvent('auth.enterpriseCTA', 'click')
                }}
            >
                consider Sourcegraph Enterprise
            </Link>
            .
        </>
    )

    if (allowSignup) {
        return (
            <Text className="mt-3 text-center">
                New to Sourcegraph? <Link to="/sign-up">Sign up</Link>{' '}
                {sourcegraphDotComMode && <>To use Sourcegraph on private repositories, {dotcomCTAs}</>}
            </Text>
        )
    }

    if (isRequestAccessAllowed) {
        return (
            <Text className="mt-3 text-center text-muted">
                Need an account? <Link to="/request-access">Request access</Link> or contact your site admin.
            </Text>
        )
    }

    if (sourcegraphDotComMode) {
        return (
            <Text className="mt-3 text-center text-muted">
                Currently, we are unable to create accounts using email. Please use the providers listed above to
                continue. <br /> For private code, {dotcomCTAs}
            </Text>
        )
    }

    return <Text className="mt-3 text-center text-muted">Need an account? Contact your site admin.</Text>
}
