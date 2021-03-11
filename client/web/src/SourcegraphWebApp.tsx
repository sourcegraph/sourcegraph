import 'focus-visible'

import { ShortcutProvider } from '@slimsag/react-shortcuts'
import ServerIcon from 'mdi-react/ServerIcon'
import * as React from 'react'
import { hot } from 'react-hot-loader/root'
import { Route } from 'react-router'
import { BrowserRouter } from 'react-router-dom'
import { combineLatest, from, Subscription, fromEvent, of, Subject } from 'rxjs'
import { bufferCount, startWith, switchMap } from 'rxjs/operators'
import { setLinkComponent } from '../../shared/src/components/Link'
import {
    Controller as ExtensionsController,
    createController as createExtensionsController,
} from '../../shared/src/extensions/controller'
import { Notifications } from '../../shared/src/notifications/Notifications'
import { PlatformContext } from '../../shared/src/platform/context'
import { EMPTY_SETTINGS_CASCADE, SettingsCascadeProps } from '../../shared/src/settings/settings'
import { authenticatedUser, AuthenticatedUser } from './auth'
import { ErrorBoundary } from './components/ErrorBoundary'
import { FeedbackText } from './components/FeedbackText'
import { HeroPage } from './components/HeroPage'
import { RouterLinkOrAnchor } from './components/RouterLinkOrAnchor'
import { Tooltip } from '../../branded/src/components/tooltip/Tooltip'
import { ExtensionAreaRoute } from './extensions/extension/ExtensionArea'
import { ExtensionAreaHeaderNavItem } from './extensions/extension/ExtensionAreaHeader'
import { ExtensionsAreaRoute } from './extensions/ExtensionsArea'
import { ExtensionsAreaHeaderActionButton } from './extensions/ExtensionsAreaHeader'
import { Layout, LayoutProps } from './Layout'
import { updateUserSessionStores } from './marketing/util'
import { OrgAreaRoute } from './org/area/OrgArea'
import { OrgAreaHeaderNavItem } from './org/area/OrgHeader'
import { createPlatformContext } from './platform/context'
import { fetchHighlightedFileLineRanges } from './repo/backend'
import { RepoContainerRoute } from './repo/RepoContainer'
import { RepoHeaderActionButton } from './repo/RepoHeader'
import { RepoRevisionContainerRoute } from './repo/RepoRevisionContainer'
import { LayoutRouteProps } from './routes'
import {
    search,
    fetchSavedSearches,
    fetchRecentSearches,
    fetchRecentFileViews,
    fetchSearchContexts,
} from './search/backend'
import { SiteAdminAreaRoute } from './site-admin/SiteAdminArea'
import { SiteAdminSideBarGroups } from './site-admin/SiteAdminSidebar'
import { ThemePreference } from './theme'
import { eventLogger } from './tracking/eventLogger'
import { withActivation } from './tracking/withActivation'
import { UserAreaRoute } from './user/area/UserArea'
import { UserAreaHeaderNavItem } from './user/area/UserAreaHeader'
import { UserSettingsAreaRoute } from './user/settings/UserSettingsArea'
import { UserSettingsSidebarItems } from './user/settings/UserSettingsSidebar'
import { resolveVersionContext, parseSearchURL, resolveSearchContextSpec } from './search'
import { KeyboardShortcutsProps } from './keyboardShortcuts/keyboardShortcuts'
import { QueryState } from './search/helpers'
import { RepoSettingsAreaRoute } from './repo/settings/RepoSettingsArea'
import { RepoSettingsSideBarGroup } from './repo/settings/RepoSettingsSidebar'
import { NotificationType } from '../../shared/src/api/client/services/notifications'
import { VersionContext } from './schema/site.schema'
import { globbingEnabledFromSettings } from './util/globbing'
import {
    SITE_SUBJECT_NO_ADMIN,
    viewerSubjectFromSettings,
    defaultPatternTypeFromSettings,
    experimentalFeaturesFromSettings,
} from './util/settings'
import { SearchPatternType } from '../../shared/src/graphql-operations'
import { HTTPStatusError } from '../../shared/src/backend/fetch'
import {
    createCodeMonitor,
    deleteCodeMonitor,
    fetchCodeMonitor,
    fetchUserCodeMonitors,
    toggleCodeMonitorEnabled,
    updateCodeMonitor,
} from './enterprise/code-monitoring/backend'
import { aggregateStreamingSearch } from './search/stream'
import { ISearchContext } from '../../shared/src/graphql/schema'
import { logCodeInsightsChanges } from './insights/analytics'
import { listUserRepositories } from './site-admin/backend'

export interface SourcegraphWebAppProps extends KeyboardShortcutsProps {
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
    showBatchChanges: boolean
}

interface SourcegraphWebAppState extends SettingsCascadeProps {
    error?: Error

    /** The currently authenticated user (or null if the viewer is anonymous). */
    authenticatedUser?: AuthenticatedUser | null

    viewerSubject: LayoutProps['viewerSubject']

    /** The user's preference for the theme (light, dark or following system theme) */
    themePreference: ThemePreference

    /**
     * Whether the OS uses light theme, synced from a media query.
     * If the browser/OS does not this, will default to true.
     */
    systemIsLightTheme: boolean

    /**
     * The current search query in the navbar.
     */
    navbarSearchQueryState: QueryState

    /**
     * The current parsed search query, with all UI-configurable parameters
     * (eg. pattern type, case sensitivity, version context) removed
     */
    parsedSearchQuery: string

    /**
     * The current search pattern type.
     */
    searchPatternType: SearchPatternType

    /**
     * Whether the current search is case sensitive.
     */
    searchCaseSensitivity: boolean

    /**
     * Whether to display the copy query button.
     */
    copyQueryButton: boolean

    /*
     * The version context the instance is in. If undefined, it means no version context is selected.
     */
    versionContext?: string

    /**
     * Available version contexts defined in the site configuration.
     */
    availableVersionContexts?: VersionContext[]

    /**
     * The previously used version context, as specified in localStorage.
     */
    previousVersionContext: string | null

    showRepogroupHomepage: boolean

    showOnboardingTour: boolean

    showEnterpriseHomePanels: boolean

    showSearchContext: boolean
    availableSearchContexts: ISearchContext[]
    selectedSearchContextSpec?: string
    defaultSearchContextSpec: string
    hasUserAddedRepositories: boolean

    /**
     * Whether globbing is enabled for filters.
     */
    globbing: boolean

    /**
     * Whether we show the mulitiline editor at /search/console
     */
    showMultilineSearchConsole: boolean

    /**
     * Whether we show the mulitiline editor at /search/query-builder
     */
    showQueryBuilder: boolean

    /**
     * Wether to enable enable contextual syntax highlighting and hovers for search queries
     */
    enableSmartQuery: boolean

    /**
     * Whether the code monitoring feature flag is enabled.
     */
    enableCodeMonitoring: boolean
}

const notificationClassNames = {
    [NotificationType.Log]: 'alert alert-secondary',
    [NotificationType.Success]: 'alert alert-success',
    [NotificationType.Info]: 'alert alert-info',
    [NotificationType.Warning]: 'alert alert-warning',
    [NotificationType.Error]: 'alert alert-danger',
}

const LIGHT_THEME_LOCAL_STORAGE_KEY = 'light-theme'
const LAST_VERSION_CONTEXT_KEY = 'sg-last-version-context'
const LAST_SEARCH_CONTEXT_KEY = 'sg-last-search-context'

/** Reads the stored theme preference from localStorage */
const readStoredThemePreference = (): ThemePreference => {
    const value = localStorage.getItem(LIGHT_THEME_LOCAL_STORAGE_KEY)
    // Handle both old and new preference values
    switch (value) {
        case 'true':
        case 'light':
            return ThemePreference.Light
        case 'false':
        case 'dark':
            return ThemePreference.Dark
        default:
            return ThemePreference.System
    }
}

setLinkComponent(RouterLinkOrAnchor)

const LayoutWithActivation = window.context.sourcegraphDotComMode ? Layout : withActivation(Layout)

/**
 * The root component.
 *
 * This is the non-hot-reload component. It is wrapped in `hot(...)` below to make it
 * hot-reloadable in development.
 */
class ColdSourcegraphWebApp extends React.Component<SourcegraphWebAppProps, SourcegraphWebAppState> {
    private readonly subscriptions = new Subscription()
    private readonly userRepositoriesUpdates = new Subject<void>()
    private readonly darkThemeMediaList = window.matchMedia('(prefers-color-scheme: dark)')
    private readonly platformContext: PlatformContext = createPlatformContext()
    private readonly extensionsController: ExtensionsController = createExtensionsController(this.platformContext)

    constructor(props: SourcegraphWebAppProps) {
        super(props)
        this.subscriptions.add(this.extensionsController)

        const parsedSearchURL = parseSearchURL(window.location.search)
        // The patternType in the URL query parameter. If none is provided, default to literal.
        // This will be updated with the default in settings when the web app mounts.
        const urlPatternType = parsedSearchURL.patternType || SearchPatternType.literal
        const urlCase = parsedSearchURL.caseSensitive
        const availableVersionContexts = window.context.experimentalFeatures.versionContexts
        const previousVersionContext = localStorage.getItem(LAST_VERSION_CONTEXT_KEY)
        const resolvedVersionContext = availableVersionContexts
            ? resolveVersionContext(parsedSearchURL.versionContext || undefined, availableVersionContexts) ||
              resolveVersionContext(previousVersionContext || undefined, availableVersionContexts) ||
              undefined
            : undefined

        this.state = {
            themePreference: readStoredThemePreference(),
            systemIsLightTheme: !this.darkThemeMediaList.matches,
            navbarSearchQueryState: { query: '' },
            settingsCascade: EMPTY_SETTINGS_CASCADE,
            viewerSubject: SITE_SUBJECT_NO_ADMIN,
            parsedSearchQuery: parsedSearchURL.query || '',
            searchPatternType: urlPatternType,
            searchCaseSensitivity: urlCase,
            copyQueryButton: false,
            versionContext: resolvedVersionContext,
            availableVersionContexts,
            previousVersionContext,
            showRepogroupHomepage: false,
            showOnboardingTour: false,
            showSearchContext: false,
            availableSearchContexts: [],
            selectedSearchContextSpec: 'global',
            defaultSearchContextSpec: 'global', // global is default for now, user will be able to change this at some point
            hasUserAddedRepositories: false,
            showEnterpriseHomePanels: false,
            globbing: false,
            showMultilineSearchConsole: false,
            showQueryBuilder: false,
            enableSmartQuery: false,
            enableCodeMonitoring: false,
        }
    }

    /** Returns whether Sourcegraph should be in light theme */
    private isLightTheme(): boolean {
        return this.state.themePreference === 'system'
            ? this.state.systemIsLightTheme
            : this.state.themePreference === 'light'
    }

    public componentDidMount(): void {
        updateUserSessionStores()

        document.documentElement.classList.add('theme')

        this.subscriptions.add(
            combineLatest([from(this.platformContext.settings), authenticatedUser.pipe(startWith(null))]).subscribe(
                ([settingsCascade, authenticatedUser]) => {
                    this.setState(state => ({
                        settingsCascade,
                        authenticatedUser,
                        ...experimentalFeaturesFromSettings(settingsCascade),
                        globbing: globbingEnabledFromSettings(settingsCascade),
                        searchPatternType: defaultPatternTypeFromSettings(settingsCascade) || state.searchPatternType,
                        viewerSubject: viewerSubjectFromSettings(settingsCascade, authenticatedUser),
                    }))
                },
                () => this.setState({ authenticatedUser: null })
            )
        )

        // Observe settings mutations for analytics
        this.subscriptions.add(
            from(this.platformContext.settings)
                .pipe(bufferCount(2, 1))
                .subscribe(([oldSettings, newSettings]) => {
                    logCodeInsightsChanges(oldSettings, newSettings, eventLogger)
                })
        )

        // React to OS theme change
        this.subscriptions.add(
            fromEvent<MediaQueryListEvent>(this.darkThemeMediaList, 'change').subscribe(event => {
                this.setState({ systemIsLightTheme: !event.matches })
            })
        )

        this.subscriptions.add(
            fetchSearchContexts.subscribe(contexts => {
                this.setState({ availableSearchContexts: contexts })
            })
        )

        this.subscriptions.add(
            combineLatest([this.userRepositoriesUpdates, authenticatedUser])
                .pipe(
                    switchMap(([, authenticatedUser]) =>
                        authenticatedUser ? listUserRepositories({ id: authenticatedUser.id, first: 1 }) : of(null)
                    )
                )
                .subscribe(userRepositories => {
                    const hasUserAddedRepositories = userRepositories !== null && userRepositories.nodes.length > 0
                    this.setState({ hasUserAddedRepositories })
                })
        )

        this.subscriptions.add(
            authenticatedUser.subscribe(authenticatedUser => {
                if (authenticatedUser === null) {
                    return
                }
                const previousSearchContextSpec = localStorage.getItem(LAST_SEARCH_CONTEXT_KEY)
                const context = `@${authenticatedUser.username}`
                this.setState({
                    defaultSearchContextSpec: context,
                    selectedSearchContextSpec: previousSearchContextSpec || context,
                })
            })
        )

        /**
         * Listens for uncaught 401 errors when a user when a user was previously authenticated.
         *
         * Don't subscribe to this event when there wasn't an authenticated user,
         * as it could lead to an infinite loop of 401 -> reload -> 401
         */
        this.subscriptions.add(
            authenticatedUser
                .pipe(
                    switchMap(authenticatedUser =>
                        authenticatedUser ? fromEvent<ErrorEvent>(window, 'error') : of(null)
                    )
                )
                .subscribe(event => {
                    if (event?.error instanceof HTTPStatusError && event.error.status === 401) {
                        location.reload()
                    }
                })
        )

        // Send initial versionContext to extensions
        this.extensionsController.services.workspace.versionContext.next(this.state.versionContext)

        this.userRepositoriesUpdates.next()
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
        document.documentElement.classList.remove('theme', 'theme-light', 'theme-dark')
    }

    public componentDidUpdate(): void {
        localStorage.setItem(LIGHT_THEME_LOCAL_STORAGE_KEY, this.state.themePreference)
        document.documentElement.classList.toggle('theme-light', this.isLightTheme())
        document.documentElement.classList.toggle('theme-dark', !this.isLightTheme())
    }

    public render(): React.ReactFragment | null {
        if (window.pageError && window.pageError.statusCode !== 404) {
            const statusCode = window.pageError.statusCode
            const statusText = window.pageError.statusText
            const errorMessage = window.pageError.error
            const errorID = window.pageError.errorID

            let subtitle: JSX.Element | undefined
            if (errorID) {
                subtitle = <FeedbackText headerText="Sorry, there's been a problem." />
            }
            if (errorMessage) {
                subtitle = (
                    <div className="app__error">
                        {subtitle}
                        {subtitle && <hr className="my-3" />}
                        <pre>{errorMessage}</pre>
                    </div>
                )
            } else {
                subtitle = <div className="app__error">{subtitle}</div>
            }
            return <HeroPage icon={ServerIcon} title={`${statusCode}: ${statusText}`} subtitle={subtitle} />
        }

        const { authenticatedUser } = this.state
        if (authenticatedUser === undefined) {
            return null
        }

        const { children, ...props } = this.props

        return (
            <ErrorBoundary location={null}>
                <ShortcutProvider>
                    <BrowserRouter key={0}>
                        {/* eslint-disable react/jsx-no-bind */}
                        <Route
                            path="/"
                            render={routeComponentProps => (
                                <LayoutWithActivation
                                    {...props}
                                    {...routeComponentProps}
                                    authenticatedUser={authenticatedUser}
                                    viewerSubject={this.state.viewerSubject}
                                    settingsCascade={this.state.settingsCascade}
                                    showBatchChanges={this.props.showBatchChanges}
                                    // Theme
                                    isLightTheme={this.isLightTheme()}
                                    themePreference={this.state.themePreference}
                                    onThemePreferenceChange={this.onThemePreferenceChange}
                                    // Search query
                                    navbarSearchQueryState={this.state.navbarSearchQueryState}
                                    onNavbarQueryChange={this.onNavbarQueryChange}
                                    fetchHighlightedFileLineRanges={fetchHighlightedFileLineRanges}
                                    searchRequest={search}
                                    parsedSearchQuery={this.state.parsedSearchQuery}
                                    setParsedSearchQuery={this.setParsedSearchQuery}
                                    patternType={this.state.searchPatternType}
                                    setPatternType={this.setPatternType}
                                    caseSensitive={this.state.searchCaseSensitivity}
                                    setCaseSensitivity={this.setCaseSensitivity}
                                    versionContext={this.state.versionContext}
                                    setVersionContext={this.setVersionContext}
                                    availableVersionContexts={this.state.availableVersionContexts}
                                    previousVersionContext={this.state.previousVersionContext}
                                    copyQueryButton={this.state.copyQueryButton}
                                    // Extensions
                                    platformContext={this.platformContext}
                                    extensionsController={this.extensionsController}
                                    telemetryService={eventLogger}
                                    isSourcegraphDotCom={window.context.sourcegraphDotComMode}
                                    showRepogroupHomepage={this.state.showRepogroupHomepage}
                                    showOnboardingTour={this.state.showOnboardingTour}
                                    showSearchContext={this.canShowSearchContext()}
                                    selectedSearchContextSpec={this.getSelectedSearchContextSpec()}
                                    setSelectedSearchContextSpec={this.setSelectedSearchContextSpec}
                                    availableSearchContexts={this.state.availableSearchContexts}
                                    defaultSearchContextSpec={this.state.defaultSearchContextSpec}
                                    showEnterpriseHomePanels={this.state.showEnterpriseHomePanels}
                                    globbing={this.state.globbing}
                                    showMultilineSearchConsole={this.state.showMultilineSearchConsole}
                                    showQueryBuilder={this.state.showQueryBuilder}
                                    enableSmartQuery={this.state.enableSmartQuery}
                                    enableCodeMonitoring={this.state.enableCodeMonitoring}
                                    fetchSavedSearches={fetchSavedSearches}
                                    fetchRecentSearches={fetchRecentSearches}
                                    fetchRecentFileViews={fetchRecentFileViews}
                                    createCodeMonitor={createCodeMonitor}
                                    fetchUserCodeMonitors={fetchUserCodeMonitors}
                                    fetchCodeMonitor={fetchCodeMonitor}
                                    updateCodeMonitor={updateCodeMonitor}
                                    deleteCodeMonitor={deleteCodeMonitor}
                                    toggleCodeMonitorEnabled={toggleCodeMonitorEnabled}
                                    streamSearch={aggregateStreamingSearch}
                                    onUserRepositoriesUpdate={this.onUserRepositoriesUpdate}
                                />
                            )}
                        />
                        {/* eslint-enable react/jsx-no-bind */}
                    </BrowserRouter>
                    <Tooltip key={1} />
                    <Notifications
                        key={2}
                        extensionsController={this.extensionsController}
                        notificationClassNames={notificationClassNames}
                    />
                </ShortcutProvider>
            </ErrorBoundary>
        )
    }

    private onThemePreferenceChange = (themePreference: ThemePreference): void => {
        this.setState({ themePreference })
    }

    private onNavbarQueryChange = (navbarSearchQueryState: QueryState): void => {
        this.setState({ navbarSearchQueryState })
    }

    private setParsedSearchQuery = (query: string): void => {
        this.setState({ parsedSearchQuery: query })
    }

    private setPatternType = (patternType: SearchPatternType): void => {
        this.setState({
            searchPatternType: patternType,
        })
    }

    private setCaseSensitivity = (caseSensitive: boolean): void => {
        this.setState({
            searchCaseSensitivity: caseSensitive,
        })
    }

    private setVersionContext = (versionContext: string | undefined): void => {
        const resolvedVersionContext = resolveVersionContext(versionContext, this.state.availableVersionContexts)
        if (!resolvedVersionContext) {
            localStorage.removeItem(LAST_VERSION_CONTEXT_KEY)
            this.setState({ versionContext: undefined, previousVersionContext: null })
        } else {
            localStorage.setItem(LAST_VERSION_CONTEXT_KEY, resolvedVersionContext)
            this.setState({ versionContext: resolvedVersionContext, previousVersionContext: resolvedVersionContext })
        }

        this.extensionsController.services.workspace.versionContext.next(resolvedVersionContext)
    }

    private onUserRepositoriesUpdate = (userRepoCount: number): void => {
        this.setState({ hasUserAddedRepositories: userRepoCount > 0 })
    }

    private canShowSearchContext = (): boolean => this.state.showSearchContext && this.state.hasUserAddedRepositories

    private getSelectedSearchContextSpec = (): string | undefined =>
        this.canShowSearchContext() ? this.state.selectedSearchContextSpec : undefined

    private setSelectedSearchContextSpec = (spec: string): void => {
        const { availableSearchContexts, defaultSearchContextSpec } = this.state
        const resolvedSearchContextSpec = resolveSearchContextSpec(
            spec,
            availableSearchContexts,
            defaultSearchContextSpec
        )
        this.setState({ selectedSearchContextSpec: resolvedSearchContextSpec })
        localStorage.setItem(LAST_SEARCH_CONTEXT_KEY, resolvedSearchContextSpec)
    }
}

export const SourcegraphWebApp = hot(ColdSourcegraphWebApp)
