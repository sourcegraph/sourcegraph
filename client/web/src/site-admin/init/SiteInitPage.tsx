import React from 'react'
import { Redirect } from 'react-router'
import { SignUpArguments, SignUpForm } from '../../auth/SignUpForm'
import { submitTrialRequest } from '../../marketing/backend'
import { BrandLogo } from '../../components/branding/BrandLogo'
import { ThemeProps } from '../../../../shared/src/theme'
import { AuthenticatedUser } from '../../auth'
import { SourcegraphContext } from '../../jscontext'

const initSite = async (args: SignUpArguments): Promise<void> => {
    const pingUrl = new URL('https://sourcegraph.com/ping-from-self-hosted')
    pingUrl.searchParams.set('email', args.email)

    await fetch(pingUrl.toString(), {
        credentials: 'include',
    })
        .then() // no op
        .catch((error): void => {
            console.error(error)
        })
    const response = await fetch('/-/site-init', {
        credentials: 'same-origin',
        method: 'POST',
        headers: {
            ...window.context.xhrHeaders,
            Accept: 'application/json',
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(args),
    })
    if (response.status !== 200) {
        const text = await response.text()
        throw new Error(text)
    }
    if (args.requestedTrial) {
        submitTrialRequest(args.email)
    }
    window.location.replace('/site-admin')
}

interface Props extends ThemeProps {
    authenticatedUser: Pick<AuthenticatedUser, 'username'> | null

    /**
     * Whether site initialization is needed. If not set, the global value from
     * `window.context.needsSiteInit` is used.
     */
    needsSiteInit?: typeof window.context.needsSiteInit
    context: Pick<SourcegraphContext, 'sourcegraphDotComMode' | 'authProviders'>
}

/**
 * A page that is shown when the Sourcegraph instance has not yet been initialized.
 * Only the person who first accesses the instance will see this.
 */
export const SiteInitPage: React.FunctionComponent<Props> = ({
    authenticatedUser,
    isLightTheme,
    needsSiteInit = window.context.needsSiteInit,
    context,
}) => {
    if (!needsSiteInit) {
        return <Redirect to="/search" />
    }

    return (
        <div className="site-init-page">
            <div className="site-init-page__content card">
                <div className="card-body p-4">
                    <BrandLogo className="w-100 mb-3" isLightTheme={isLightTheme} variant="logo" />
                    {authenticatedUser ? (
                        // If there's already a user but the site is not initialized, then the we're in an
                        // unexpected state, likely because of a previous bug or because someone manually modified
                        // the site_config DB table.
                        <p>
                            You're signed in as <strong>{authenticatedUser.username}</strong>. A site admin must
                            initialize Sourcegraph before you can continue.
                        </p>
                    ) : (
                        <>
                            <h2 className="site-init-page__header">Welcome</h2>
                            <p>Create an admin account to start using Sourcegraph.</p>
                            <SignUpForm
                                className="w-100"
                                buttonLabel="Create admin account & continue"
                                doSignUp={initSite}
                                context={context}
                            />
                        </>
                    )}
                </div>
            </div>
        </div>
    )
}
