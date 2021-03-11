import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import React, { Suspense, useCallback, useEffect, useMemo } from 'react'
import { Redirect, Route, RouteComponentProps, Switch, matchPath } from 'react-router'
import { Observable } from 'rxjs'
import { ActivationProps } from '../../shared/src/components/activation/Activation'
import { FetchFileParameters } from '../../shared/src/components/CodeExcerpt'
import { ExtensionsControllerProps } from '../../shared/src/extensions/controller'
import * as GQL from '../../shared/src/graphql/schema'
import { ResizablePanel } from '../../branded/src/components/panel/Panel'
import { PlatformContextProps } from '../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../shared/src/settings/settings'
import { ErrorLike } from '../../shared/src/util/errors'
import { parseHash } from '../../shared/src/util/url'
import { ErrorBoundary } from './components/ErrorBoundary'
import { useScrollToLocationHash } from './components/useScrollToLocationHash'
import { GlobalContributions } from './contributions'
import { ExtensionAreaRoute } from './extensions/extension/ExtensionArea'
import { ExtensionAreaHeaderNavItem } from './extensions/extension/ExtensionAreaHeader'
import { ExtensionsAreaRoute } from './extensions/ExtensionsArea'
import { ExtensionsAreaHeaderActionButton } from './extensions/ExtensionsAreaHeader'
import { GlobalAlerts } from './global/GlobalAlerts'
import { GlobalDebug } from './global/GlobalDebug'
import { KeyboardShortcutsHelp } from './keyboardShortcuts/KeyboardShortcutsHelp'
import { GlobalNavbar } from './nav/GlobalNavbar'
import { OrgAreaRoute } from './org/area/OrgArea'
import { OrgAreaHeaderNavItem } from './org/area/OrgHeader'
import { fetchHighlightedFileLineRanges } from './repo/backend'
import { RepoContainerRoute } from './repo/RepoContainer'
import { RepoHeaderActionButton } from './repo/RepoHeader'
import { RepoRevisionContainerRoute } from './repo/RepoRevisionContainer'
import { LayoutRouteProps } from './routes'
import {
    parseSearchURLQuery,
    PatternTypeProps,
    CaseSensitivityProps,
    CopyQueryButtonProps,
    RepogroupHomepageProps,
    OnboardingTourProps,
    HomePanelsProps,
    SearchStreamingProps,
    ParsedSearchQueryProps,
    MutableVersionContextProps,
    parseSearchURL,
    SearchContextProps,
    isSearchContextSpecAvailable,
} from './search'
import { SiteAdminAreaRoute } from './site-admin/SiteAdminArea'
import { SiteAdminSideBarGroups } from './site-admin/SiteAdminSidebar'
import { UserAreaRoute } from './user/area/UserArea'
import { UserAreaHeaderNavItem } from './user/area/UserAreaHeader'
import { UserSettingsAreaRoute } from './user/settings/UserSettingsArea'
import { UserSettingsSidebarItems } from './user/settings/UserSettingsSidebar'
import { parseBrowserRepoURL } from './util/url'
import { SurveyToast } from './marketing/SurveyToast'
import { ThemeProps } from '../../shared/src/theme'
import { ThemePreferenceProps } from './theme'
import { KeyboardShortcutsProps, KEYBOARD_SHORTCUT_SHOW_HELP } from './keyboardShortcuts/keyboardShortcuts'
import { QueryState } from './search/helpers'
import { RepoSettingsAreaRoute } from './repo/settings/RepoSettingsArea'
import { RepoSettingsSideBarGroup } from './repo/settings/RepoSettingsSidebar'
import { Settings } from './schema/settings.schema'
import { Remote } from 'comlink'
import { FlatExtensionHostAPI } from '../../shared/src/api/contract'
import { useBreadcrumbs } from './components/Breadcrumbs'
import { AuthenticatedUser, authRequired as authRequiredObservable } from './auth'
import { SearchPatternType } from './graphql-operations'
import { TelemetryProps } from '../../shared/src/telemetry/telemetryService'
import { useObservable } from '../../shared/src/util/useObservable'
import { useExtensionAlertAnimation } from './nav/UserNavItem'
import { CodeMonitoringProps } from './enterprise/code-monitoring'
import { UserRepositoriesUpdateProps } from './util'
import { FilterKind, findFilter } from '../../shared/src/search/query/validate'
import { FilterType } from '../../shared/src/search/query/filters'
import { omitContextFilter } from '../../shared/src/search/query/transformer'

export interface LayoutProps
    extends RouteComponentProps<{}>,
        SettingsCascadeProps<Settings>,
        PlatformContextProps,
        ExtensionsControllerProps,
        KeyboardShortcutsProps,
        ThemeProps,
        TelemetryProps,
        ThemePreferenceProps,
        ActivationProps,
        ParsedSearchQueryProps,
        PatternTypeProps,
        CaseSensitivityProps,
        CopyQueryButtonProps,
        MutableVersionContextProps,
        RepogroupHomepageProps,
        OnboardingTourProps,
        SearchContextProps,
        HomePanelsProps,
        SearchStreamingProps,
        CodeMonitoringProps,
        SearchContextProps,
        UserRepositoriesUpdateProps {
    extensionAreaRoutes: readonly ExtensionAreaRoute[]
    extensionAreaHeaderNavItems: readonly ExtensionAreaHeaderNavItem[]
    extensionsAreaRoutes: readonly ExtensionsAreaRoute[]
    extensionsAreaHeaderActionButtons: readonly ExtensionsAreaHeaderActionButton[]
    siteAdminAreaRoutes: readonly SiteAdminAreaRoute[]
    siteAdminSideBarGroups: SiteAdminSideBarGroups
    siteAdminOverviewComponents: readonly React.ComponentType[]
    userAreaHeaderNavItems: readonly UserAreaHeaderNavItem[]
    userAreaRoutes: readonly UserAreaRoute[]
    userSettingsSideBarItems: UserSettingsSidebarItems
    userSettingsAreaRoutes: readonly UserSettingsAreaRoute[]
    orgAreaHeaderNavItems: readonly OrgAreaHeaderNavItem[]
    orgAreaRoutes: readonly OrgAreaRoute[]
    repoContainerRoutes: readonly RepoContainerRoute[]
    repoRevisionContainerRoutes: readonly RepoRevisionContainerRoute[]
    repoHeaderActionButtons: readonly RepoHeaderActionButton[]
    repoSettingsAreaRoutes: readonly RepoSettingsAreaRoute[]
    repoSettingsSidebarGroups: readonly RepoSettingsSideBarGroup[]
    routes: readonly LayoutRouteProps<any>[]

    authenticatedUser: AuthenticatedUser | null

    /**
     * The subject GraphQL node ID of the viewer, which is used to look up the viewer's settings. This is either
     * the site's GraphQL node ID (for anonymous users) or the authenticated user's GraphQL node ID.
     */
    viewerSubject: Pick<GQL.ISettingsSubject, 'id' | 'viewerCanAdminister'>

    // Search
    navbarSearchQueryState: QueryState
    onNavbarQueryChange: (queryState: QueryState) => void
    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>
    searchRequest: (
        query: QueryState['query'],
        version: string,
        patternType: SearchPatternType,
        versionContext: string | undefined,
        extensionHostPromise: Promise<Remote<FlatExtensionHostAPI>>
    ) => Observable<GQL.ISearchResults | ErrorLike>

    globbing: boolean
    showMultilineSearchConsole: boolean
    showQueryBuilder: boolean
    enableSmartQuery: boolean
    isSourcegraphDotCom: boolean
    showBatchChanges: boolean
    fetchSavedSearches: () => Observable<GQL.ISavedSearch[]>
    children?: never
}

export const Layout: React.FunctionComponent<LayoutProps> = props => {
    const routeMatch = props.routes.find(({ path, exact }) => matchPath(props.location.pathname, { path, exact }))?.path
    const isSearchRelatedPage = (routeMatch === '/:repoRevAndRest+' || routeMatch?.startsWith('/search')) ?? false
    const minimalNavLinks = routeMatch === '/cncf'
    const isSearchHomepage = props.location.pathname === '/search' && !parseSearchURLQuery(props.location.search)

    // Update parsedSearchQuery, patternType, caseSensitivity, versionContext, and selectedSearchContextSpec based on current URL
    const {
        history,
        parsedSearchQuery: currentQuery,
        patternType: currentPatternType,
        caseSensitive: currentCaseSensitive,
        versionContext: currentVersionContext,
        selectedSearchContextSpec,
        availableSearchContexts,
        location,
        setParsedSearchQuery,
        setPatternType,
        setCaseSensitivity,
        setVersionContext,
        setSelectedSearchContextSpec,
    } = props

    const { query = '', patternType, caseSensitive, versionContext } = useMemo(() => parseSearchURL(location.search), [
        location.search,
    ])

    useEffect(() => {
        const globalContextFilter = findFilter(query, FilterType.context, FilterKind.Global)
        const searchContextSpec = globalContextFilter?.value ? globalContextFilter.value.value : undefined

        let finalQuery = query
        if (
            globalContextFilter &&
            searchContextSpec &&
            isSearchContextSpecAvailable(searchContextSpec, availableSearchContexts)
        ) {
            // If a global search context spec is available to the user, we omit it from the
            // query and move it to the search contexts dropdown
            finalQuery = omitContextFilter(finalQuery, globalContextFilter)
        }

        if (finalQuery !== currentQuery) {
            setParsedSearchQuery(finalQuery)
        }

        // Only override filters from URL if there is a search query
        if (query) {
            if (patternType && patternType !== currentPatternType) {
                setPatternType(patternType)
            }

            if (caseSensitive !== currentCaseSensitive) {
                setCaseSensitivity(caseSensitive)
            }

            if (versionContext !== currentVersionContext) {
                setVersionContext(versionContext)
            }

            if (searchContextSpec && searchContextSpec !== selectedSearchContextSpec) {
                setSelectedSearchContextSpec(searchContextSpec)
            }
        }
    }, [
        history,
        caseSensitive,
        currentCaseSensitive,
        currentPatternType,
        currentQuery,
        currentVersionContext,
        selectedSearchContextSpec,
        patternType,
        query,
        setCaseSensitivity,
        setParsedSearchQuery,
        setPatternType,
        setVersionContext,
        versionContext,
        setSelectedSearchContextSpec,
        availableSearchContexts,
    ])

    // Hack! Hardcode these routes into cmd/frontend/internal/app/ui/router.go
    const repogroupPages = [
        '/refactor-python2-to-3',
        '/kubernetes',
        '/golang',
        '/react-hooks',
        '/android',
        '/stanford',
        '/cncf',
    ]
    const isRepogroupPage = repogroupPages.includes(props.location.pathname)

    // TODO add a component layer as the parent of the Layout component rendering "top-level" routes that do not render the navbar,
    // so that Layout can always render the navbar.
    const needsSiteInit = window.context?.needsSiteInit
    const isSiteInit = props.location.pathname === '/site-admin/init'
    const isSignInOrUp =
        props.location.pathname === '/sign-in' ||
        props.location.pathname === '/sign-up' ||
        props.location.pathname === '/password-reset'

    const authRequired = useObservable(authRequiredObservable)

    const hideGlobalSearchInput: boolean =
        props.location.pathname === '/stats' ||
        props.location.pathname === '/search/query-builder' ||
        props.location.pathname === '/search/console'

    const breadcrumbProps = useBreadcrumbs()

    // Control browser extension discoverability animation here.
    // `Layout` is the lowest common ancestor of `UserNavItem` (target) and `RepoContainer` (trigger)
    const { isExtensionAlertAnimating, startExtensionAlertAnimation } = useExtensionAlertAnimation()
    const onExtensionAlertDismissed = useCallback(() => {
        startExtensionAlertAnimation()
    }, [startExtensionAlertAnimation])

    useScrollToLocationHash(props.location)
    // Remove trailing slash (which is never valid in any of our URLs).
    if (props.location.pathname !== '/' && props.location.pathname.endsWith('/')) {
        return <Redirect to={{ ...props.location, pathname: props.location.pathname.slice(0, -1) }} />
    }

    const context = {
        ...props,
        ...breadcrumbProps,
        onExtensionAlertDismissed,
    }

    return (
        <div className="layout">
            <KeyboardShortcutsHelp
                keyboardShortcutForShow={KEYBOARD_SHORTCUT_SHOW_HELP}
                keyboardShortcuts={props.keyboardShortcuts}
            />
            <GlobalAlerts
                isSiteAdmin={!!props.authenticatedUser && props.authenticatedUser.siteAdmin}
                settingsCascade={props.settingsCascade}
                history={props.history}
            />
            {!isSiteInit && <SurveyToast authenticatedUser={props.authenticatedUser} />}
            {!isSiteInit && !isSignInOrUp && (
                <GlobalNavbar
                    {...props}
                    authRequired={!!authRequired}
                    isSearchRelatedPage={isSearchRelatedPage}
                    variant={
                        hideGlobalSearchInput
                            ? 'no-search-input'
                            : isSearchHomepage
                            ? 'low-profile'
                            : isRepogroupPage
                            ? 'low-profile-with-logo'
                            : 'default'
                    }
                    hideNavLinks={false}
                    minimalNavLinks={minimalNavLinks}
                    isExtensionAlertAnimating={isExtensionAlertAnimating}
                />
            )}
            {needsSiteInit && !isSiteInit && <Redirect to="/site-admin/init" />}
            <ErrorBoundary location={props.location}>
                <Suspense fallback={<LoadingSpinner className="icon-inline m-2" />}>
                    <Switch>
                        {/* eslint-disable react/jsx-no-bind */}
                        {props.routes.map(
                            ({ render, condition = () => true, ...route }) =>
                                condition(context) && (
                                    <Route
                                        {...route}
                                        key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                                        component={undefined}
                                        render={routeComponentProps => (
                                            <div className="layout__app-router-container">
                                                {render({ ...context, ...routeComponentProps })}
                                            </div>
                                        )}
                                    />
                                )
                        )}
                        {/* eslint-enable react/jsx-no-bind */}
                    </Switch>
                </Suspense>
            </ErrorBoundary>
            {parseHash(props.location.hash).viewState && props.location.pathname !== '/sign-in' && (
                <ResizablePanel
                    {...props}
                    repoName={`git://${parseBrowserRepoURL(props.location.pathname).repoName}`}
                    fetchHighlightedFileLineRanges={fetchHighlightedFileLineRanges}
                />
            )}
            <GlobalContributions
                key={3}
                extensionsController={props.extensionsController}
                platformContext={props.platformContext}
                history={props.history}
            />
            <GlobalDebug {...props} />
        </div>
    )
}
