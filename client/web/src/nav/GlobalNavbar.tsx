import * as H from 'history'
import React, { useEffect } from 'react'
import { ActivationProps } from '../../../shared/src/components/activation/Activation'
import { ExtensionsControllerProps } from '../../../shared/src/extensions/controller'
import { PlatformContextProps } from '../../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../../shared/src/settings/settings'
import { AuthenticatedUser } from '../auth'
import {
    PatternTypeProps,
    CaseSensitivityProps,
    CopyQueryButtonProps,
    OnboardingTourProps,
    ParsedSearchQueryProps,
    SearchContextProps,
} from '../search'
import { SearchNavbarItem } from '../search/input/SearchNavbarItem'
import { showDotComMarketing } from '../util/features'
import { NavLinks } from './NavLinks'
import { ThemeProps } from '../../../shared/src/theme'
import { ThemePreferenceProps } from '../theme'
import { KeyboardShortcutsProps } from '../keyboardShortcuts/keyboardShortcuts'
import { QueryState } from '../search/helpers'
import { Link } from '../../../shared/src/components/Link'
import { VersionContextDropdown } from './VersionContextDropdown'
import { VersionContextProps } from '../../../shared/src/search/util'
import { VersionContext } from '../schema/site.schema'
import { TelemetryProps } from '../../../shared/src/telemetry/telemetryService'
import { BrandLogo } from '../components/branding/BrandLogo'
import { LinkOrSpan } from '../../../shared/src/components/LinkOrSpan'
import { ExtensionAlertAnimationProps } from './UserNavItem'
import { LayoutRouteProps } from '../routes'

interface Props
    extends SettingsCascadeProps,
        PlatformContextProps,
        ExtensionsControllerProps,
        KeyboardShortcutsProps,
        TelemetryProps,
        ThemeProps,
        ThemePreferenceProps,
        ExtensionAlertAnimationProps,
        ActivationProps,
        Pick<ParsedSearchQueryProps, 'parsedSearchQuery'>,
        PatternTypeProps,
        CaseSensitivityProps,
        CopyQueryButtonProps,
        VersionContextProps,
        SearchContextProps,
        OnboardingTourProps {
    history: H.History
    location: H.Location<{ query: string }>
    authenticatedUser: AuthenticatedUser | null
    authRequired: boolean
    navbarSearchQueryState: QueryState
    onNavbarQueryChange: (queryState: QueryState) => void
    isSourcegraphDotCom: boolean
    isSearchRelatedPage: boolean
    showBatchChanges: boolean
    routes: readonly LayoutRouteProps<{}>[]

    // Whether globbing is enabled for filters.
    globbing: boolean

    // Whether to additionally highlight or provide hovers for tokens, e.g., regexp character sets.
    enableSmartQuery: boolean

    /**
     * Which variation of the global navbar to render.
     *
     * 'low-profile' renders the the navbar with no border or background. Used on the search
     * homepage.
     *
     * 'low-profile-with-logo' renders the low-profile navbar but with the homepage logo. Used on repogroup pages.
     */
    variant: 'default' | 'low-profile' | 'low-profile-with-logo' | 'no-search-input'

    setVersionContext: (versionContext: string | undefined) => void
    availableVersionContexts: VersionContext[] | undefined

    minimalNavLinks?: boolean
    branding?: typeof window.context.branding

    /** For testing only. Used because reactstrap's Popover is incompatible with react-test-renderer. */
    hideNavLinks: boolean
}

export const GlobalNavbar: React.FunctionComponent<Props> = ({
    authRequired,
    isSearchRelatedPage,
    navbarSearchQueryState,
    versionContext,
    setVersionContext,
    availableVersionContexts,
    caseSensitive,
    patternType,
    onNavbarQueryChange,
    hideNavLinks,
    variant,
    isLightTheme,
    branding,
    location,
    history,
    minimalNavLinks,
    ...props
}) => {
    // Workaround: can't put this in optional parameter value because of https://github.com/babel/babel/issues/11166
    branding = branding ?? window.context?.branding

    const query = props.parsedSearchQuery

    useEffect(() => {
        // On a non-search related page or non-repo page, we clear the query in
        // the main query input to avoid misleading users
        // that the query is relevant in any way on those pages.
        if (!isSearchRelatedPage) {
            onNavbarQueryChange({ query: '' })
            return
        }
        // Do nothing if there is no query in the URL
        if (!query) {
            return
        }
        onNavbarQueryChange({ query })
    }, [isSearchRelatedPage, onNavbarQueryChange, query])

    const logo = (
        <LinkOrSpan to={authRequired ? undefined : '/search'} className="global-navbar__logo-link">
            <BrandLogo
                branding={branding}
                isLightTheme={isLightTheme}
                variant="symbol"
                className="global-navbar__logo"
            />
        </LinkOrSpan>
    )
    const navLinks = !authRequired && !hideNavLinks && (
        <NavLinks
            showDotComMarketing={showDotComMarketing}
            minimalNavLinks={minimalNavLinks}
            location={location}
            history={history}
            isLightTheme={isLightTheme}
            {...props}
        />
    )

    return (
        <div
            className={`global-navbar ${
                variant === 'low-profile' || variant === 'low-profile-with-logo'
                    ? ''
                    : 'global-navbar--bg border-bottom'
            } py-1`}
        >
            {variant === 'low-profile' || variant === 'low-profile-with-logo' ? (
                <>
                    {variant === 'low-profile-with-logo' && logo}
                    <div className="flex-1" />
                    {navLinks}
                </>
            ) : variant === 'no-search-input' ? (
                <>
                    {logo}
                    <div className="nav-item flex-1">
                        <Link to="/search" className="nav-link">
                            Search
                        </Link>
                    </div>
                    {navLinks}
                </>
            ) : (
                <>
                    {logo}
                    {authRequired ? (
                        <div className="flex-1" />
                    ) : (
                        <div className="global-navbar__search-box-container d-none d-sm-flex flex-row">
                            <VersionContextDropdown
                                history={history}
                                navbarSearchQuery={navbarSearchQueryState.query}
                                caseSensitive={caseSensitive}
                                patternType={patternType}
                                versionContext={versionContext}
                                setVersionContext={setVersionContext}
                                availableVersionContexts={availableVersionContexts}
                                selectedSearchContextSpec={props.selectedSearchContextSpec}
                            />
                            <SearchNavbarItem
                                {...props}
                                navbarSearchState={navbarSearchQueryState}
                                onChange={onNavbarQueryChange}
                                location={location}
                                history={history}
                                versionContext={versionContext}
                                isLightTheme={isLightTheme}
                                patternType={patternType}
                                caseSensitive={caseSensitive}
                            />
                        </div>
                    )}
                    {navLinks}
                </>
            )}
        </div>
    )
}
