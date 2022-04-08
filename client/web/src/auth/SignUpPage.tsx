import React, { useEffect } from 'react'

import classNames from 'classnames'
import { Redirect, useLocation } from 'react-router-dom'

import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'
import { Link } from '@sourcegraph/wildcard'

import { AuthenticatedUser } from '../auth'
import { HeroPage } from '../components/HeroPage'
import { PageTitle } from '../components/PageTitle'
import { FeatureFlagProps } from '../featureFlags/featureFlags'
import { SourcegraphContext } from '../jscontext'
import { eventLogger } from '../tracking/eventLogger'

import { CloudSignUpPage, ShowEmailFormQueryParameter } from './CloudSignUpPage'
import { SourcegraphIcon } from './icons'
import { getReturnTo, maybeAddPostSignUpRedirect } from './SignInSignUpCommon'
import { SignUpArguments, SignUpForm } from './SignUpForm'
import { VsCodeSignUpPage } from './VsCodeSignUpPage'

import signInSignUpCommonStyles from './SignInSignUpCommon.module.scss'

export interface SignUpPageProps extends ThemeProps, TelemetryProps, FeatureFlagProps {
    authenticatedUser: AuthenticatedUser | null
    context: Pick<
        SourcegraphContext,
        'allowSignup' | 'experimentalFeatures' | 'authProviders' | 'sourcegraphDotComMode' | 'xhrHeaders'
    >
}

export const SignUpPage: React.FunctionComponent<SignUpPageProps> = ({
    authenticatedUser,
    context,
    isLightTheme,
    telemetryService,
    featureFlags,
}) => {
    const location = useLocation()
    const query = new URLSearchParams(location.search)
    const invitedBy = query.get('invitedBy')
    const returnTo = getReturnTo(location)

    useEffect(() => {
        eventLogger.logViewEvent('SignUp', null, false)

        if (invitedBy !== null) {
            const parameters = {
                isAuthenticated: !!authenticatedUser,
                allowSignup: context.allowSignup,
            }
            eventLogger.log('SignUpInvitedByUser', parameters, parameters)
        }
    }, [invitedBy, authenticatedUser, context.allowSignup])

    if (authenticatedUser) {
        return <Redirect to={returnTo} />
    }

    if (!context.allowSignup) {
        return <Redirect to="/sign-in" />
    }

    let newUserFromEmailInvitation = false
    if (context.sourcegraphDotComMode && returnTo.includes('/organizations/invitation/')) {
        newUserFromEmailInvitation = true
    }

    const handleSignUp = (args: SignUpArguments): Promise<void> =>
        fetch('/-/sign-up', {
            credentials: 'same-origin',
            method: 'POST',
            headers: {
                ...context.xhrHeaders,
                Accept: 'application/json',
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(args),
        }).then(response => {
            if (response.status !== 200) {
                return response.text().then(text => Promise.reject(new Error(text)))
            }

            // if sign up is successful and enablePostSignupFlow feature is ON
            // and user is not signing up from an email invitation to join an org -
            // redirect user to the /post-sign-up page
            if (context.experimentalFeatures.enablePostSignupFlow && !newUserFromEmailInvitation) {
                window.location.replace(new URL(maybeAddPostSignUpRedirect(), window.location.href).pathname)
            } else {
                window.location.replace(returnTo)
            }

            return Promise.resolve()
        })

    if (query.get('editor') === 'vscode') {
        return (
            <VsCodeSignUpPage
                source={query.get('src')}
                onSignUp={handleSignUp}
                isLightTheme={isLightTheme}
                showEmailForm={query.has(ShowEmailFormQueryParameter)}
                context={context}
                telemetryService={telemetryService}
                featureFlags={featureFlags}
            />
        )
    }

    if (context.sourcegraphDotComMode) {
        return (
            <CloudSignUpPage
                source={query.get('src')}
                onSignUp={handleSignUp}
                isLightTheme={isLightTheme}
                showEmailForm={query.has(ShowEmailFormQueryParameter)}
                context={context}
                telemetryService={telemetryService}
                featureFlags={featureFlags}
            />
        )
    }

    return (
        <div className={signInSignUpCommonStyles.signinSignupPage}>
            <PageTitle title="Sign up" />
            <HeroPage
                icon={SourcegraphIcon}
                iconLinkTo={context.sourcegraphDotComMode ? '/search' : undefined}
                iconClassName="bg-transparent"
                title={
                    context.sourcegraphDotComMode ? 'Sign up for Sourcegraph Cloud' : 'Sign up for Sourcegraph Server'
                }
                lessPadding={true}
                body={
                    <div className={classNames('pb-5', signInSignUpCommonStyles.signupPageContainer)}>
                        {context.sourcegraphDotComMode && <p className="pt-1 pb-2">Start searching public code now</p>}
                        <SignUpForm featureFlags={featureFlags} context={context} onSignUp={handleSignUp} />
                        <p className="mt-3">
                            Already have an account? <Link to={`/sign-in${location.search}`}>Sign in</Link>
                        </p>
                    </div>
                }
            />
        </div>
    )
}
