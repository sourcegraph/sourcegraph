import { type FC, Suspense, useCallback, useLayoutEffect, useState } from 'react'

import classNames from 'classnames'
import { matchPath, useLocation, Route, Routes, Navigate, type RouteObject } from 'react-router-dom'

import { useKeyboardShortcut } from '@sourcegraph/shared/src/keyboardShortcuts/useKeyboardShortcut'
import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { useTheme, Theme } from '@sourcegraph/shared/src/theme'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { FeedbackPrompt, LoadingSpinner, useLocalStorage } from '@sourcegraph/wildcard'

import { communitySearchContextsRoutes } from './communitySearchContexts/routes'
import { AppRouterContainer } from './components/AppRouterContainer'
import { RouteError } from './components/ErrorBoundary'
import { LazyFuzzyFinder } from './components/fuzzyFinder/LazyFuzzyFinder'
import { KeyboardShortcutsHelp } from './components/KeyboardShortcutsHelp/KeyboardShortcutsHelp'
import { useScrollToLocationHash } from './components/useScrollToLocationHash'
import { useUserHistory } from './components/useUserHistory'
import { GlobalContributions } from './contributions'
import { useFeatureFlag } from './featureFlags/useFeatureFlag'
import { GlobalAlerts } from './global/GlobalAlerts'
import { useHandleSubmitFeedback } from './hooks'
import type { LegacyLayoutRouteContext } from './LegacyRouteContext'
import { SurveyToast } from './marketing/toast'
import { GlobalNavbar } from './nav/GlobalNavbar'
import { NewGlobalNavigationBar, useNewSearchNavigation } from './nav/new-global-navigation'
import { PageRoutes } from './routes.constants'
import { parseSearchURLQuery } from './search'
import { SearchQueryStateObserver } from './SearchQueryStateObserver'
import { isSourcegraphDev, useDeveloperSettings } from './stores'
import { isAccessTokenCallbackPage } from './user/settings/accessTokens/UserSettingsCreateAccessTokenCallbackPage'

import styles from './storm/pages/LayoutPage/LayoutPage.module.scss'

const LazySetupWizard = lazyComponent(() => import('./setup-wizard/SetupWizard'), 'SetupWizard')
const LazyDeveloperDialog = lazyComponent(() => import('./devsettings/DeveloperDialog'), 'DeveloperDialog')

export interface LegacyLayoutProps
    extends Omit<LegacyLayoutRouteContext, 'breadcrumbs' | 'useBreadcrumb' | 'setBreadcrumb' | 'isMacPlatform'> {
    children?: never
}

/**
 * Syntax highlighting changes for WCAG 2.1 contrast compliance (currently behind feature flag)
 * https://github.com/sourcegraph/sourcegraph/issues/36251
 */
const CONTRAST_COMPLIANT_CLASSNAME = 'theme-contrast-compliant-syntax-highlighting'

export const LegacyLayout: FC<LegacyLayoutProps> = props => {
    const location = useLocation()

    // TODO: Replace with useMatches once top-level <Router/> is V6
    const route = props.routes.find(
        route =>
            (route.path && matchPath(route.path, location.pathname)) ||
            (route.path && matchPath(route.path.replace(/\/\*$/, ''), location.pathname))
    )

    const routeMatch = route?.path

    const isSearchRelatedPage = (routeMatch === PageRoutes.RepoContainer || routeMatch?.startsWith('/search')) ?? false
    const isSearchHomepage = location.pathname === '/search' && !parseSearchURLQuery(location.search)
    const isSearchConsolePage = routeMatch?.startsWith('/search/console')
    const isSearchJobsPage = routeMatch?.startsWith(PageRoutes.SearchJobs)
    const isSearchNotebooksPage = routeMatch?.startsWith(PageRoutes.Notebooks)
    const isCodySearchPage = routeMatch === PageRoutes.CodySearch
    const isRepositoryRelatedPage = routeMatch === PageRoutes.RepoContainer ?? false

    // Since the access token callback page is rendered in a nested route, we can't use
    // `route.handle.isFullPage` to determine whether to render the header. Instead, we check
    // whether the current page is the access token callback page.
    const isAuthTokenCallbackPage = isAccessTokenCallbackPage()

    const isFullPageRoute = !!route?.handle?.isFullPage || isAuthTokenCallbackPage

    // eslint-disable-next-line no-restricted-syntax
    const [wasSetupWizardSkipped] = useLocalStorage('setup.skipped', false)

    const showDeveloperDialog =
        useDeveloperSettings(state => state.showDialog) &&
        (process.env.NODE_ENV === 'development' || isSourcegraphDev(props.authenticatedUser))
    const { fuzzyFinder } = useExperimentalFeatures(features => ({
        // enable fuzzy finder by default unless it's explicitly disabled in settings.
        fuzzyFinder: features.fuzzyFinder ?? true,
    }))
    const isSetupWizardPage = location.pathname.startsWith(PageRoutes.SetupWizard)

    const [isFuzzyFinderVisible, setFuzzyFinderVisible] = useState(false)
    const userHistory = useUserHistory(props.authenticatedUser?.id, isRepositoryRelatedPage)

    const communitySearchContextPaths = communitySearchContextsRoutes.map(route => route.path)
    const isCommunitySearchContextPage = communitySearchContextPaths.includes(location.pathname)

    // TODO add a component layer as the parent of the Layout component rendering "top-level" routes that do not render the navbar,
    // so that Layout can always render the navbar.
    const needsSiteInit = window.context?.needsSiteInit
    const disableFeedbackSurvey = window.context?.disableFeedbackSurvey
    const isSiteInit = location.pathname === PageRoutes.SiteAdminInit.toString()
    const isSignInOrUp =
        routeMatch &&
        [
            PageRoutes.SignIn,
            PageRoutes.SignUp,
            PageRoutes.PasswordReset,
            PageRoutes.Welcome,
            PageRoutes.RequestAccess,
        ].includes(routeMatch as PageRoutes)
    const isGetCodyPage = location.pathname === PageRoutes.GetCody.toString()
    const isPostSignUpPage = location.pathname === PageRoutes.PostSignUp.toString()

    const [newSearchNavigation] = useNewSearchNavigation()
    const [enableContrastCompliantSyntaxHighlighting] = useFeatureFlag('contrast-compliant-syntax-highlighting')

    const { theme } = useTheme()
    const showHelpShortcut = useKeyboardShortcut('keyboardShortcutsHelp')
    const [keyboardShortcutsHelpOpen, setKeyboardShortcutsHelpOpen] = useState(false)
    const [feedbackModalOpen, setFeedbackModalOpen] = useState(false)
    const showKeyboardShortcutsHelp = useCallback(() => setKeyboardShortcutsHelpOpen(true), [])
    const hideKeyboardShortcutsHelp = useCallback(() => setKeyboardShortcutsHelpOpen(false), [])
    const showFeedbackModal = useCallback(() => setFeedbackModalOpen(true), [])

    const { handleSubmitFeedback } = useHandleSubmitFeedback({
        routeMatch,
    })

    useLayoutEffect(() => {
        const isLightTheme = theme === Theme.Light

        document.documentElement.classList.add('theme')
        document.documentElement.classList.toggle('theme-light', isLightTheme)
        document.documentElement.classList.toggle('theme-dark', !isLightTheme)
    }, [theme])

    useScrollToLocationHash(location)

    // Note: this was a poor UX and is disabled for now, see https://github.com/sourcegraph/sourcegraph/issues/30192
    // const [tosAccepted, setTosAccepted] = useState(true) // Assume TOS has been accepted so that we don't show the TOS modal on initial load
    // useEffect(() => setTosAccepted(!props.authenticatedUser || props.authenticatedUser.tosAccepted), [
    //     props.authenticatedUser,
    // ])
    // const afterTosAccepted = useCallback(() => {
    //     setTosAccepted(true)
    // }, [])

    // Remove trailing slash (which is never valid in any of our URLs).
    if (location.pathname !== '/' && location.pathname.endsWith('/')) {
        return <Navigate replace={true} to={{ ...location, pathname: location.pathname.slice(0, -1) }} />
    }

    if (isSetupWizardPage && !!props.authenticatedUser?.siteAdmin) {
        return (
            <Suspense
                fallback={
                    <div className="flex flex-1">
                        <LoadingSpinner className="m-2" />
                    </div>
                }
            >
                <LazySetupWizard telemetryService={props.telemetryService} />
            </Suspense>
        )
    }

    // We have to use window.context here instead of injected context-based
    // props because we have to have this prop changes over time based on
    // setup wizard state, since we don't have a good solution for this at the
    // moment, we use mutable window.context object here.
    // TODO remove window.context and use injected context store/props
    if (window.context?.needsRepositoryConfiguration && !wasSetupWizardSkipped && props.authenticatedUser?.siteAdmin) {
        return <Navigate to={PageRoutes.SetupWizard} replace={true} />
    }

    // Some routes by their design require rendering on a blank page
    // without the UI chrome that Layout component renders by default.
    // If route has handle: { fullPage: true } we render just the route content
    // and its container block without rendering global nav
    // and other standard UI chrome elements.
    if (isFullPageRoute) {
        return <ApplicationRoutes routes={props.routes} />
    }

    const showNavigationSearchBox =
        isSearchRelatedPage &&
        !isSearchHomepage &&
        !isCommunitySearchContextPage &&
        !isSearchConsolePage &&
        !isSearchNotebooksPage &&
        !isCodySearchPage &&
        !isSearchJobsPage

    return (
        <div
            className={classNames(
                styles.layout,
                enableContrastCompliantSyntaxHighlighting && CONTRAST_COMPLIANT_CLASSNAME
            )}
        >
            {showHelpShortcut?.keybindings.map((keybinding, index) => (
                <Shortcut key={index} {...keybinding} onMatch={showKeyboardShortcutsHelp} />
            ))}
            <KeyboardShortcutsHelp isOpen={keyboardShortcutsHelpOpen} onDismiss={hideKeyboardShortcutsHelp} />

            {feedbackModalOpen && (
                <FeedbackPrompt
                    onSubmit={handleSubmitFeedback}
                    modal={true}
                    openByDefault={true}
                    authenticatedUser={
                        props.authenticatedUser
                            ? {
                                  username: props.authenticatedUser.username || '',
                                  email: props.authenticatedUser.emails.find(email => email.isPrimary)?.email || '',
                              }
                            : null
                    }
                    onClose={() => setFeedbackModalOpen(false)}
                />
            )}

            <GlobalAlerts authenticatedUser={props.authenticatedUser} />
            {!isSiteInit && !isSignInOrUp && !props.isSourcegraphDotCom && !disableFeedbackSurvey && (
                <SurveyToast authenticatedUser={props.authenticatedUser} />
            )}
            {!isSiteInit && !isSignInOrUp && !isGetCodyPage && !isPostSignUpPage && (
                <>
                    {newSearchNavigation ? (
                        <NewGlobalNavigationBar
                            routes={props.routes}
                            showSearchBox={showNavigationSearchBox}
                            authenticatedUser={props.authenticatedUser}
                            isSourcegraphDotCom={props.isSourcegraphDotCom}
                            notebooksEnabled={props.notebooksEnabled}
                            searchContextsEnabled={props.searchContextsEnabled}
                            codeMonitoringEnabled={props.codeMonitoringEnabled}
                            batchChangesEnabled={props.batchChangesEnabled}
                            codeInsightsEnabled={props.codeInsightsEnabled ?? false}
                            showFeedbackModal={showFeedbackModal}
                            selectedSearchContextSpec={props.selectedSearchContextSpec}
                            telemetryService={props.telemetryService}
                        />
                    ) : (
                        <GlobalNavbar
                            {...props}
                            showSearchBox={showNavigationSearchBox}
                            setFuzzyFinderIsVisible={setFuzzyFinderVisible}
                            isRepositoryRelatedPage={isRepositoryRelatedPage}
                            showKeyboardShortcutsHelp={showKeyboardShortcutsHelp}
                            showFeedbackModal={showFeedbackModal}
                        />
                    )}
                </>
            )}
            {needsSiteInit && !isSiteInit && <Navigate replace={true} to="/site-admin/init" />}
            <ApplicationRoutes routes={props.routes} />
            <GlobalContributions key={3} />
            {fuzzyFinder && (
                <LazyFuzzyFinder
                    isVisible={isFuzzyFinderVisible}
                    setIsVisible={setFuzzyFinderVisible}
                    isRepositoryRelatedPage={isRepositoryRelatedPage}
                    settingsCascade={props.settingsCascade}
                    telemetryService={props.telemetryService}
                    telemetryRecorder={props.platformContext.telemetryRecorder}
                    location={location}
                    userHistory={userHistory}
                />
            )}
            {showDeveloperDialog && <LazyDeveloperDialog />}
            <SearchQueryStateObserver
                platformContext={props.platformContext}
                searchContextsEnabled={props.searchAggregationEnabled}
                setSelectedSearchContextSpec={props.setSelectedSearchContextSpec}
                selectedSearchContextSpec={props.selectedSearchContextSpec}
            />
        </div>
    )
}

interface ApplicationRoutes {
    routes: RouteObject[]
}

const ApplicationRoutes: FC<ApplicationRoutes> = props => {
    const { routes } = props

    return (
        <Suspense
            fallback={
                <div className="flex flex-1">
                    <LoadingSpinner className="m-2" />
                </div>
            }
        >
            <AppRouterContainer>
                <Routes>
                    {routes.map(({ ...route }) => (
                        <Route
                            key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                            path={route.path}
                            element={route.element}
                            handle={route.handle}
                            errorElement={<RouteError />}
                        />
                    ))}
                </Routes>
            </AppRouterContainer>
            {/**
             * The portal root is inside the suspense boundary so that it is hidden
             * when we navigate to the lazily loaded routes or other actions which trigger
             * the Suspense boundary to show the fallback UI. Existing children are not unmounted
             * until the promise is resolved.
             *
             * See: https://github.com/facebook/react/pull/15861
             */}
            <div id="references-panel-react-portal" />
        </Suspense>
    )
}
