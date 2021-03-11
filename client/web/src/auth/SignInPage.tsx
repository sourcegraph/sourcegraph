import * as H from 'history'
import React, { useEffect, useState } from 'react'
import { Link, Redirect } from 'react-router-dom'
import { HeroPage } from '../components/HeroPage'
import { PageTitle } from '../components/PageTitle'
import { eventLogger } from '../tracking/eventLogger'
import { getReturnTo } from './SignInSignUpCommon'
import { UsernamePasswordSignInForm } from './UsernamePasswordSignInForm'
import { AuthenticatedUser } from '../auth'
import { SourcegraphIcon } from './icons'
import { partition } from 'lodash'
import { OrDivider } from './OrDivider'
import { ErrorAlert } from '../components/alerts'
import classNames from 'classnames'
import GithubIcon from 'mdi-react/GithubIcon'
import { SourcegraphContext } from '../jscontext'

interface SignInPageProps {
    location: H.Location
    history: H.History
    authenticatedUser: AuthenticatedUser | null
    context: Pick<
        SourcegraphContext,
        'allowSignup' | 'authProviders' | 'sourcegraphDotComMode' | 'xhrHeaders' | 'resetPasswordEnabled'
    >
}

export const SignInPage: React.FunctionComponent<SignInPageProps> = props => {
    useEffect(() => eventLogger.logViewEvent('SignIn', null, false))

    const [error, setError] = useState<Error | null>(null)

    if (props.authenticatedUser) {
        const returnTo = getReturnTo(props.location)
        return <Redirect to={returnTo} />
    }

    const [[builtInAuthProvider], thirdPartyAuthProviders] = partition(
        props.context.authProviders,
        provider => provider.isBuiltin
    )

    const body =
        !builtInAuthProvider && thirdPartyAuthProviders.length === 0 ? (
            <div className="alert alert-info mt-3">
                No authentication providers are available. Contact a site administrator for help.
            </div>
        ) : (
            <div className="mb-4 signin-page__container pb-5">
                {error && <ErrorAlert className="mt-4 mb-0 text-left" error={error} icon={false} />}
                <div
                    className={classNames(
                        'signin-signup-form signin-form test-signin-form rounded p-4 my-3',
                        error ? 'mt-3' : 'mt-4'
                    )}
                >
                    {builtInAuthProvider && (
                        <UsernamePasswordSignInForm
                            {...props}
                            onAuthError={setError}
                            noThirdPartyProviders={thirdPartyAuthProviders.length === 0}
                        />
                    )}
                    {builtInAuthProvider && thirdPartyAuthProviders.length > 0 && <OrDivider className="mb-3 py-1" />}
                    {thirdPartyAuthProviders.map((provider, index) => (
                        // Use index as key because display name may not be unique. This is OK
                        // here because this list will not be updated during this component's lifetime.
                        /* eslint-disable react/no-array-index-key */
                        <div className="mb-2" key={index}>
                            <a href={provider.authenticationURL} className="btn btn-secondary btn-block">
                                {provider.displayName === 'GitHub' && (
                                    <>
                                        <GithubIcon className="icon-inline" />{' '}
                                    </>
                                )}
                                Continue with {provider.displayName}
                            </a>
                        </div>
                    ))}
                </div>
                {props.context.allowSignup ? (
                    <p>
                        New to Sourcegraph? <Link to={`/sign-up${location.search}`}>Sign up</Link>
                    </p>
                ) : (
                    <p className="text-muted">Need an account? Contact your site admin</p>
                )}
            </div>
        )

    return (
        <div className="signin-signup-page sign-in-page web-content">
            <PageTitle title="Sign in" />
            <HeroPage
                icon={SourcegraphIcon}
                iconLinkTo={props.context.sourcegraphDotComMode ? '/search' : undefined}
                iconClassName="bg-transparent"
                lessPadding={true}
                title={
                    props.context.sourcegraphDotComMode
                        ? 'Sign in to Sourcegraph Cloud'
                        : 'Sign in to Sourcegraph Server'
                }
                body={body}
            />
        </div>
    )
}
