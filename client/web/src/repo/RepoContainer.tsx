import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import React, { useMemo, useState, useEffect, useCallback, useRef } from 'react'
import { escapeRegExp } from 'lodash'
import { Route, RouteComponentProps, Switch } from 'react-router'
import { NEVER, ObservableInput, of } from 'rxjs'
import { catchError } from 'rxjs/operators'
import { redirectToExternalHost } from '.'
import {
    isRepoNotFoundErrorLike,
    isRepoSeeOtherErrorLike,
    isCloneInProgressErrorLike,
} from '../../../shared/src/backend/errors'
import { ActivationProps } from '../../../shared/src/components/activation/Activation'
import { ExtensionsControllerProps } from '../../../shared/src/extensions/controller'
import { PlatformContextProps } from '../../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../../shared/src/settings/settings'
import { ErrorLike, isErrorLike, asError } from '../../../shared/src/util/errors'
import { encodeURIPathComponent, makeRepoURI } from '../../../shared/src/util/url'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { HeroPage } from '../components/HeroPage'
import {
    searchQueryForRepoRevision,
    PatternTypeProps,
    CaseSensitivityProps,
    CopyQueryButtonProps,
    SearchContextProps,
} from '../search'
import { RouteDescriptor } from '../util/contributions'
import { parseBrowserRepoURL } from '../util/url'
import { GoToCodeHostAction } from './actions/GoToCodeHostAction'
import { fetchFileExternalLinks, fetchRepository, resolveRevision } from './backend'
import { RepoHeader, RepoHeaderActionButton, RepoHeaderContributionsLifecycleProps } from './RepoHeader'
import { RepoRevisionContainer, RepoRevisionContainerRoute } from './RepoRevisionContainer'
import { RepositoryNotFoundPage } from './RepositoryNotFoundPage'
import { ThemeProps } from '../../../shared/src/theme'
import { RepoSettingsAreaRoute } from './settings/RepoSettingsArea'
import { RepoSettingsSideBarGroup } from './settings/RepoSettingsSidebar'
import { ErrorMessage } from '../components/alerts'
import { QueryState } from '../search/helpers'
import * as H from 'history'
import { VersionContextProps } from '../../../shared/src/search/util'
import { BreadcrumbSetters, BreadcrumbsProps } from '../components/Breadcrumbs'
import { useObservable } from '../../../shared/src/util/useObservable'
import { repeatUntil } from '../../../shared/src/util/rxjs/repeatUntil'
import { RepoHeaderContributionPortal } from './RepoHeaderContributionPortal'
import { Link } from '../../../shared/src/components/Link'
import { UncontrolledPopover } from 'reactstrap'
import MenuDownIcon from 'mdi-react/MenuDownIcon'
import { RepositoriesPopover } from './RepositoriesPopover'
import { displayRepoName } from '../../../shared/src/components/RepoFileLink'
import { AuthenticatedUser } from '../auth'
import { TelemetryProps } from '../../../shared/src/telemetry/telemetryService'
import { ExternalLinkFields, RepositoryFields } from '../graphql-operations'
import { browserExtensionInstalled } from '../tracking/analyticsUtils'
import { InstallBrowserExtensionAlert } from './actions/InstallBrowserExtensionAlert'
import { IS_CHROME } from '../marketing/util'
import { useLocalStorage } from '../util/useLocalStorage'
import { Settings } from '../schema/settings.schema'
import SourceRepositoryIcon from 'mdi-react/SourceRepositoryIcon'
import { escapeSpaces } from '../../../shared/src/search/query/filters'

/**
 * Props passed to sub-routes of {@link RepoContainer}.
 */
export interface RepoContainerContext
    extends RepoHeaderContributionsLifecycleProps,
        SettingsCascadeProps,
        ExtensionsControllerProps,
        PlatformContextProps,
        ThemeProps,
        HoverThresholdProps,
        TelemetryProps,
        ActivationProps,
        PatternTypeProps,
        CaseSensitivityProps,
        CopyQueryButtonProps,
        VersionContextProps,
        Pick<SearchContextProps, 'selectedSearchContextSpec'>,
        BreadcrumbSetters {
    repo: RepositoryFields
    authenticatedUser: AuthenticatedUser | null
    repoSettingsAreaRoutes: readonly RepoSettingsAreaRoute[]
    repoSettingsSidebarGroups: readonly RepoSettingsSideBarGroup[]

    /** The URL route match for {@link RepoContainer}. */
    routePrefix: string

    onDidUpdateExternalLinks: (externalLinks: ExternalLinkFields[] | undefined) => void

    globbing: boolean
}

/** A sub-route of {@link RepoContainer}. */
export interface RepoContainerRoute extends RouteDescriptor<RepoContainerContext> {}

const RepoPageNotFound: React.FunctionComponent = () => (
    <HeroPage icon={MapSearchIcon} title="404: Not Found" subtitle="The repository page was not found." />
)

interface RepoContainerProps
    extends RouteComponentProps<{ repoRevAndRest: string }>,
        SettingsCascadeProps<Settings>,
        PlatformContextProps,
        TelemetryProps,
        ExtensionsControllerProps,
        ActivationProps,
        ThemeProps,
        ExtensionAlertProps,
        PatternTypeProps,
        CaseSensitivityProps,
        CopyQueryButtonProps,
        VersionContextProps,
        Pick<SearchContextProps, 'selectedSearchContextSpec'>,
        BreadcrumbSetters,
        BreadcrumbsProps {
    repoContainerRoutes: readonly RepoContainerRoute[]
    repoRevisionContainerRoutes: readonly RepoRevisionContainerRoute[]
    repoHeaderActionButtons: readonly RepoHeaderActionButton[]
    repoSettingsAreaRoutes: readonly RepoSettingsAreaRoute[]
    repoSettingsSidebarGroups: readonly RepoSettingsSideBarGroup[]
    authenticatedUser: AuthenticatedUser | null
    onNavbarQueryChange: (state: QueryState) => void
    history: H.History
    globbing: boolean
}

export const HOVER_COUNT_KEY = 'hover-count'
const HAS_DISMISSED_ALERT_KEY = 'has-dismissed-extension-alert'

export const HOVER_THRESHOLD = 5

export interface HoverThresholdProps {
    /**
     * Called when a hover with content is shown.
     */
    onHoverShown?: () => void
}

export interface ExtensionAlertProps {
    onExtensionAlertDismissed: () => void
}

/**
 * Renders a horizontal bar and content for a repository page.
 */
export const RepoContainer: React.FunctionComponent<RepoContainerProps> = props => {
    const { repoName, revision, rawRevision, filePath, commitRange, position, range } = parseBrowserRepoURL(
        location.pathname + location.search + location.hash
    )

    // Fetch repository upon mounting the component.
    const repoOrError = useObservable(
        useMemo(
            () =>
                fetchRepository({ repoName }).pipe(
                    catchError(
                        (error): ObservableInput<ErrorLike> => {
                            const redirect = isRepoSeeOtherErrorLike(error)
                            if (redirect) {
                                redirectToExternalHost(redirect)
                                return NEVER
                            }
                            return of(asError(error))
                        }
                    )
                ),
            [repoName]
        )
    )

    const resolvedRevisionOrError = useObservable(
        React.useMemo(
            () =>
                resolveRevision({ repoName, revision }).pipe(
                    catchError(error => {
                        if (isCloneInProgressErrorLike(error)) {
                            return of<ErrorLike>(asError(error))
                        }
                        throw error
                    }),
                    repeatUntil(value => !isCloneInProgressErrorLike(value), { delay: 1000 }),
                    catchError(error => of<ErrorLike>(asError(error)))
                ),
            [repoName, revision]
        )
    )

    // The external links to show in the repository header, if any.
    const [externalLinks, setExternalLinks] = useState<ExternalLinkFields[] | undefined>()

    // The lifecycle props for repo header contributions.
    const [
        repoHeaderContributionsLifecycleProps,
        setRepoHeaderContributionsLifecycleProps,
    ] = useState<RepoHeaderContributionsLifecycleProps>()

    const childBreadcrumbSetters = props.useBreadcrumb(
        useMemo(() => {
            if (isErrorLike(repoOrError) || !repoOrError) {
                return
            }

            return {
                key: 'repository',
                element: (
                    <>
                        <Link
                            to={
                                resolvedRevisionOrError && !isErrorLike(resolvedRevisionOrError)
                                    ? resolvedRevisionOrError.rootTreeURL
                                    : repoOrError.url
                            }
                            className="font-weight-bold text-nowrap test-repo-header-repo-link"
                        >
                            <SourceRepositoryIcon className="icon-inline" /> {displayRepoName(repoOrError.name)}
                        </Link>
                        <button
                            type="button"
                            id="repo-popover"
                            className="btn btn-icon px-0"
                            aria-label="Change repository"
                        >
                            <MenuDownIcon className="icon-inline" />
                        </button>
                        <UncontrolledPopover
                            placement="bottom-start"
                            target="repo-popover"
                            trigger="legacy"
                            hideArrow={true}
                            popperClassName="border-0"
                        >
                            <RepositoriesPopover
                                currentRepo={repoOrError.id}
                                history={props.history}
                                location={props.location}
                            />
                        </UncontrolledPopover>
                    </>
                ),
            }
        }, [repoOrError, resolvedRevisionOrError, props.history, props.location])
    )

    // Update the workspace roots service to reflect the current repo / resolved revision
    useEffect(() => {
        props.extensionsController.services.workspace.roots.next(
            resolvedRevisionOrError && !isErrorLike(resolvedRevisionOrError)
                ? [
                      {
                          uri: makeRepoURI({
                              repoName,
                              revision: resolvedRevisionOrError.commitID,
                          }),
                          inputRevision: revision || '',
                      },
                  ]
                : []
        )
        // Clear the Sourcegraph extensions model's roots when navigating away.
        return () => props.extensionsController.services.workspace.roots.next([])
    }, [props.extensionsController.services.workspace.roots, repoName, resolvedRevisionOrError, revision])

    // Update the navbar query to reflect the current repo / revision
    const { globbing, onNavbarQueryChange } = props
    useEffect(() => {
        let query = searchQueryForRepoRevision(repoName, globbing, revision)
        if (filePath) {
            query = `${query.trimEnd()} file:${escapeSpaces(globbing ? filePath : '^' + escapeRegExp(filePath))}`
        }
        onNavbarQueryChange({
            query,
        })
    }, [revision, filePath, repoName, onNavbarQueryChange, globbing])

    const isBrowserExtensionInstalled = useObservable(browserExtensionInstalled)
    const codeHostIntegrationMessaging =
        (!isErrorLike(props.settingsCascade.final) &&
            props.settingsCascade.final?.['alerts.codeHostIntegrationMessaging']) ||
        'browser-extension'

    // Browser extension discoverability features (alert, popover for `GoToCodeHostAction)
    const [hasDismissedExtensionAlert, setHasDismissedExtensionAlert] = useLocalStorage(HAS_DISMISSED_ALERT_KEY, false)
    const [hasDismissedPopover, setHasDismissedPopover] = useState(false)
    const [hoverCount, setHoverCount] = useLocalStorage(HOVER_COUNT_KEY, 0)
    const canShowPopover =
        !hasDismissedPopover &&
        isBrowserExtensionInstalled === false &&
        codeHostIntegrationMessaging === 'browser-extension' &&
        hoverCount >= HOVER_THRESHOLD
    const showExtensionAlert = useMemo(
        () => isBrowserExtensionInstalled === false && !hasDismissedExtensionAlert && hoverCount >= HOVER_THRESHOLD,
        // Intentionally use useMemo() here without a dependency on hoverCount to only show the alert on the next reload,
        // to not cause an annoying layout shift from displaying the alert.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [hasDismissedExtensionAlert, isBrowserExtensionInstalled]
    )

    const { onExtensionAlertDismissed } = props

    // Increment hovers that the user has seen. Enable browser extension discoverability
    // features after hover count threshold is reached (e.g. alerts, popovers)
    // Store hover count in ref to avoid circular dependency
    // hoverCount -> onHoverShown -> WebHoverOverlay (onHoverShown in useEffect deps) -> onHoverShown()
    const hoverCountReference = useRef(hoverCount)
    hoverCountReference.current = hoverCount
    const onHoverShown = useCallback(() => {
        const count = hoverCountReference.current + 1
        if (count > HOVER_THRESHOLD) {
            // No need to keep updating localStorage
            return
        }
        setHoverCount(count)
    }, [setHoverCount])

    const onPopoverDismissed = useCallback(() => {
        setHasDismissedPopover(true)
    }, [])

    const onAlertDismissed = useCallback(() => {
        onExtensionAlertDismissed()
        setHasDismissedExtensionAlert(true)
    }, [onExtensionAlertDismissed, setHasDismissedExtensionAlert])

    if (!repoOrError) {
        // Render nothing while loading
        return null
    }

    const viewerCanAdminister = !!props.authenticatedUser && props.authenticatedUser.siteAdmin

    if (isErrorLike(repoOrError)) {
        // Display error page
        if (isRepoNotFoundErrorLike(repoOrError)) {
            return <RepositoryNotFoundPage repo={repoName} viewerCanAdminister={viewerCanAdminister} />
        }
        return <HeroPage icon={AlertCircleIcon} title="Error" subtitle={<ErrorMessage error={repoOrError} />} />
    }

    const repoMatchURL = '/' + encodeURIPathComponent(repoName)

    const context: RepoContainerContext = {
        ...props,
        ...repoHeaderContributionsLifecycleProps,
        ...childBreadcrumbSetters,
        onHoverShown,
        repo: repoOrError,
        routePrefix: repoMatchURL,
        onDidUpdateExternalLinks: setExternalLinks,
    }

    return (
        <div className="repo-container test-repo-container w-100 d-flex flex-column">
            {showExtensionAlert && (
                <InstallBrowserExtensionAlert
                    isChrome={IS_CHROME}
                    onAlertDismissed={onAlertDismissed}
                    externalURLs={repoOrError.externalURLs}
                    codeHostIntegrationMessaging={codeHostIntegrationMessaging}
                />
            )}
            <RepoHeader
                {...props}
                actionButtons={props.repoHeaderActionButtons}
                revision={revision}
                repo={repoOrError}
                resolvedRev={resolvedRevisionOrError}
                onLifecyclePropsChange={setRepoHeaderContributionsLifecycleProps}
                isAlertDisplayed={showExtensionAlert}
            />
            <RepoHeaderContributionPortal
                position="right"
                priority={2}
                {...repoHeaderContributionsLifecycleProps}
                element={
                    <GoToCodeHostAction
                        key="go-to-code-host"
                        repo={repoOrError}
                        // We need a revision to generate code host URLs, if revision isn't available, we use the default branch or HEAD.
                        revision={rawRevision || repoOrError.defaultBranch?.displayName || 'HEAD'}
                        filePath={filePath}
                        commitRange={commitRange}
                        position={position}
                        range={range}
                        externalLinks={externalLinks}
                        fetchFileExternalLinks={fetchFileExternalLinks}
                        canShowPopover={canShowPopover}
                        onPopoverDismissed={onPopoverDismissed}
                    />
                }
            />
            <ErrorBoundary location={props.location}>
                <Switch>
                    {/* eslint-disable react/jsx-no-bind */}
                    {[
                        '',
                        ...(rawRevision ? [`@${rawRevision}`] : []), // must exactly match how the revision was encoded in the URL
                        '/-/blob',
                        '/-/tree',
                        '/-/commits',
                    ].map(routePath => (
                        <Route
                            path={`${repoMatchURL}${routePath}`}
                            key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                            exact={routePath === ''}
                            render={routeComponentProps => (
                                <RepoRevisionContainer
                                    {...routeComponentProps}
                                    {...context}
                                    {...childBreadcrumbSetters}
                                    routes={props.repoRevisionContainerRoutes}
                                    revision={revision || ''}
                                    resolvedRevisionOrError={resolvedRevisionOrError}
                                    // must exactly match how the revision was encoded in the URL
                                    routePrefix={`${repoMatchURL}${rawRevision ? `@${rawRevision}` : ''}`}
                                />
                            )}
                        />
                    ))}
                    {props.repoContainerRoutes.map(
                        ({ path, render, exact, condition = () => true }) =>
                            condition(context) && (
                                <Route
                                    path={context.routePrefix + path}
                                    key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                                    exact={exact}
                                    // RouteProps.render is an exception
                                    render={routeComponentProps => render({ ...context, ...routeComponentProps })}
                                />
                            )
                    )}
                    <Route key="hardcoded-key" component={RepoPageNotFound} />
                    {/* eslint-enable react/jsx-no-bind */}
                </Switch>
            </ErrorBoundary>
        </div>
    )
}
