import React, { Suspense, useCallback, useLayoutEffect, useState } from 'react'

import classNames from 'classnames'
import { Outlet, useLocation, Navigate, useMatches, useMatch } from 'react-router-dom'

import { useKeyboardShortcut } from '@sourcegraph/shared/src/keyboardShortcuts/useKeyboardShortcut'
import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { useTheme, Theme } from '@sourcegraph/shared/src/theme'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { FeedbackPrompt, LoadingSpinner, useLocalStorage } from '@sourcegraph/wildcard'

import { communitySearchContextsRoutes } from '../../../communitySearchContexts/routes'
import { AppRouterContainer } from '../../../components/AppRouterContainer'
import { ErrorBoundary } from '../../../components/ErrorBoundary'
import { LazyFuzzyFinder } from '../../../components/fuzzyFinder/LazyFuzzyFinder'
import { KeyboardShortcutsHelp } from '../../../components/KeyboardShortcutsHelp/KeyboardShortcutsHelp'
import { useScrollToLocationHash } from '../../../components/useScrollToLocationHash'
import { useUserHistory } from '../../../components/useUserHistory'
import { useFeatureFlag } from '../../../featureFlags/useFeatureFlag'
import { GlobalAlerts } from '../../../global/GlobalAlerts'
import { useHandleSubmitFeedback } from '../../../hooks'
import type { LegacyLayoutRouteContext } from '../../../LegacyRouteContext'
import { CodySurveyToast, SurveyToast } from '../../../marketing/toast'
import { GlobalNavbar } from '../../../nav/GlobalNavbar'
import { PageRoutes } from '../../../routes.constants'
import { parseSearchURLQuery } from '../../../search'
import { SearchQueryStateObserver } from '../../../SearchQueryStateObserver'
import { isSourcegraphDev, useDeveloperSettings } from '../../../stores'

import styles from './LayoutPage.module.scss'

const LazySetupWizard = lazyComponent(() => import('../../../setup-wizard/SetupWizard'), 'SetupWizard')
const LazyDeveloperDialog = lazyComponent(() => import('../../../devsettings/DeveloperDialog'), 'DeveloperDialog')

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
    const isSearchNotebooksPage = routeMatches.some(routeMatch => routeMatch.pathname.startsWith(PageRoutes.Notebooks))
    const isCodySearchPage = routeMatches.some(routeMatch => routeMatch.pathname.startsWith(PageRoutes.CodySearch))

    // eslint-disable-next-line no-restricted-syntax
    const [wasSetupWizardSkipped] = useLocalStorage('setup.skipped', false)
    const { fuzzyFinder } = useExperimentalFeatures(features => ({
        // enable fuzzy finder by default unless it's explicitly disabled in settings
        fuzzyFinder: features.fuzzyFinder ?? true,
    }))
    const isSetupWizardPage = location.pathname.startsWith(PageRoutes.SetupWizard)

    const showDeveloperDialog =
        useDeveloperSettings(state => state.showDialog) &&
        (process.env.NODE_ENV === 'development' || isSourcegraphDev(props.authenticatedUser))
    const [isFuzzyFinderVisible, setFuzzyFinderVisible] = useState(false)
    const userHistory = useUserHistory(props.authenticatedUser?.id, isRepositoryRelatedPage)

    const communitySearchContextPaths = communitySearchContextsRoutes.map(route => route.path)
    const isCommunitySearchContextPage = communitySearchContextPaths.includes(location.pathname)

    // TODO add a component layer as the parent of the Layout component rendering "top-level" routes that do not render the navbar,
    // so that Layout can always render the navbar.
    const needsSiteInit = window.context?.needsSiteInit
    const disableFeedbackSurvey = window.context?.disableFeedbackSurvey
    const needsRepositoryConfiguration = window.context?.needsRepositoryConfiguration
    const isSiteInit = location.pathname === PageRoutes.SiteAdminInit
    const isSignInOrUp = useIsSignInOrSignUpPage()
    const isGetCodyPage = location.pathname === PageRoutes.GetCody

    const [enableContrastCompliantSyntaxHighlighting] = useFeatureFlag('contrast-compliant-syntax-highlighting')

    const { theme } = useTheme()
    const [keyboardShortcutsHelpOpen, setKeyboardShortcutsHelpOpen] = useState(false)
    const [feedbackModalOpen, setFeedbackModalOpen] = useState(false)
    const showHelpShortcut = useKeyboardShortcut('keyboardShortcutsHelp')

    const showKeyboardShortcutsHelp = useCallback(() => setKeyboardShortcutsHelpOpen(true), [])
    const hideKeyboardShortcutsHelp = useCallback(() => setKeyboardShortcutsHelpOpen(false), [])
    const showFeedbackModal = useCallback(() => setFeedbackModalOpen(true), [])

    const { handleSubmitFeedback } = useHandleSubmitFeedback({
        routeMatch: routeMatches && routeMatches.length > 0 ? routeMatches.at(-1)!.pathname : undefined,
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
    if (needsRepositoryConfiguration && !wasSetupWizardSkipped && props.authenticatedUser?.siteAdmin) {
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
                telemetryRecorder={props.platformContext.telemetryRecorder}
            />
            {!isSiteInit && !isSignInOrUp && !props.isSourcegraphDotCom && !disableFeedbackSurvey && (
                <SurveyToast authenticatedUser={props.authenticatedUser} />
            )}
            {!isSiteInit && props.isSourcegraphDotCom && props.authenticatedUser && (
                <CodySurveyToast
                    telemetryService={props.telemetryService}
                    authenticatedUser={props.authenticatedUser}
                />
            )}
            {!isSiteInit && !isSignInOrUp && !isGetCodyPage && (
                <GlobalNavbar
                    {...props}
                    routes={[]}
                    showSearchBox={
                        isSearchRelatedPage &&
                        !isSearchHomepage &&
                        !isCommunitySearchContextPage &&
                        !isSearchConsolePage &&
                        !isSearchNotebooksPage &&
                        !isCodySearchPage
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
            </ErrorBoundary>
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
