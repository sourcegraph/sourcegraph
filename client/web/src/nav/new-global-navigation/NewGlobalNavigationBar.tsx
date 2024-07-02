import { useCallback, useState, type ComponentType, type FC, type PropsWithChildren } from 'react'

import { mdiClose, mdiMenu } from '@mdi/js'
import classNames from 'classnames'
import BarChartIcon from 'mdi-react/BarChartIcon'
import MagnifyIcon from 'mdi-react/MagnifyIcon'
import ToolsIcon from 'mdi-react/ToolsIcon'
import { NavLink, useLocation, useNavigate, useSearchParams, type RouteObject } from 'react-router-dom'
import shallow from 'zustand/shallow'

import { Toggles } from '@sourcegraph/branded/src/search-ui/input/toggles/Toggles'
import type { SearchQueryState, SubmitSearchParameters } from '@sourcegraph/shared/src/search'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Button, ButtonLink, Icon, Link, Modal, Text } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { BatchChangesIconNav } from '../../batches/icons'
import { CodyLogo } from '../../cody/components/CodyLogo'
import { BrandLogo } from '../../components/branding/BrandLogo'
import { DeveloperSettingsGlobalNavItem } from '../../devsettings/DeveloperSettingsGlobalNavItem'
import { useRoutesMatch } from '../../hooks'
import { PageRoutes } from '../../routes.constants'
import { isSearchJobsEnabled } from '../../search-jobs/utility'
import { LazyV2SearchInput } from '../../search/input/LazyV2SearchInput'
import { setSearchCaseSensitivity, setSearchMode, setSearchPatternType, useNavbarQueryState } from '../../stores'
import { InlineNavigationPanel, linkForCodyNavItem } from '../GlobalNavbar'
import { UserNavItem } from '../UserNavItem'

import styles from './NewGlobalNavigationBar.module.scss'

interface NewGlobalNavigationBar extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    notebooksEnabled: boolean
    searchContextsEnabled: boolean
    codeMonitoringEnabled: boolean
    batchChangesEnabled: boolean
    codeInsightsEnabled: boolean
    showSearchBox: boolean
    selectedSearchContextSpec?: string
    showFeedbackModal: () => void
    routes: RouteObject[]
}

/**
 * New experimental global navigation bar with inline search bar and
 * dynamic navigation items.
 */
export const NewGlobalNavigationBar: FC<
    NewGlobalNavigationBar & {
        __testing__initialSideMenuOpen?: boolean
    }
> = props => {
    const {
        isSourcegraphDotCom,
        notebooksEnabled,
        searchContextsEnabled,
        codeMonitoringEnabled,
        batchChangesEnabled,
        codeInsightsEnabled,
        authenticatedUser,
        selectedSearchContextSpec,
        showSearchBox,
        showFeedbackModal,
        telemetryService,
        telemetryRecorder,
        __testing__initialSideMenuOpen,
    } = props

    const isLightTheme = useIsLightTheme()
    const [params] = useSearchParams()
    const [isSideMenuOpen, setSideMenuOpen] = useState(__testing__initialSideMenuOpen ?? false)
    const routeMatch = useRoutesMatch(props.routes)

    // Features enablement flags and conditions
    const isLicensed = !!window.context?.licenseInfo
    const showSearchContext = searchContextsEnabled && !isSourcegraphDotCom
    const showSearchJobs = isSearchJobsEnabled()
    const showSearchNotebook = notebooksEnabled && !isSourcegraphDotCom
    const showCodeMonitoring = codeMonitoringEnabled && !isSourcegraphDotCom
    const showBatchChanges = batchChangesEnabled && isLicensed && !isSourcegraphDotCom
    const showCodeInsights = codeInsightsEnabled && !isSourcegraphDotCom
    // We only show the hamburger icon on a repo page and search results page
    const showHamburger =
        routeMatch === PageRoutes.RepoContainer || (routeMatch === PageRoutes.Search && params.get('q'))

    return (
        <>
            <nav aria-label="Main" className={classNames(styles.nav, { [styles.navWithoutMenu]: !showHamburger })}>
                {showHamburger && (
                    <Button
                        variant="secondary"
                        outline={true}
                        className={styles.menuButton}
                        onClick={() => setSideMenuOpen(true)}
                    >
                        <Icon svgPath={mdiMenu} aria-label="Navigation menu" />
                    </Button>
                )}

                <NavLink to={PageRoutes.Search}>
                    <BrandLogo variant="symbol" isLightTheme={isLightTheme} className={styles.logo} />
                </NavLink>

                {showSearchBox ? (
                    <NavigationSearchBox
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        authenticatedUser={authenticatedUser}
                        selectedSearchContextSpec={selectedSearchContextSpec}
                        telemetryService={telemetryService}
                        telemetryRecorder={telemetryRecorder}
                    />
                ) : (
                    <InlineNavigationPanel
                        showSearchContext={showSearchContext}
                        authenticatedUser={authenticatedUser}
                        showSearchJobs={showSearchJobs}
                        showSearchNotebook={showSearchNotebook}
                        showCodeMonitoring={showCodeMonitoring}
                        showBatchChanges={showBatchChanges}
                        showCodeInsights={showCodeInsights}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        className={styles.inlineNavigationList}
                        routeMatch={routeMatch}
                    />
                )}

                {authenticatedUser ? (
                    <UserNavItem
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        authenticatedUser={authenticatedUser}
                        showFeedbackModal={showFeedbackModal}
                        className="ml-auto"
                        showKeyboardShortcutsHelp={() => {}}
                        telemetryService={telemetryService}
                    />
                ) : (
                    <SignInUpButtons isSourcegraphDotCom={isSourcegraphDotCom} />
                )}
            </nav>

            {isSideMenuOpen && (
                <SidebarNavigation
                    showSearchContext={showSearchContext}
                    showSearchJobs={showSearchJobs}
                    showSearchNotebook={showSearchNotebook}
                    showCodeMonitoring={showCodeMonitoring}
                    showBatchChanges={showBatchChanges}
                    showCodeInsights={showCodeInsights}
                    isSourcegraphDotCom={isSourcegraphDotCom}
                    onClose={() => setSideMenuOpen(false)}
                />
            )}
        </>
    )
}

type NavigationSearchBoxState = Pick<
    SearchQueryState,
    | 'queryState'
    | 'setQueryState'
    | 'submitSearch'
    | 'searchCaseSensitivity'
    | 'searchPatternType'
    | 'defaultPatternType'
    | 'searchMode'
>

/**
 * Search query state selector to filter out only needed state fields from
 * global search query state store. (Re-render nav search box only whenever one
 * of these fields has been changed)
 */
const selectQueryState = (state: SearchQueryState): NavigationSearchBoxState => ({
    queryState: state.queryState,
    setQueryState: state.setQueryState,
    submitSearch: state.submitSearch,
    searchCaseSensitivity: state.searchCaseSensitivity,
    searchPatternType: state.searchPatternType,
    defaultPatternType: state.defaultPatternType,
    searchMode: state.searchMode,
})

interface NavigationSearchBoxProps extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    selectedSearchContextSpec?: string
}

/**
 * Compact version of search box UI, shows expanded version when the
 * search box gets focus.
 */
const NavigationSearchBox: FC<NavigationSearchBoxProps> = props => {
    const { authenticatedUser, isSourcegraphDotCom, selectedSearchContextSpec, telemetryService, telemetryRecorder } =
        props

    const navigate = useNavigate()
    const location = useLocation()

    const {
        searchMode,
        queryState,
        searchPatternType,
        defaultPatternType,
        searchCaseSensitivity,
        setQueryState,
        submitSearch,
    } = useNavbarQueryState(selectQueryState, shallow)

    const submitSearchOnChange = useCallback(
        (parameters: Partial<SubmitSearchParameters> = {}) => {
            submitSearch({
                location,
                source: 'nav',
                historyOrNavigate: navigate,
                selectedSearchContextSpec,
                telemetryRecorder,
                ...parameters,
            })
        },
        [submitSearch, navigate, location, selectedSearchContextSpec, telemetryRecorder]
    )

    // TODO: Move this check outside of navigation component and share it via context
    const structuralSearchDisabled = window.context?.experimentalFeatures?.structuralSearch !== 'enabled'

    return (
        <LazyV2SearchInput
            visualMode="compact"
            patternType={searchPatternType}
            interpretComments={false}
            queryState={queryState}
            submitSearch={submitSearchOnChange}
            isSourcegraphDotCom={isSourcegraphDotCom}
            authenticatedUser={authenticatedUser}
            selectedSearchContextSpec={selectedSearchContextSpec}
            telemetryService={telemetryService}
            telemetryRecorder={telemetryRecorder}
            className={styles.searchBar}
            onChange={setQueryState}
            onSubmit={submitSearchOnChange}
        >
            <Toggles
                searchMode={searchMode}
                patternType={searchPatternType}
                defaultPatternType={defaultPatternType}
                caseSensitive={searchCaseSensitivity}
                navbarSearchQuery={queryState.query}
                structuralSearchDisabled={structuralSearchDisabled}
                setPatternType={setSearchPatternType}
                setCaseSensitivity={setSearchCaseSensitivity}
                setSearchMode={setSearchMode}
                submitSearch={submitSearchOnChange}
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
            />
        </LazyV2SearchInput>
    )
}

interface SignInUpButtonsProps {
    isSourcegraphDotCom: boolean
}

const SignInUpButtons: FC<SignInUpButtonsProps> = props => {
    const { isSourcegraphDotCom } = props
    const location = useLocation()

    return (
        <div className={styles.signInButtons}>
            <Button
                as={Link}
                to={'/sign-in?returnTo=' + encodeURI(location.pathname + location.search + location.hash)}
                size="sm"
                variant="secondary"
                outline={true}
                className="mr-1"
            >
                Sign in
            </Button>
            {!isSourcegraphDotCom && window.context?.allowSignup && (
                <ButtonLink to="/sign-up" variant="primary" size="sm">
                    Sign up
                </ButtonLink>
            )}
        </div>
    )
}

interface SidebarNavigationProps {
    isSourcegraphDotCom: boolean
    showSearchContext: boolean
    showSearchJobs: boolean
    showSearchNotebook: boolean
    showCodeMonitoring: boolean
    showBatchChanges: boolean
    showCodeInsights: boolean
    onClose: () => void
}

const SidebarNavigation: FC<SidebarNavigationProps> = props => {
    const {
        showSearchContext,
        showSearchJobs,
        showSearchNotebook,
        showCodeMonitoring,
        showBatchChanges,
        showCodeInsights,
        isSourcegraphDotCom,
        onClose,
    } = props

    const isLightTheme = useIsLightTheme()

    const handleNavigationClick = (): void => {
        // Close the navigation modal/sidebar on any navigation transition
        // But leave it open in case of any other click (like developer link open event)
        onClose()
    }

    return (
        <Modal aria-label="Sidebar navigation" className={styles.sidebarNavigation} onDismiss={onClose}>
            <header className={styles.sidebarNavigationHeader}>
                <Button variant="secondary" outline={true} className={styles.menuButton} onClick={onClose}>
                    <Icon svgPath={mdiClose} aria-label="Close sidebar navigation" />
                </Button>
                <NavLink to={PageRoutes.Search} className={styles.sidebarNavigationLogoLink}>
                    <BrandLogo variant="logo" isLightTheme={isLightTheme} className={styles.sidebarNavigationLogo} />
                </NavLink>
            </header>

            <nav className={styles.sidebarNavigationNav}>
                <ul className={styles.sidebarNavigationList}>
                    <NavItemLink url={PageRoutes.Search} icon={MagnifyIcon} onClick={handleNavigationClick}>
                        Code Search
                    </NavItemLink>

                    {window.context?.codyEnabledOnInstance && (
                        <NavItemLink
                            url={linkForCodyNavItem(isSourcegraphDotCom)}
                            icon={CodyLogo}
                            onClick={handleNavigationClick}
                        >
                            Cody
                        </NavItemLink>
                    )}

                    {showBatchChanges && (
                        <NavItemLink url="/batch-changes" icon={BatchChangesIconNav} onClick={handleNavigationClick}>
                            Batch Changes
                        </NavItemLink>
                    )}

                    {showCodeInsights && (
                        <NavItemLink url="/insights" icon={BarChartIcon} onClick={handleNavigationClick}>
                            Insights
                        </NavItemLink>
                    )}

                    {(showSearchContext ||
                        showSearchNotebook ||
                        showCodeMonitoring ||
                        showBatchChanges ||
                        showCodeInsights) && (
                        <li className={classNames(styles.navItem, styles.navItemNested)}>
                            <span className={styles.navGroupTitle}>
                                <Icon as={ToolsIcon} className={styles.icon} aria-hidden={true} /> Tools
                            </span>

                            <ul
                                className={classNames(styles.sidebarNavigationList, styles.sidebarNavigationListNested)}
                            >
                                {showSearchContext && (
                                    <NavItemLink url={PageRoutes.Contexts} onClick={handleNavigationClick}>
                                        Contexts
                                    </NavItemLink>
                                )}
                                {showSearchNotebook && (
                                    <NavItemLink url={PageRoutes.Notebooks} onClick={handleNavigationClick}>
                                        Notebooks
                                    </NavItemLink>
                                )}
                                {showCodeMonitoring && (
                                    <NavItemLink url="/code-monitoring" onClick={handleNavigationClick}>
                                        Code Monitoring
                                    </NavItemLink>
                                )}
                                {showSearchJobs && (
                                    <NavItemLink url={PageRoutes.SearchJobs} onClick={handleNavigationClick}>
                                        Search Jobs
                                    </NavItemLink>
                                )}
                            </ul>
                        </li>
                    )}

                    {isSourcegraphDotCom && (
                        <NavItemLink url="https://sourcegraph.com" external={true} onClick={handleNavigationClick}>
                            About Sourcegraph
                        </NavItemLink>
                    )}
                </ul>
            </nav>

            <footer className={styles.footer}>
                {process.env.NODE_ENV === 'development' && (
                    <DeveloperSettingsGlobalNavItem className={styles.developerLink} />
                )}
                <Text className={styles.version}>Sourcegraph version: {window.context.version ?? 'unknown'}</Text>
            </footer>
        </Modal>
    )
}

interface NavItemLinkProps {
    url: string
    external?: boolean
    icon?: ComponentType<{ className?: string }>
    onClick?: () => void
}

const NavItemLink: FC<PropsWithChildren<NavItemLinkProps>> = props => {
    const { url, external, icon: IconComponent, children, onClick } = props

    return (
        <li className={styles.navItem}>
            <Button
                as={Link}
                to={url}
                rel={external ? 'noreferrer noopener' : undefined}
                target={external ? '_blank' : undefined}
                className={styles.navLink}
                onClick={onClick}
            >
                {IconComponent && <Icon as={IconComponent} className={styles.icon} aria-hidden={true} />} {children}
            </Button>
        </li>
    )
}
