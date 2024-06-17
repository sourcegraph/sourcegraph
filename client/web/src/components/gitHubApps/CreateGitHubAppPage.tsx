import React, { type FC, useState, useCallback, useRef, useEffect } from 'react'

import classNames from 'classnames'
import { noop } from 'lodash'
import { useNavigate } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import { Alert, Container, Button, Input, Label, Text, PageHeader, Checkbox, Link } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import type { GitHubAppDomain, GitHubAppKind } from '../../graphql-operations'
import { PageTitle } from '../PageTitle'

interface StateResponse {
    state?: string
    webhookUUID?: string
    appID?: string
}

interface FormOptions {
    state: string
    name: string
    webhookURL?: string
}

export interface CreateGitHubAppPageProps extends TelemetryV2Props {
    /** The events that the new GitHub App should subscribe to by default. */
    defaultEvents: string[]
    /** The permissions that the new GitHub App will request by default. */
    defaultPermissions: Record<string, string>
    /**
     * The title to show at the top of the page, defaults to "Create GitHub App".
     */
    pageTitle?: string
    /**
     * The main description to show at the top of the page underneath the header. If
     * omitted, a generic introduction to GitHub Apps with a link to the docs will be
     * shown.
     */
    headerDescription?: React.ReactNode
    /** An optional annotation to show in the page header. */
    headerAnnotation?: React.ReactNode
    /** The domain the new GitHub App is meant to be used for in Sourcegraph. */
    appDomain: GitHubAppDomain
    /** The purpose of the GitHub App to be created. This is only applicable when the appDomain is BATCHES. */
    appKind: GitHubAppKind
    /** The name to use for the new GitHub App. Defaults to "Sourcegraph". */
    defaultAppName?: string
    /*
     * If omitted, the user will be asked to specify a URL from the form. If provided, it
     * will be left off the form.
     */
    baseURL?: string
    /**
     * A customer validation function for the URL input. Returns true if the URL is valid,
     * or a string with an error message reason if not.
     */
    validateURL?: (url: string) => true | string
    /** The currently authenticated user */
    authenticatedUser: AuthenticatedUser
    /**
     * Whether or not the page is being rendered in a minimized mode.
     * Minimized mode is when this component is rendered in a modal.
     */
    minimizedMode?: boolean
}

/**
 * Page for creating and connecting a new GitHub App.
 */
export const CreateGitHubAppPage: FC<CreateGitHubAppPageProps> = ({
    defaultEvents,
    defaultPermissions,
    pageTitle = 'Create GitHub App',
    headerDescription,
    headerAnnotation,
    appDomain,
    defaultAppName = 'Sourcegraph',
    baseURL,
    validateURL,
    telemetryRecorder,
    appKind,
    authenticatedUser,
    minimizedMode,
}) => {
    const navigate = useNavigate()
    const ref = useRef<HTMLFormElement>(null)
    const formInput = useRef<HTMLInputElement>(null)
    const [name, setName] = useState<string>(defaultAppName)
    const [nameError, setNameError] = useState<string>()
    const [url, setUrl] = useState<string>(baseURL || 'https://github.com')
    const [urlError, setUrlError] = useState<string>()
    const [org, setOrg] = useState<string>('')
    const [isPublic, setIsPublic] = useState<boolean>(false)
    const [error, setError] = useState<string>()

    useEffect(() => {
        EVENT_LOGGER.logPageView('SiteAdminCreateGiHubApp')
        telemetryRecorder.recordEvent('admin.GitHubApp.create', 'view')
    }, [telemetryRecorder])

    const originURL = window.location.origin
    const getManifest = useCallback(
        (name: string, webhookURL?: string): string =>
            JSON.stringify({
                name: name.trim(),
                url: originURL,
                hook_attributes: webhookURL ? { url: webhookURL } : undefined,
                redirect_url: new URL('/githubapp/redirect', originURL).href,
                setup_url: new URL('/githubapp/setup', originURL).href,
                callback_urls: [new URL('/.auth/github/callback', originURL).href],
                setup_on_update: true,
                public: isPublic,
                default_permissions: defaultPermissions,
                default_events: defaultEvents,
            }),
        [originURL, defaultEvents, defaultPermissions, isPublic]
    )

    const createActionUrl = useCallback(
        (state: string): string => {
            let prefix = 'settings/apps/new'
            const trimmedOrg = org.trim()
            if (trimmedOrg !== '') {
                prefix = `organizations/${encodeURIComponent(trimmedOrg)}/settings/apps/new`
            }

            const trimmedUrl = url.trim()
            const originURL = trimmedUrl.endsWith('/') ? trimmedUrl : `${trimmedUrl}/`
            return new URL(`${prefix}?state=${state}`, originURL).href
        },
        [org, url]
    )

    const submitForm = useCallback(
        ({ state, webhookURL, name }: FormOptions) => {
            if (state && ref.current && formInput.current) {
                const actionUrl = createActionUrl(state)
                ref.current.action = actionUrl
                formInput.current.value = getManifest(name, webhookURL)
                ref.current.submit()
            }
        },
        [createActionUrl, getManifest]
    )

    const createState = useCallback(async () => {
        setError(undefined)
        try {
            // We encode the name and url here so that special characters like `#` are interpreted as
            // part of the URL and not the fragment.
            const response = await fetch(
                `/githubapp/new-app-state?appName=${encodeURIComponent(
                    name
                )}&webhookURN=${url}&domain=${appDomain}&baseURL=${encodeURIComponent(url)}&kind=${appKind}&userID=${
                    authenticatedUser.id
                }`
            )
            if (!response.ok) {
                if (response.body instanceof ReadableStream) {
                    const error = await response.text()
                    throw new Error(error)
                }
            }
            const jsonResponse = (await response.json()) as StateResponse
            let webhookURL: string | undefined
            if (jsonResponse.webhookUUID?.length) {
                webhookURL = new URL(`/.api/webhooks/${jsonResponse.webhookUUID}`, originURL).href
            }
            if (!jsonResponse.state?.length) {
                throw new Error('Response from server missing state parameter')
            }
            submitForm({ state: jsonResponse.state, webhookURL, name })
        } catch (error_) {
            if (error_ instanceof Error) {
                setError(error_.message)
            } else {
                // eslint-disable-next-line no-console
                console.error(error_)
                setError('Unknown error occurred.')
            }
        }
    }, [submitForm, name, appDomain, url, originURL, appKind, authenticatedUser.id])

    const handleNameChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
        setName(event.target.value)
        const trimmedValue = event.target.value.trim()
        if (trimmedValue.length < 3) {
            setNameError('Name must be 3 characters or more.')
        } else if (trimmedValue.length > 34) {
            // A requirement from GitHub's side
            setNameError('Name must be 34 characters or less.')
        } else {
            setNameError(undefined)
        }
    }, [])

    const handleUrlChange = useCallback(
        (event: React.ChangeEvent<HTMLInputElement>) => {
            const trimmedURL = event.target.value.trim()
            setUrl(trimmedURL)
            try {
                new URL(trimmedURL)
            } catch {
                return setUrlError('URL is not valid.')
            }
            if (validateURL) {
                const error = validateURL(event.target.value)
                if (error !== true) {
                    return setUrlError(error)
                }
            }
            setUrlError(undefined)
        },
        [validateURL]
    )

    const handleOrgChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => setOrg(event.target.value), [])
    const toggleIsPublic = useCallback(() => setIsPublic(isPublic => !isPublic), [])

    return (
        <>
            {!minimizedMode && (
                <>
                    <PageTitle title={pageTitle} />
                    <PageHeader
                        path={[{ text: pageTitle }]}
                        headingElement="h2"
                        description={
                            headerDescription || (
                                <>
                                    Register a GitHub App to better manage GitHub code host connections.{' '}
                                    <Link to="/help/admin/external_service/github#using-a-github-app" target="_blank">
                                        See how GitHub App configuration works.
                                    </Link>
                                </>
                            )
                        }
                        annotation={headerAnnotation}
                        className="mb-3"
                    />
                </>
            )}

            <Container className="mb-3">
                {error && <Alert variant="danger">Error creating GitHub App: {error}</Alert>}
                <Text>
                    Provide the details for a new GitHub App with the form below. Once you click "Create GitHub App",
                    you will be routed to <strong>{baseURL || 'GitHub'}</strong> to create the App and choose which
                    repositories to grant it access to. Once created on <strong>{baseURL || 'GitHub'}</strong>, you'll
                    be redirected back here to finish connecting it to Sourcegraph.
                </Text>
                <Label className="w-100">
                    <Text alignment="left" className="mb-2">
                        GitHub App Name
                    </Text>
                    <Input
                        type="text"
                        onChange={handleNameChange}
                        value={name}
                        error={nameError}
                        status={nameError ? 'error' : undefined}
                        placeholder="Sourcegraph"
                        message="The display name of your GitHub App. It must be unique across the GitHub instance."
                    />
                </Label>
                {baseURL ? null : (
                    <Label className="w-100 mt-2">
                        <Text alignment="left" className="mb-2">
                            GitHub URL
                        </Text>
                        <Input
                            type="text"
                            onChange={handleUrlChange}
                            value={url}
                            error={urlError}
                            status={urlError ? 'error' : undefined}
                            placeholder="https://github.com"
                            message="The base URL of the GitHub instance, e.g., https://github.com, https://github.company.com."
                        />
                    </Label>
                )}
                <Label className="w-100 mt-2">
                    <Text alignment="left" className="mb-2">
                        Organization name <span className="text-muted">(optional)</span>
                    </Text>
                    <Input
                        type="text"
                        onChange={handleOrgChange}
                        value={org}
                        message={
                            <>
                                By default, the GitHub App will be registered on your personal account. To register the
                                App on a GitHub organization instead, specify the organization name. Only{' '}
                                <Link
                                    to="https://docs.github.com/en/organizations/managing-peoples-access-to-your-organization-with-roles/roles-in-an-organization#organization-owners"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    organization owners
                                </Link>{' '}
                                can register GitHub Apps.
                            </>
                        }
                    />
                </Label>
                <Checkbox
                    wrapperClassName="mt-2"
                    id="app-is-public"
                    onChange={toggleIsPublic}
                    checked={isPublic}
                    label={
                        <>
                            Make App public <span className="text-muted">(optional)</span>
                        </>
                    }
                    message={
                        <>
                            Your GitHub App must be public if you want to install it on multiple organizations or user
                            accounts.{' '}
                            <Link
                                to="/help/admin/external_service/github#multiple-installations"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Learn more about public vs. private GitHub Apps.
                            </Link>
                        </>
                    }
                />
                {/* eslint-disable-next-line react/forbid-elements */}
                <form ref={ref} method="post">
                    {/* eslint-disable-next-line react/forbid-elements */}
                    <input ref={formInput} name="manifest" onChange={noop} hidden={true} />
                </form>
            </Container>
            <div
                className={classNames({
                    'd-flex flex-row-reverse': minimizedMode,
                })}
            >
                <Button variant="primary" onClick={createState} disabled={!!nameError || !!urlError}>
                    Create Github App
                </Button>
                <Button
                    className={classNames({
                        'ml-2': !minimizedMode,
                        'mr-2': minimizedMode,
                    })}
                    onClick={() => navigate(-1)}
                    variant="secondary"
                >
                    Cancel
                </Button>
            </div>
        </>
    )
}
