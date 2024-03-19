import React from 'react'

import { mdiChevronLeft } from '@mdi/js'
import classNames from 'classnames'
import { useLocation } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Link, Icon, H2 } from '@sourcegraph/wildcard'

import { BrandLogo } from '../components/branding/BrandLogo'
import type { SourcegraphContext } from '../jscontext'

import { ExternalsAuth } from './components/ExternalsAuth'
import { type SignUpArguments, SignUpForm } from './SignUpForm'

import styles from './VsCodeSignUpPage.module.scss'

export const ShowEmailFormQueryParameter = 'showEmail'

export interface VsCodeSignUpPageProps extends TelemetryProps, TelemetryV2Props {
    source: string | null
    showEmailForm: boolean
    /** Called to perform the signup on the server. */
    onSignUp: (args: SignUpArguments) => Promise<void>
    context: Pick<SourcegraphContext, 'externalURL' | 'authMinPasswordLength'>
}

const VSCodeIcon: React.FC = () => (
    <svg width="30" height="30" viewBox="0 0 30 30" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path
            d="M22.0834 21.3325V8.46915L13.5834 14.9008L22.0834 21.3325ZM1.14506 11.0192C0.939444 10.7993 0.822721 10.511 0.817486 10.21C0.812252 9.90905 0.918879 9.61685 1.11672 9.38999L2.81672 7.81749C3.10006 7.56249 3.79422 7.44915 4.30422 7.81749L9.14922 11.515L20.3834 1.24415C20.8367 0.79082 21.6159 0.606653 22.5084 1.07415L28.1751 3.77999C28.6851 4.07749 29.1667 4.54499 29.1667 5.40915V24.5342C29.1667 25.1008 28.7559 25.71 28.3167 25.9508L22.0834 28.9258C21.6301 29.11 20.7801 28.94 20.4826 28.6425L9.12089 18.3008L4.30422 21.9842C3.76589 22.3525 3.10006 22.2533 2.81672 21.9842L1.11672 20.4258C0.663389 19.9583 0.720056 19.1933 1.18756 18.7258L5.43756 14.9008"
            fill="#339AF0"
        />
    </svg>
)

/**
 * Sign up page specifically from users via our VS Code integration
 */
export const VsCodeSignUpPage: React.FunctionComponent<React.PropsWithChildren<VsCodeSignUpPageProps>> = ({
    showEmailForm,
    onSignUp,
    context,
    telemetryService,
    telemetryRecorder,
}) => {
    const isLightTheme = useIsLightTheme()
    const location = useLocation()

    const queryWithUseEmailToggled = new URLSearchParams(location.search)
    if (showEmailForm) {
        queryWithUseEmailToggled.delete(ShowEmailFormQueryParameter)
    } else {
        queryWithUseEmailToggled.append(ShowEmailFormQueryParameter, 'true')
    }

    const assetsRoot = window.context?.assetsRoot || ''

    const signUpForm = (
        <SignUpForm
            onSignUp={args => onSignUp(args)}
            context={{
                authProviders: [],
                sourcegraphDotComMode: true,
                authMinPasswordLength: context.authMinPasswordLength,
            }}
            buttonLabel="Sign up"
            experimental={true}
            className="my-3"
            telemetryRecorder={telemetryRecorder}
        />
    )

    const renderCodeHostAuth = (): JSX.Element => (
        <>
            <ExternalsAuth
                page="vscode-signup-page"
                context={context}
                githubLabel="Continue with GitHub"
                gitlabLabel="Continue with GitLab"
                googleLabel="Continue with Google"
                onClick={() => {}}
                telemetryRecorder={telemetryRecorder}
                telemetryService={telemetryService}
            />
        </>
    )

    const renderEmailAuthForm = (): JSX.Element => (
        <>
            <small className="d-block mt-3">
                <Link
                    className="d-flex align-items-center"
                    to={`${location.pathname}?${queryWithUseEmailToggled.toString()}`}
                >
                    <Icon className={styles.backIcon} aria-hidden={true} svgPath={mdiChevronLeft} />
                    Go back
                </Link>
            </small>

            {signUpForm}
        </>
    )

    const renderAuthMethod = (): JSX.Element => (showEmailForm ? renderEmailAuthForm() : renderCodeHostAuth())

    return (
        <div className={styles.page}>
            <header className="position-relative">
                <div className={styles.headerBackground1} />
                <div className={styles.headerBackground2} />
            </header>

            <div className={classNames('d-flex', 'justify-content-center', 'mb-5', styles.leftOrRightContainer)}>
                <div className={styles.leftOrRight}>
                    <BrandLogo isLightTheme={isLightTheme} variant="logo" className={styles.logo} />
                    <H2 className={classNames('d-flex', 'align-items-center', 'mb-3', 'mt-1', styles.pageHeading)}>
                        <div className={classNames(styles.iconCirlce, 'mr-3')}>
                            <VSCodeIcon />
                        </div>{' '}
                        <strong className="mr-1">Unlock the full potential of the Sourcegraph extension</strong>
                    </H2>
                    With a Sourcegraph account, you can:
                    <ul className={styles.featureList}>
                        <li>Search all of your code from your code host, even without downloading it locally</li>
                        <li>Reference and re-use code from all your projects without leaving VS Code</li>
                        <li>Create code monitors to alert you to changes in code</li>
                    </ul>
                    <div className={styles.companiesHeader}>
                        Trusted by developers at the world's most innovative companies:
                    </div>
                    <img
                        src={`${assetsRoot}/img/customer-logos-${isLightTheme ? 'light' : 'dark'}.svg`}
                        alt="Cloudflare, Uber, SoFi, Dropbox, Plaid, Toast"
                        className={styles.customerLogos}
                    />
                </div>
                <div className={classNames(styles.leftOrRight, styles.signUpWrapper)}>
                    {' '}
                    <H2>Create a free account</H2>
                    {renderAuthMethod()}
                    <small className="text-muted">
                        By registering, you agree to our{' '}
                        <Link to="https://sourcegraph.com/terms" target="_blank" rel="noopener">
                            Terms of Service
                        </Link>{' '}
                        and{' '}
                        <Link to="https://sourcegraph.com/privacy" target="_blank" rel="noopener">
                            Privacy Policy
                        </Link>
                        .
                    </small>
                    <hr className={styles.separator} />
                    <div>
                        Already have an account? <Link to={`/sign-in${location.search}`}>Sign in</Link>
                    </div>
                </div>
            </div>
        </div>
    )
}
