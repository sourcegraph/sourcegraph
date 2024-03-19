import {
    type FC,
    type MutableRefObject,
    type SetStateAction,
    useEffect,
    useLayoutEffect,
    useMemo,
    useRef,
    useState,
} from 'react'

import classNames from 'classnames'
import BarChartIcon from 'mdi-react/BarChartIcon'
import MagnifyIcon from 'mdi-react/MagnifyIcon'
import { type RouteObject, useLocation } from 'react-router-dom'
import useResizeObserver from 'use-resize-observer'

import { isMacPlatform } from '@sourcegraph/common'
import { shortcutDisplayName } from '@sourcegraph/shared/src/keyboardShortcuts'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { Settings } from '@sourcegraph/shared/src/schema/settings.schema'
import type { SearchContextInputProps } from '@sourcegraph/shared/src/search'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Button, ButtonLink, Link, ProductStatusBadge } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import type { BatchChangesProps } from '../batches'
import { BatchChangesNavItem } from '../batches/BatchChangesNavItem'
import type { CodeMonitoringProps } from '../codeMonitoring'
import { CodyLogo } from '../cody/components/CodyLogo'
import { BrandLogo } from '../components/branding/BrandLogo'
import { useFuzzyFinderFeatureFlags } from '../components/fuzzyFinder/FuzzyFinderFeatureFlag'
import { DeveloperSettingsGlobalNavItem } from '../devsettings/DeveloperSettingsGlobalNavItem'
import { useFeatureFlag, useKeywordSearch } from '../featureFlags/useFeatureFlag'
import { useRoutesMatch } from '../hooks'
import type { CodeInsightsProps } from '../insights/types'
import type { NotebookProps } from '../notebooks'
import { OnboardingChecklist } from '../onboarding'
import type { OwnConfigProps } from '../own/OwnConfigProps'
import { PageRoutes } from '../routes.constants'
import { isSearchJobsEnabled } from '../search-jobs/utility'
import { SearchNavbarItem } from '../search/input/SearchNavbarItem'
import { AccessRequestsGlobalNavItem } from '../site-admin/AccessRequestsPage/AccessRequestsGlobalNavItem'
import { useDeveloperSettings, useNavbarQueryState } from '../stores'
import { SvelteKitNavItem } from '../sveltekit/SvelteKitNavItem'
import { isCodyOnlyLicense, isCodeSearchOnlyLicense } from '../util/license'

import { NavAction, NavActions, NavBar, NavGroup, NavItem, NavLink } from '.'
import { NavDropdown, type NavDropdownItem } from './NavBar/NavDropdown'
import { StatusMessagesNavItem } from './StatusMessagesNavItem'
import { UserNavItem } from './UserNavItem'

import styles from './GlobalNavbar.module.scss'

export interface GlobalNavbarProps
    extends SettingsCascadeProps<Settings>,
        PlatformContextProps,
        TelemetryProps,
        SearchContextInputProps,
        CodeInsightsProps,
        BatchChangesProps,
        NotebookProps,
        CodeMonitoringProps,
        OwnConfigProps {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    showSearchBox: boolean
    routes: RouteObject[]

    isSearchAutoFocusRequired?: boolean
    isRepositoryRelatedPage?: boolean
    branding?: typeof window.context.branding
    showKeyboardShortcutsHelp: () => void
    showFeedbackModal: () => void

    setFuzzyFinderIsVisible: React.Dispatch<SetStateAction<boolean>>
}

/**
 * Calculates NavLink variant based whether current content fits into container or not.
 * @param containerReference a reference to navbar container
 */
function useCalculatedNavLinkVariant(containerReference: MutableRefObject<HTMLElement | null>): 'compact' | undefined {
    const { width = 0 } = useResizeObserver({ ref: containerReference })

    const [navLinkVariant, setNavLinkVariant] = useState<'compact'>()
    const [savedWindowWidth, setSavedWindowWidth] = useState<number>()

    useLayoutEffect(() => {
        const container = containerReference.current
        if (!container) {
            return
        }

        if (container.offsetWidth < container.scrollWidth) {
            setNavLinkVariant('compact')
            setSavedWindowWidth(width)
        } else if (savedWindowWidth && width > savedWindowWidth) {
            setNavLinkVariant(undefined)
        }
    }, [containerReference, savedWindowWidth, width])

    return navLinkVariant
}

function FuzzyFinderNavItem(setFuzzyFinderVisible: React.Dispatch<SetStateAction<boolean>>): JSX.Element {
    return (
        <NavAction className="d-none d-sm-flex">
            <Button
                onClick={() => setFuzzyFinderVisible(true)}
                className={classNames(styles.fuzzyFinderItem)}
                size="sm"
            >
                <span aria-hidden={true} aria-label={isMacPlatform() ? 'command-k' : 'ctrl-k'}>
                    {shortcutDisplayName('Mod+K')}
                </span>
            </Button>
        </NavAction>
    )
}

export const GlobalNavbar: React.FunctionComponent<React.PropsWithChildren<GlobalNavbarProps>> = ({
    showSearchBox,
    branding = window.context?.branding,
    isSourcegraphDotCom,
    isRepositoryRelatedPage,
    codeInsightsEnabled,
    searchContextsEnabled,
    codeMonitoringEnabled,
    notebooksEnabled,
    ownEnabled,
    showFeedbackModal,
    ...props
}) => {
    const location = useLocation()

    const routeMatch = useRoutesMatch(props.routes)

    const onNavbarQueryChange = useNavbarQueryState(state => state.setQueryState)
    const isLicensed = !!window.context?.licenseInfo
    const disableCodeSearchFeatures = isCodyOnlyLicense()
    // Search context management is still enabled on .com
    // but should not show in the navbar. Users can still
    // access this feature via the context dropdown.
    const showSearchContext = searchContextsEnabled && !isSourcegraphDotCom && !disableCodeSearchFeatures
    const showCodeMonitoring = codeMonitoringEnabled && !isSourcegraphDotCom && !disableCodeSearchFeatures
    const showSearchNotebook = notebooksEnabled && !isSourcegraphDotCom && !disableCodeSearchFeatures
    const showSearchJobs = isSearchJobsEnabled() && !disableCodeSearchFeatures
    const showBatchChanges =
        props.batchChangesEnabled && isLicensed && !isSourcegraphDotCom && !disableCodeSearchFeatures

    const [codySearchEnabled] = useFeatureFlag('cody-web-search')
    const [isAdminOnboardingEnabled] = useFeatureFlag('admin-onboarding')

    useEffect(() => {
        // On a non-search related page or non-repo page, we clear the query in
        // the main query input to avoid misleading users
        // that the query is relevant in any way on those pages.
        if (!showSearchBox) {
            onNavbarQueryChange({ query: '' })
            return
        }
    }, [showSearchBox, onNavbarQueryChange])

    const codeInsights = (codeInsightsEnabled && !isSourcegraphDotCom && !disableCodeSearchFeatures) ?? false

    const { fuzzyFinderNavbar } = useFuzzyFinderFeatureFlags()

    const isLightTheme = useIsLightTheme()

    const developerMode = useDeveloperSettings(settings => settings.enabled) || process.env.NODE_ENV === 'development'

    const showKeywordSearchToggle = useKeywordSearch()

    return (
        <>
            <NavBar
                logo={
                    <BrandLogo
                        branding={branding}
                        isLightTheme={isLightTheme}
                        variant="symbol"
                        className={styles.logo}
                    />
                }
            >
                <InlineNavigationPanel
                    authenticatedUser={props.authenticatedUser}
                    showSearchContext={showSearchContext}
                    showCodySearch={codySearchEnabled}
                    showSearchJobs={showSearchJobs}
                    showSearchNotebook={showSearchNotebook}
                    showCodeMonitoring={showCodeMonitoring}
                    showBatchChanges={showBatchChanges}
                    showCodeInsights={codeInsights}
                    routeMatch={routeMatch}
                    isSourcegraphDotCom={isSourcegraphDotCom}
                />

                <NavActions>
                    {developerMode && (
                        <NavAction>
                            <DeveloperSettingsGlobalNavItem />
                        </NavAction>
                    )}
                    <SvelteKitNavItem />
                    {props.authenticatedUser?.siteAdmin && (
                        <AccessRequestsGlobalNavItem className="d-flex align-items-center py-1" />
                    )}
                    {fuzzyFinderNavbar && FuzzyFinderNavItem(props.setFuzzyFinderIsVisible)}
                    {props.authenticatedUser?.siteAdmin && (
                        <>
                            {isAdminOnboardingEnabled && (
                                <NavAction>
                                    <OnboardingChecklist />
                                </NavAction>
                            )}
                            <NavAction>
                                <StatusMessagesNavItem />
                            </NavAction>
                        </>
                    )}
                    {!props.authenticatedUser ? (
                        <>
                            <NavAction>
                                <div>
                                    <Button
                                        className="mr-1"
                                        to={
                                            '/sign-in?returnTo=' +
                                            encodeURI(location.pathname + location.search + location.hash)
                                        }
                                        variant="secondary"
                                        outline={true}
                                        size="sm"
                                        as={Link}
                                    >
                                        Sign in
                                    </Button>
                                    {!isSourcegraphDotCom && window.context?.allowSignup && (
                                        <ButtonLink to="/sign-up" variant="primary" size="sm">
                                            Sign up
                                        </ButtonLink>
                                    )}
                                </div>
                            </NavAction>
                        </>
                    ) : (
                        <NavAction>
                            <UserNavItem
                                {...props}
                                authenticatedUser={props.authenticatedUser}
                                isSourcegraphDotCom={isSourcegraphDotCom}
                                showFeedbackModal={showFeedbackModal}
                            />
                        </NavAction>
                    )}
                </NavActions>
            </NavBar>
            {showSearchBox && (
                <div className={styles.searchNavBar}>
                    <SearchNavbarItem
                        {...props}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        searchContextsEnabled={searchContextsEnabled}
                        isRepositoryRelatedPage={isRepositoryRelatedPage}
                        showKeywordSearchToggle={showKeywordSearchToggle}
                    />
                </div>
            )}
        </>
    )
}

export interface InlineNavigationPanelProps {
    showSearchContext: boolean
    showCodySearch: boolean
    showSearchJobs: boolean
    showSearchNotebook: boolean
    showCodeMonitoring: boolean
    showBatchChanges: boolean
    showCodeInsights: boolean
    isSourcegraphDotCom: boolean
    authenticatedUser: AuthenticatedUser | null

    /** A current react router route match */
    routeMatch?: string
    className?: string
}

export const InlineNavigationPanel: FC<InlineNavigationPanelProps> = props => {
    const {
        showSearchContext,
        showCodySearch,
        showSearchJobs,
        showSearchNotebook,
        showBatchChanges,
        showCodeInsights,
        showCodeMonitoring,
        isSourcegraphDotCom,
        routeMatch,
        className,
    } = props

    const navbarReference = useRef<HTMLDivElement | null>(null)
    const navLinkVariant = useCalculatedNavLinkVariant(navbarReference)
    const disableCodyFeatures = isCodeSearchOnlyLicense()
    const disableCodeSearchFeatures = isCodyOnlyLicense()

    const searchNavBarItems = useMemo(() => {
        const items: (NavDropdownItem | false)[] = [
            showSearchContext && { path: PageRoutes.Contexts, content: 'Contexts' },
            showSearchNotebook && { path: PageRoutes.Notebooks, content: 'Notebooks' },
            // We hardcode the code monitoring path here because PageRoutes.CodeMonitoring is a catch-all
            // path for all code monitoring sub links.
            showCodeMonitoring && { path: '/code-monitoring', content: 'Monitoring' },
            showCodySearch && {
                path: PageRoutes.CodySearch,
                content: (
                    <>
                        Natural language search <ProductStatusBadge status="experimental" />
                    </>
                ),
            },
            showSearchJobs && {
                path: PageRoutes.SearchJobs,
                content: (
                    <>
                        Search Jobs <ProductStatusBadge className="ml-2" status="beta" />
                    </>
                ),
            },
        ]
        return items.filter<NavDropdownItem>((item): item is NavDropdownItem => !!item)
    }, [showSearchContext, showCodySearch, showSearchJobs, showCodeMonitoring, showSearchNotebook])

    const searchNavigation =
        searchNavBarItems.length > 0 ? (
            <NavDropdown
                key="search"
                toggleItem={{
                    path: PageRoutes.Search,
                    altPath: PageRoutes.RepoContainer,
                    icon: MagnifyIcon,
                    content: 'Code Search',
                    variant: navLinkVariant,
                }}
                routeMatch={routeMatch}
                homeItem={{ content: 'Search home' }}
                items={searchNavBarItems}
                name="search"
            />
        ) : (
            <NavItem icon={MagnifyIcon} key="search">
                <NavLink variant={navLinkVariant} to={PageRoutes.Search}>
                    Code Search
                </NavLink>
            </NavItem>
        )

    const CodyLogoWrapper = (): JSX.Element => <CodyLogo withColor={routeMatch === `${PageRoutes.Cody}/*`} />
    const hideCodyDropdown = disableCodyFeatures || !props.authenticatedUser
    const codyNavigation = hideCodyDropdown ? (
        <NavItem icon={() => <CodyLogoWrapper />} key="cody">
            <NavLink variant={navLinkVariant} to={disableCodyFeatures ? PageRoutes.Cody : PageRoutes.CodyChat}>
                Cody AI
            </NavLink>
        </NavItem>
    ) : (
        <NavDropdown
            key="cody"
            toggleItem={{
                path: isSourcegraphDotCom ? PageRoutes.CodyManagement : PageRoutes.Cody,
                icon: () => <CodyLogoWrapper />,
                content: 'Cody AI',
                variant: navLinkVariant,
            }}
            routeMatch={routeMatch}
            items={[
                {
                    path: isSourcegraphDotCom ? PageRoutes.CodyManagement : PageRoutes.Cody,
                    content: 'Dashboard',
                },
                {
                    path: PageRoutes.CodyChat,
                    content: 'Web Chat',
                },
            ]}
            name="cody"
        />
    )

    let prioritizedLinks: JSX.Element[] = [searchNavigation, codyNavigation]

    if (disableCodeSearchFeatures) {
        // This should be cheap considering there will only be two items in the array.
        prioritizedLinks = prioritizedLinks.reverse()
    }

    return (
        <NavGroup ref={navbarReference} className={classNames(className, styles.list)}>
            {prioritizedLinks}
            {showBatchChanges && <BatchChangesNavItem variant={navLinkVariant} />}
            {showCodeInsights && (
                <NavItem icon={BarChartIcon}>
                    <NavLink variant={navLinkVariant} to="/insights">
                        Insights
                    </NavLink>
                </NavItem>
            )}
            {isSourcegraphDotCom && (
                <NavItem>
                    <NavLink variant={navLinkVariant} to="https://sourcegraph.com" external={true}>
                        About Sourcegraph
                    </NavLink>
                </NavItem>
            )}
        </NavGroup>
    )
}
