import React from 'react'

import classNames from 'classnames'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Link } from '@sourcegraph/wildcard'

import type { AuthProvider, SourcegraphContext } from '../../jscontext'
import { EventName, V2AuthProviderTypes } from '../../util/constants'

import styles from './ExternalsAuth.module.scss'

export type AuthPages =
    | 'vscode-signup-page'
    | 'cloud-signup-page'
    | 'cody-marketing-page'
    | 'get-cody-page'
    | 'try-cody-widget-blob'
    | 'try-cody-widget-repo'
const v2Pages: { [p in AuthPages]: number } = {
    'vscode-signup-page': 0,
    'cloud-signup-page': 1,
    'cody-marketing-page': 2,
    'get-cody-page': 3,
    'try-cody-widget-blob': 4,
    'try-cody-widget-repo': 5,
}

interface ExternalsAuthProps extends TelemetryProps, TelemetryV2Props {
    // page is the page on which external auth was initiated; used as metadata for telemetry.
    page: AuthPages
    context: Pick<SourcegraphContext, 'externalURL'>
    githubLabel: string
    gitlabLabel: string
    googleLabel: string
    onClick: (type: AuthProvider['serviceType']) => void
    withCenteredText?: boolean
    ctaClassName?: string
    iconClassName?: string
    redirect?: string
}

const GithubIcon: React.FunctionComponent<React.PropsWithChildren<{ className?: string }>> = ({ className }) => (
    <svg
        className={className}
        xmlns="http://www.w3.org/2000/svg"
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
    >
        <g clipPath="url(#clip0_1098_3272)">
            <path
                fillRule="evenodd"
                clipRule="evenodd"
                d="M8.00016 0.333496C6.10098 0.334481 4.26407 1.00713 2.81789 2.23118C1.37172 3.45522 0.410582 5.15084 0.106343 7.01483C-0.197896 8.87882 0.174597 10.7896 1.15723 12.4056C2.13986 14.0216 3.66854 15.2373 5.46993 15.8355C5.86735 15.9092 6.01704 15.6629 6.01704 15.4535C6.01704 15.2441 6.00909 14.6368 6.00644 13.973C3.78091 14.4537 3.31064 13.0338 3.31064 13.0338C2.94766 12.1118 2.42307 11.8694 2.42307 11.8694C1.69712 11.3768 2.47739 11.386 2.47739 11.386C3.2815 11.4427 3.70408 12.2066 3.70408 12.2066C4.41678 13.4224 5.57591 13.0707 6.03161 12.8652C6.10315 12.3502 6.31113 11.9998 6.54031 11.8009C4.76253 11.6007 2.89467 10.9184 2.89467 7.87044C2.88365 7.07997 3.17866 6.31553 3.71865 5.73528C3.63652 5.53507 3.3623 4.72632 3.79681 3.62778C3.79681 3.62778 4.46845 3.4144 5.99718 4.44312C7.30842 4.08658 8.69189 4.08658 10.0031 4.44312C11.5305 3.4144 12.2008 3.62778 12.2008 3.62778C12.6367 4.72368 12.3625 5.53244 12.2803 5.73528C12.822 6.31562 13.1177 7.0814 13.1056 7.87308C13.1056 10.9276 11.2338 11.6007 9.45337 11.797C9.73951 12.0446 9.99519 12.528 9.99519 13.2709C9.99519 14.3352 9.98591 15.1914 9.98591 15.4535C9.98591 15.6656 10.1303 15.9132 10.5357 15.8355C12.3373 15.2373 13.8661 14.0213 14.8487 12.4051C15.8313 10.7888 16.2036 8.8777 15.899 7.01353C15.5945 5.14936 14.6329 3.45373 13.1862 2.2299C11.7395 1.00607 9.90221 0.333857 8.0028 0.333496H8.00016Z"
                fill="#0F111A"
            />
            <path
                d="M3.0296 11.7544C3.01238 11.7939 2.9488 11.8058 2.89713 11.7781C2.84547 11.7505 2.80705 11.6991 2.8256 11.6583C2.84415 11.6174 2.90641 11.6069 2.95807 11.6346C3.00974 11.6622 3.04947 11.7149 3.0296 11.7544Z"
                fill="#0F111A"
            />
            <path
                d="M3.35417 12.1144C3.32674 12.1282 3.29535 12.132 3.26539 12.1253C3.23542 12.1185 3.20874 12.1017 3.18991 12.0775C3.13824 12.0222 3.12764 11.9458 3.16738 11.9116C3.20712 11.8773 3.27866 11.8931 3.33033 11.9485C3.38199 12.0038 3.39391 12.0802 3.35417 12.1144Z"
                fill="#0F111A"
            />
            <path
                d="M3.66952 12.5714C3.6205 12.6056 3.53704 12.5714 3.49067 12.5029C3.47785 12.4906 3.46766 12.4758 3.46069 12.4595C3.45373 12.4433 3.45013 12.4257 3.45013 12.408C3.45013 12.3903 3.45373 12.3728 3.46069 12.3565C3.46766 12.3402 3.47785 12.3255 3.49067 12.3132C3.53969 12.2803 3.62315 12.3132 3.66952 12.3804C3.71588 12.4475 3.71721 12.5371 3.66952 12.5714Z"
                fill="#0F111A"
            />
            <path
                d="M4.09737 13.014C4.05365 13.0627 3.9649 13.0496 3.89204 12.9837C3.81918 12.9178 3.80195 12.8283 3.84567 12.7809C3.88939 12.7334 3.97814 12.7466 4.05365 12.8111C4.12916 12.8757 4.14374 12.9666 4.09737 13.014Z"
                fill="#0F111A"
            />
            <path
                d="M4.69747 13.2723C4.6776 13.3342 4.58752 13.3618 4.49744 13.3355C4.40735 13.3091 4.34774 13.2354 4.36496 13.1722C4.38218 13.1089 4.47359 13.0799 4.565 13.1089C4.6564 13.1379 4.71469 13.2077 4.69747 13.2723Z"
                fill="#0F111A"
            />
            <path
                d="M5.35189 13.3166C5.35189 13.3812 5.27771 13.4365 5.18233 13.4378C5.08695 13.4391 5.00879 13.3864 5.00879 13.3219C5.00879 13.2573 5.08297 13.202 5.17835 13.2007C5.27373 13.1994 5.35189 13.2508 5.35189 13.3166Z"
                fill="#0F111A"
            />
            <path
                d="M5.96124 13.2154C5.97317 13.28 5.90693 13.3472 5.81155 13.363C5.71617 13.3788 5.63271 13.3406 5.62079 13.2774C5.60887 13.2141 5.67775 13.1456 5.77048 13.1285C5.86321 13.1114 5.94932 13.1509 5.96124 13.2154Z"
                fill="#0F111A"
            />
        </g>
        <defs>
            <clipPath id="clip0_1098_3272">
                <rect width="16" height="16" fill="white" />
            </clipPath>
        </defs>
    </svg>
)

const GitlabColorIcon: React.FunctionComponent<React.PropsWithChildren<{ className?: string }>> = ({ className }) => (
    <svg
        className={className}
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
    >
        <path
            d="M15.7331 5.99981L15.7116 5.94322L13.534 0.355057C13.4899 0.245331 13.4115 0.152251 13.31 0.08929C13.2344 0.0416309 13.1484 0.0120763 13.059 0.00299452C12.9696 -0.00608725 12.8793 0.00555682 12.7954 0.0369932C12.7114 0.0684297 12.636 0.118784 12.5754 0.184021C12.5148 0.249257 12.4705 0.327561 12.4462 0.412656L10.9763 4.83973H5.02382L3.55393 0.412656C3.52941 0.327688 3.48505 0.249532 3.42439 0.18442C3.36372 0.119308 3.28843 0.0690431 3.20452 0.0376324C3.12061 0.0062216 3.0304 -0.00546551 2.94108 0.00350336C2.85176 0.0124722 2.76581 0.0418485 2.69008 0.08929C2.58866 0.152251 2.51023 0.245331 2.46616 0.355057L0.289574 5.94423L0.266976 5.99981C-0.0463884 6.8055 -0.0849937 7.68958 0.15698 8.51874C0.398953 9.3479 0.908387 10.0772 1.60847 10.5966L1.61668 10.6027L1.63517 10.6169L4.94781 13.0593L6.59129 14.281L7.5897 15.0237C7.70685 15.1108 7.84965 15.1579 7.99646 15.1579C8.14327 15.1579 8.28608 15.1108 8.40322 15.0237L9.40164 14.281L11.0451 13.0593L14.3814 10.6027L14.3906 10.5956C15.0907 10.0764 15.6002 9.34732 15.8423 8.51836C16.0845 7.68941 16.0461 6.80547 15.7331 5.99981Z"
            fill="#E24329"
        />
        <path
            d="M15.7331 5.99995L15.7115 5.94336C14.6505 6.15755 13.6508 6.59994 12.7841 7.23884L8.00256 10.7959L11.0471 13.0594L14.3834 10.6029L14.3926 10.5958C15.0923 10.0763 15.6014 9.34708 15.8432 8.51814C16.0849 7.6892 16.0463 6.80541 15.7331 5.99995Z"
            fill="#FC6D26"
        />
        <path
            d="M4.94788 13.0595L6.59136 14.2812L7.58977 15.0239C7.70691 15.111 7.84972 15.1581 7.99653 15.1581C8.14334 15.1581 8.28615 15.111 8.40329 15.0239L9.4017 14.2812L11.0452 13.0595L8.00064 10.7959L4.94788 13.0595Z"
            fill="#FCA326"
        />
        <path
            d="M3.21599 7.23881C2.34953 6.60023 1.35017 6.15817 0.289574 5.94434L0.266976 5.99991C-0.0463884 6.80561 -0.0849937 7.68969 0.15698 8.51885C0.398953 9.34801 0.908387 10.0773 1.60847 10.5968L1.61668 10.6028L1.63517 10.617L4.94781 13.0594L7.99441 10.7958L3.21599 7.23881Z"
            fill="#FC6D26"
        />
    </svg>
)

const GoogleIcon: React.FunctionComponent<React.PropsWithChildren<{ className?: string }>> = ({ className }) => (
    <svg className={className} xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" width="16px" height="16px">
        <path
            fill="#FFC107"
            d="M43.611,20.083H42V20H24v8h11.303c-1.649,4.657-6.08,8-11.303,8c-6.627,0-12-5.373-12-12c0-6.627,5.373-12,12-12c3.059,0,5.842,1.154,7.961,3.039l5.657-5.657C34.046,6.053,29.268,4,24,4C12.955,4,4,12.955,4,24c0,11.045,8.955,20,20,20c11.045,0,20-8.955,20-20C44,22.659,43.862,21.35,43.611,20.083z"
        />
        <path
            fill="#FF3D00"
            d="M6.306,14.691l6.571,4.819C14.655,15.108,18.961,12,24,12c3.059,0,5.842,1.154,7.961,3.039l5.657-5.657C34.046,6.053,29.268,4,24,4C16.318,4,9.656,8.337,6.306,14.691z"
        />
        <path
            fill="#4CAF50"
            d="M24,44c5.166,0,9.86-1.977,13.409-5.192l-6.19-5.238C29.211,35.091,26.715,36,24,36c-5.202,0-9.619-3.317-11.283-7.946l-6.522,5.025C9.505,39.556,16.227,44,24,44z"
        />
        <path
            fill="#1976D2"
            d="M43.611,20.083H42V20H24v8h11.303c-0.792,2.237-2.231,4.166-4.087,5.571c0.001-0.001,0.002-0.001,0.003-0.002l6.19,5.238C36.971,39.205,44,34,44,24C44,22.659,43.862,21.35,43.611,20.083z"
        />
    </svg>
)

/**
 * ExternalsAuth is ONLY intended for use in Sourcegraph.com components.
 */
export const ExternalsAuth: React.FunctionComponent<React.PropsWithChildren<ExternalsAuthProps>> = ({
    page,
    context,
    githubLabel,
    gitlabLabel,
    googleLabel,
    onClick,
    withCenteredText,
    ctaClassName,
    iconClassName,
    redirect,
    telemetryService,
    telemetryRecorder,
}) => {
    // Since this component is only intended for use on Sourcegraph.com, it's OK to hardcode
    // our providers.
    const redirectQuery = redirect ? `&redirect=${redirect}` : ''
    // Use absolute URL to force full-page reload (because the auth routes are
    // handled by the backend router, not the frontend router).
    const gitHubLoginUrl = `${context.externalURL}/.auth/openidconnect/login?prompt_auth=github&pc=sams${redirectQuery}`
    const gitLabLoginUrl = `${context.externalURL}/.auth/openidconnect/login?prompt_auth=gitlab&pc=sams${redirectQuery}`
    const googleLoginUrl = `${context.externalURL}/.auth/openidconnect/login?prompt_auth=google&pc=sams${redirectQuery}`

    const logAuthAndCallback = (
        type: AuthProvider['serviceType'],
        callback: (type: AuthProvider['serviceType']) => void
    ): void => {
        telemetryService.log(EventName.AUTH_INITIATED, { type, page }, { type, page })
        telemetryRecorder.recordEvent('auth', 'initiate', {
            metadata: { type: V2AuthProviderTypes[type], page: v2Pages[page] },
        })
        callback(type)
    }

    return (
        <>
            <Link
                to={gitHubLoginUrl}
                className={classNames(
                    'text-decoration-none',
                    withCenteredText && 'd-flex justify-content-center',
                    styles.signUpButton,
                    styles.githubButton,
                    ctaClassName
                )}
                onClick={() => logAuthAndCallback('github', onClick)}
            >
                <GithubIcon className={classNames('mr-2', iconClassName)} />
                {githubLabel}
            </Link>

            <Link
                to={gitLabLoginUrl}
                className={classNames(
                    'text-decoration-none',
                    withCenteredText && 'd-flex justify-content-center',
                    styles.signUpButton,
                    styles.gitlabButton,
                    ctaClassName
                )}
                onClick={() => logAuthAndCallback('gitlab', onClick)}
            >
                <GitlabColorIcon className={classNames('mr-2', iconClassName)} /> {gitlabLabel}
            </Link>

            <Link
                to={googleLoginUrl}
                className={classNames(
                    'text-decoration-none',
                    withCenteredText && 'd-flex justify-content-center',
                    styles.signUpButton,
                    styles.googleButton,
                    ctaClassName
                )}
                onClick={() => logAuthAndCallback('openidconnect', onClick)}
            >
                <GoogleIcon className={classNames('mr-2', iconClassName)} /> {googleLabel}
            </Link>
        </>
    )
}
