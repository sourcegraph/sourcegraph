import React, { Suspense, useCallback, useLayoutEffect, useState } from 'react'

import classNames from 'classnames'
import { Outlet, useLocation, Navigate, useMatches, useMatch } from 'react-router-dom'

import { TabbedPanelContent } from '@sourcegraph/branded/src/components/panel/TabbedPanelContent'
import { useKeyboardShortcut } from '@sourcegraph/shared/src/keyboardShortcuts/useKeyboardShortcut'
import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { useTheme, Theme } from '@sourcegraph/shared/src/theme'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { parseQueryAndHash } from '@sourcegraph/shared/src/util/url'
import { FeedbackPrompt, LoadingSpinner, Panel } from '@sourcegraph/wildcard'

import { communitySearchContextsRoutes } from './communitySearchContexts/routes'
import { AppRouterContainer } from './components/AppRouterContainer'
import { ErrorBoundary } from './components/ErrorBoundary'
import { LazyFuzzyFinder } from './components/fuzzyFinder/LazyFuzzyFinder'
import { KeyboardShortcutsHelp } from './components/KeyboardShortcutsHelp/KeyboardShortcutsHelp'
import { useScrollToLocationHash } from './components/useScrollToLocationHash'
import { useUserHistory } from './components/useUserHistory'
import { useGlobalContributions } from './contributions'
import { useFeatureFlag } from './featureFlags/useFeatureFlag'
import { GlobalAlerts } from './global/GlobalAlerts'
import { useHandleSubmitFeedback } from './hooks'
import { LegacyLayoutRouteContext } from './LegacyRouteContext'
import { SurveyToast } from './marketing/toast'
import { GlobalNavbar } from './nav/GlobalNavbar'
import { EnterprisePageRoutes, PageRoutes } from './routes.constants'
import { parseSearchURLQuery } from './search'
import { NotepadContainer } from './search/Notepad'
import { SearchQueryStateObserver } from './SearchQueryStateObserver'
import { parseBrowserRepoURL } from './util/url'

import styles from './Layout.module.scss'

const LazySetupWizard = lazyComponent(() => import('./setup-wizard'), 'SetupWizard')

export interface LegacyLayoutProps extends LegacyLayoutRouteContext {
    children?: never
}

/**
 * Syntax highlighting changes for WCAG 2.1 contrast compliance (currently behind feature flag)
 * https://github.com/sourcegraph/sourcegraph/issues/36251
 */
const CONTRAST_COMPLIANT_CLASSNAME = 'theme-contrast-compliant-syntax-highlighting'

function useIsSignInOrSignUpPage(): boolean {
    const isSignInPage = useMatch(PageRoutes.SignIn)
    const isSignUpPage = useMatch(PageRoutes.SignUp)
    const isPasswordResetPage = useMatch(PageRoutes.PasswordReset)
    const isWelcomePage = useMatch(PageRoutes.Welcome)
    const isRequestAccessPage = useMatch(PageRoutes.RequestAccess)
    return !!(isSignInPage || isSignUpPage || isPasswordResetPage || isWelcomePage || isRequestAccessPage)
}
export const Layout: React.FC<LegacyLayoutProps> = props => {
    const location = useLocation()

    const routeMatches = useMatches()

    const isRepositoryRelatedPage =
        routeMatches.some(
            routeMatch =>
                routeMatch.handle &&
                typeof routeMatch.handle === 'object' &&
                Object.hasOwn(routeMatch.handle, 'isRepoContainer')
        ) ?? false
    // TODO: Move the search box into a shared layout component that is only used for repo routes
    //       and search routes once we have flattened the router hierarchy.
    const isSearchRelatedPage =
        (isRepositoryRelatedPage || routeMatches.some(routeMatch => routeMatch.pathname.startsWith('/search'))) ?? false
    const isSearchHomepage = location.pathname === '/search' && !parseSearchURLQuery(location.search)
    const isSearchConsolePage = routeMatches.some(routeMatch => routeMatch.pathname.startsWith('/search/console'))
    const isSearchNotebooksPage = routeMatches.some(routeMatch =>
        routeMatch.pathname.startsWith(EnterprisePageRoutes.Notebooks)
    )
    const isSearchNotebookListPage = location.pathname === EnterprisePageRoutes.Notebooks

    const { setupWizard, fuzzyFinder } = useExperimentalFeatures(features => ({
        setupWizard: features.setupWizard,
        // enable fuzzy finder by default unless it's explicitly disabled in settings
        fuzzyFinder: features.fuzzyFinder ?? true,
    }))
    const isSetupWizardPage = setupWizard && location.pathname.startsWith(PageRoutes.SetupWizard)

    const [isFuzzyFinderVisible, setFuzzyFinderVisible] = useState(false)
    const userHistory = useUserHistory(isRepositoryRelatedPage)

    const communitySearchContextPaths = communitySearchContextsRoutes.map(route => route.path)
    const isCommunitySearchContextPage = communitySearchContextPaths.includes(location.pathname)

    // TODO add a component layer as the parent of the Layout component rendering "top-level" routes that do not render the navbar,
    // so that Layout can always render the navbar.
    const needsSiteInit = window.context?.needsSiteInit
    const disableFeedbackSurvey = window.context?.disableFeedbackSurvey
    const isSiteInit = location.pathname === PageRoutes.SiteAdminInit
    const isSignInOrUp = useIsSignInOrSignUpPage()

    const [enableContrastCompliantSyntaxHighlighting] = useFeatureFlag('contrast-compliant-syntax-highlighting')

    const { theme } = useTheme()
    const [keyboardShortcutsHelpOpen, setKeyboardShortcutsHelpOpen] = useState(false)
    const [feedbackModalOpen, setFeedbackModalOpen] = useState(false)
    const showHelpShortcut = useKeyboardShortcut('keyboardShortcutsHelp')

    const showKeyboardShortcutsHelp = useCallback(() => setKeyboardShortcutsHelpOpen(true), [])
    const hideKeyboardShortcutsHelp = useCallback(() => setKeyboardShortcutsHelpOpen(false), [])
    const showFeedbackModal = useCallback(() => setFeedbackModalOpen(true), [])

    const { handleSubmitFeedback } = useHandleSubmitFeedback({
        routeMatch:
            routeMatches && routeMatches.length > 0 ? routeMatches[routeMatches.length - 1].pathname : undefined,
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

    useGlobalContributions()

    // Remove trailing slash (which is never valid in any of our URLs).
    if (location.pathname !== '/' && location.pathname.endsWith('/')) {
        return <Navigate replace={true} to={{ ...location, pathname: location.pathname.slice(0, -1) }} />
    }

    if (isSetupWizardPage) {
        return (
            <Suspense
                fallback={
                    <div className="flex flex-1">
                        <LoadingSpinner className="m-2" />
                    </div>
                }
            >
                <LazySetupWizard isSourcegraphApp={props.isSourcegraphApp} />
            </Suspense>
        )
    }

    // We have to use window.context here instead of injected context-based
    // props because we have to have this prop changes over time based on
    // setup wizard state, since we don't have a good solution for this at the
    // moment, we use mutable window.context object here.
    // TODO remove window.context and use injected context store/props
    if (setupWizard && window.context.needsRepositoryConfiguration) {
        return <Navigate to={PageRoutes.SetupWizard} replace={true} />
    }

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

            <GlobalAlerts
                authenticatedUser={props.authenticatedUser}
                settingsCascade={props.settingsCascade}
                isSourcegraphDotCom={props.isSourcegraphDotCom}
            />
            {!isSiteInit && !isSignInOrUp && !props.isSourcegraphDotCom && !disableFeedbackSurvey && (
                <SurveyToast authenticatedUser={props.authenticatedUser} />
            )}
            {!isSiteInit && !isSignInOrUp && (
                <GlobalNavbar
                    {...props}
                    routes={[]}
                    showSearchBox={
                        isSearchRelatedPage &&
                        !isSearchHomepage &&
                        !isCommunitySearchContextPage &&
                        !isSearchConsolePage &&
                        !isSearchNotebooksPage
                    }
                    setFuzzyFinderIsVisible={setFuzzyFinderVisible}
                    isRepositoryRelatedPage={isRepositoryRelatedPage}
                    showKeyboardShortcutsHelp={showKeyboardShortcutsHelp}
                    showFeedbackModal={showFeedbackModal}
                />
            )}
            {needsSiteInit && !isSiteInit && <Navigate replace={true} to="/site-admin/init" />}
            <ErrorBoundary location={location}>
                <Suspense
                    fallback={
                        <div className="flex flex-1">
                            <LoadingSpinner className="m-2" />
                        </div>
                    }
                >
                    <AppRouterContainer>
                        <Outlet />
                    </AppRouterContainer>
                </Suspense>
            </ErrorBoundary>
            {parseQueryAndHash(location.search, location.hash).viewState && location.pathname !== PageRoutes.SignIn && (
                <Panel
                    className={styles.panel}
                    position="bottom"
                    defaultSize={350}
                    storageKey="panel-size"
                    ariaLabel="References panel"
                    id="references-panel"
                >
                    <TabbedPanelContent
                        {...props}
                        repoName={`git://${parseBrowserRepoURL(location.pathname).repoName}`}
                        fetchHighlightedFileLineRanges={props.fetchHighlightedFileLineRanges}
                    />
                </Panel>
            )}
            {(isSearchNotebookListPage || (isSearchRelatedPage && !isSearchHomepage)) && (
                <NotepadContainer userId={props.authenticatedUser?.id} />
            )}
            {fuzzyFinder && (
                <LazyFuzzyFinder
                    isVisible={isFuzzyFinderVisible}
                    setIsVisible={setFuzzyFinderVisible}
                    isRepositoryRelatedPage={isRepositoryRelatedPage}
                    settingsCascade={props.settingsCascade}
                    telemetryService={props.telemetryService}
                    location={location}
                    userHistory={userHistory}
                />
            )}
            <SearchQueryStateObserver
                platformContext={props.platformContext}
                searchContextsEnabled={props.searchAggregationEnabled}
                setSelectedSearchContextSpec={props.setSelectedSearchContextSpec}
                selectedSearchContextSpec={props.selectedSearchContextSpec}
            />
        </div>
    )
}
