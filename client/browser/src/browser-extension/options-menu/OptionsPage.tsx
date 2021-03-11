import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useInputValidation, deriveInputClassName } from '../../../../shared/src/util/useInputValidation'
import { LoaderInput } from '../../../../branded/src/components/LoaderInput'
import BookOpenPageVariantIcon from 'mdi-react/BookOpenPageVariantIcon'
import classNames from 'classnames'
import { Observable } from 'rxjs'
import { Toggle } from '../../../../branded/src/components/Toggle'
import { SourcegraphLogo } from '../../../../branded/src/components/SourcegraphLogo'
import { OptionsPageAdvancedSettings } from './OptionsPageAdvancedSettings'
import EarthIcon from 'mdi-react/EarthIcon'
import LockIcon from 'mdi-react/LockIcon'
import CheckCircleOutlineIcon from 'mdi-react/CheckCircleOutlineIcon'
import { knownCodeHosts } from '../knownCodeHosts'

export interface OptionsPageProps {
    version: string

    // Sourcegraph URL
    sourcegraphUrl: string
    validateSourcegraphUrl: (url: string) => Observable<string | undefined>
    onChangeSourcegraphUrl: (url: string) => void

    // Option flags
    optionFlags: { key: string; label: string; value: boolean }[]
    onChangeOptionFlag: (key: string, value: boolean) => void

    isActivated: boolean
    onToggleActivated: (value: boolean) => void

    isFullPage: boolean
    showPrivateRepositoryAlert?: boolean
    showSourcegraphCloudAlert?: boolean
    permissionAlert?: { name: string; icon?: React.ComponentType<{ className?: string }> }
    requestPermissionsHandler?: React.MouseEventHandler
    currentHost?: string
}

// "Error code" constants for Sourcegraph URL validation
export const URL_FETCH_ERROR = 'URL_FETCH_ERROR'
export const URL_AUTH_ERROR = 'URL_AUTH_ERROR'

export const OptionsPage: React.FunctionComponent<OptionsPageProps> = ({
    version,
    sourcegraphUrl,
    validateSourcegraphUrl,
    isActivated,
    onToggleActivated,
    isFullPage,
    showPrivateRepositoryAlert,
    showSourcegraphCloudAlert,
    permissionAlert,
    requestPermissionsHandler,
    optionFlags,
    onChangeOptionFlag,
    onChangeSourcegraphUrl,
    currentHost,
}) => {
    const [showAdvancedSettings, setShowAdvancedSettings] = useState(false)
    const urlInputReference = useRef<HTMLInputElement | null>(null)
    const [urlState, nextUrlFieldChange, nextUrlInputElement] = useInputValidation(
        useMemo(
            () => ({
                initialValue: sourcegraphUrl,
                synchronousValidators: [],
                asynchronousValidators: [validateSourcegraphUrl],
            }),
            [sourcegraphUrl, validateSourcegraphUrl]
        )
    )

    const urlInputElements = useCallback(
        (urlInputElement: HTMLInputElement | null) => {
            urlInputReference.current = urlInputElement
            nextUrlInputElement(urlInputElement)
        },
        [nextUrlInputElement]
    )

    const toggleAdvancedSettings = useCallback(
        () => setShowAdvancedSettings(showAdvancedSettings => !showAdvancedSettings),
        []
    )

    const linkProps: Pick<React.AnchorHTMLAttributes<HTMLAnchorElement>, 'rel' | 'target'> = {
        target: '_blank',
        rel: 'noopener noreferrer',
    }

    useEffect(() => {
        if (urlState.kind === 'VALID') {
            onChangeSourcegraphUrl(urlState.value)
        }
    }, [onChangeSourcegraphUrl, urlState])

    return (
        <div className={classNames('options-page', isFullPage && 'options-page--full shadow')}>
            <section className="options-page__section">
                <div className="d-flex justify-content-between">
                    <SourcegraphLogo className="options-page__logo" />
                    <div>
                        <Toggle
                            value={isActivated}
                            onToggle={onToggleActivated}
                            title={`Toggle to ${isActivated ? 'disable' : 'enable'} extension`}
                            aria-label="Toggle browser extension"
                        />
                    </div>
                </div>
                <div className="options-page__version">v{version}</div>
            </section>
            <CodeHostsSection currentHost={currentHost} />
            <section className="options-page__section border-0">
                {/* eslint-disable-next-line react/forbid-elements */}
                <form onSubmit={preventDefault} noValidate={true}>
                    <label htmlFor="sourcegraph-url">Sourcegraph URL</label>
                    <LoaderInput
                        loading={urlState.kind === 'LOADING'}
                        className={classNames(deriveInputClassName(urlState))}
                    >
                        <input
                            className={classNames('form-control', deriveInputClassName(urlState))}
                            id="sourcegraph-url"
                            type="url"
                            pattern="^https://.*"
                            value={urlState.value}
                            onChange={nextUrlFieldChange}
                            ref={urlInputElements}
                            spellCheck={false}
                            required={true}
                        />
                    </LoaderInput>
                    {urlState.kind === 'LOADING' ? (
                        <small className="text-muted d-block mt-1">Checking...</small>
                    ) : urlState.kind === 'INVALID' ? (
                        <small className="invalid-feedback">
                            {urlState.reason === URL_FETCH_ERROR ? (
                                'Incorrect Sourcegraph instance address'
                            ) : urlState.reason === URL_AUTH_ERROR ? (
                                <>
                                    Authentication to Sourcegraph failed.{' '}
                                    <a href={urlState.value} {...linkProps}>
                                        Sign in to your instance
                                    </a>{' '}
                                    to continue
                                </>
                            ) : urlInputReference.current?.validity.typeMismatch ? (
                                'Please enter a valid URL, including the protocol prefix (e.g. https://sourcegraph.example.com).'
                            ) : urlInputReference.current?.validity.patternMismatch ? (
                                'The browser extension can only work over HTTPS in modern browsers.'
                            ) : (
                                urlState.reason
                            )}
                        </small>
                    ) : (
                        <small className="valid-feedback">Looks good!</small>
                    )}
                </form>
                <p className="mt-3 mb-1">
                    <small>Enter the URL of your Sourcegraph instance to use the extension on private code.</small>
                </p>

                <a href="https://docs.sourcegraph.com/integration/browser_extension#privacy" {...linkProps}>
                    <small>How do we keep your code private?</small>
                </a>
            </section>

            {permissionAlert && (
                <PermissionAlert {...permissionAlert} onClickGrantPermissions={requestPermissionsHandler} />
            )}

            {showSourcegraphCloudAlert && <SourcegraphCloudAlert />}

            {showPrivateRepositoryAlert && <PrivateRepositoryAlert />}
            <section className="options-page__section">
                <p className="mb-0">
                    <button type="button" className="btn btn-link btn-sm p-0" onClick={toggleAdvancedSettings}>
                        <small>{showAdvancedSettings ? 'Hide' : 'Show'} advanced settings</small>
                    </button>
                </p>
                {showAdvancedSettings && (
                    <OptionsPageAdvancedSettings optionFlags={optionFlags} onChangeOptionFlag={onChangeOptionFlag} />
                )}
            </section>
            <section className="d-flex">
                <div className="options-page__split-section-part">
                    <a href="https://sourcegraph.com/search" {...linkProps}>
                        <EarthIcon className="icon-inline mr-2" />
                        Sourcegraph Cloud
                    </a>
                </div>
                <div className="options-page__split-section-part">
                    <a href="https://docs.sourcegraph.com" {...linkProps}>
                        <BookOpenPageVariantIcon className="icon-inline mr-2" />
                        Documentation
                    </a>
                </div>
            </section>
        </div>
    )
}

interface PermissionAlertProps {
    icon?: React.ComponentType<{ className?: string }>
    name: string
    onClickGrantPermissions?: React.MouseEventHandler
}

const PermissionAlert: React.FunctionComponent<PermissionAlertProps> = ({
    name,
    icon: Icon,
    onClickGrantPermissions,
}) => (
    <section className="options-page__section bg-2">
        <h4>
            {Icon && <Icon className="icon-inline mr-2" />} <span>{name}</span>
        </h4>
        <p className="options-page__permission-text">
            <strong>Grant permissions</strong> to use the Sourcegraph extension on {name}.
        </p>
        <button type="button" onClick={onClickGrantPermissions} className="btn btn-sm btn-primary">
            <small>Grant permissions</small>
        </button>
    </section>
)

const PrivateRepositoryAlert: React.FunctionComponent = () => (
    <section className="options-page__section bg-2">
        <h4>
            <LockIcon className="icon-inline mr-2" />
            Private repository
        </h4>
        <p>
            To use the browser extension with your private repositories, you need to set up a{' '}
            <strong>private Sourcegraph instance</strong> and connect the browser extension to it.
        </p>
        <ol>
            <li className="mb-2">
                <a href="https://docs.sourcegraph.com/" rel="noopener" target="_blank">
                    Install and configure Sourcegraph
                </a>
                . Skip this step if you already have a private Sourcegraph instance.
            </li>
            <li className="mb-2">Click the Sourcegraph icon in the browser toolbar to bring up this popup again.</li>
            <li className="mb-2">
                Enter the URL (including the protocol) of your Sourcegraph instance above, e.g.{' '}
                <q>https://sourcegraph.example.com</q>.
            </li>
            <li>
                Make sure that the status says <q>Looks good!</q>.
            </li>
        </ol>
    </section>
)

const CodeHostsSection: React.FunctionComponent<{ currentHost?: string }> = ({ currentHost }) => (
    <section className="options-page__section">
        <p>Get code intelligence tooltips while browsing files and reading PRs on your code host.</p>
        <div>
            {knownCodeHosts.map(({ host, icon: Icon }) => (
                <span
                    key={host}
                    className={classNames('code-hosts-section__icon', {
                        // Use `endsWith` in order to match subdomains.
                        'bg-3': currentHost?.endsWith(host),
                    })}
                >
                    {Icon && <Icon />}
                </span>
            ))}
        </div>
    </section>
)

const SourcegraphCloudAlert: React.FunctionComponent = () => (
    <section className="options-page__section bg-2">
        <h4>
            <CheckCircleOutlineIcon className="icon-inline mr-2" />
            You're on Sourcegraph Cloud
        </h4>
        <p>Naturally, the browser extension is not necessary to browse public code on sourcegraph.com.</p>
    </section>
)

function preventDefault(event: React.FormEvent<HTMLFormElement>): void {
    event.preventDefault()
}
