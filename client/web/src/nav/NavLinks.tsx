import React from 'react'
import * as H from 'history'
import { ContributableMenu } from '../../../shared/src/api/protocol'
import { ActivationProps } from '../../../shared/src/components/activation/Activation'
import { ActivationDropdown } from '../../../shared/src/components/activation/ActivationDropdown'
import { Link } from '../../../shared/src/components/Link'
import { ExtensionsControllerProps } from '../../../shared/src/extensions/controller'
import { PlatformContextProps } from '../../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../../shared/src/settings/settings'
import { TelemetryProps } from '../../../shared/src/telemetry/telemetryService'
import { ThemeProps } from '../../../shared/src/theme'
import { isErrorLike } from '../../../shared/src/util/errors'
import { AuthenticatedUser } from '../auth'
import { WebActionsNavItems, WebCommandListPopoverButton } from '../components/shared'
import { BatchChangesNavItem } from '../enterprise/batches/global/nav/BatchChangesNavItem'
import { CodeMonitoringNavItem } from '../enterprise/code-monitoring/CodeMonitoringNavItem'
import { InsightsNavItem } from '../insights/InsightsNavLink'
import {
    KeyboardShortcutsProps,
    KEYBOARD_SHORTCUT_SHOW_COMMAND_PALETTE,
    KEYBOARD_SHORTCUT_SWITCH_THEME,
} from '../keyboardShortcuts/keyboardShortcuts'
import { Settings } from '../schema/settings.schema'
import { ThemePreferenceProps } from '../theme'
import { MenuNavItem } from './MenuNavItem'
import { StatusMessagesNavItem } from './StatusMessagesNavItem'
import { ExtensionAlertAnimationProps, UserNavItem } from './UserNavItem'
import { FeedbackPrompt } from './Feedback/FeedbackPrompt'
import { LayoutRouteProps } from '../routes'
import { getReactElements } from '../util/getReactElements'
import { LinkWithIcon } from '../components/LinkWithIcon'

interface Props
    extends SettingsCascadeProps<Settings>,
        KeyboardShortcutsProps,
        ExtensionsControllerProps<'executeCommand' | 'services'>,
        PlatformContextProps<'forceUpdateTooltip' | 'settings' | 'sourcegraphURL'>,
        ThemeProps,
        ThemePreferenceProps,
        ExtensionAlertAnimationProps,
        TelemetryProps,
        ActivationProps {
    location: H.Location
    history: H.History
    authenticatedUser: AuthenticatedUser | null
    showDotComMarketing: boolean
    showBatchChanges: boolean
    isSourcegraphDotCom: boolean
    minimalNavLinks?: boolean
    routes: readonly LayoutRouteProps<{}>[]
}

const getAnonymousUserNavItems = (props: Props): JSX.Element[] => {
    const { showDotComMarketing } = props

    // TODO:
    // It's not possible to move this constants to the upper scope because then "No Link component set" error is thrown.
    // We should allow such usage without throwing errors -> `setLinkComponent` should be called earlier.
    const DOCUMENTATION_LINK = (
        <LinkWithIcon
            key="documentationLink"
            hasIconPlaceholder={true}
            to="/help"
            className="nav-link nav-link--with-icon-placeholder btn btn-link text-decoration-none"
            target="_blank"
            rel="noopener"
            text="Docs"
        />
    )

    const ABOUT_LINK = (
        <LinkWithIcon
            key="aboutLink"
            hasIconPlaceholder={true}
            to="https://about.sourcegraph.com"
            className="nav-link nav-link--with-icon-placeholder btn btn-link text-decoration-none"
            target="_blank"
            rel="noopener"
            text="About"
        />
    )

    return getReactElements([DOCUMENTATION_LINK, showDotComMarketing && ABOUT_LINK])
}

const getMinimizableNavItems = (props: Props): JSX.Element[] => {
    const { showBatchChanges, settingsCascade } = props

    const settings = !isErrorLike(settingsCascade.final) ? settingsCascade.final : null
    const { codeMonitoring } = settings?.experimentalFeatures || {}
    const codeInsights =
        settings?.experimentalFeatures?.codeInsights && settings?.['insights.displayLocation.insightsPage'] !== false

    return getReactElements([
        codeInsights && <InsightsNavItem />,
        codeMonitoring && <CodeMonitoringNavItem />,
        showBatchChanges && <BatchChangesNavItem />,
    ])
}

export const NavLinks: React.FC<Props> = props => {
    const {
        settingsCascade,
        location,
        activation,
        history,
        minimalNavLinks,
        showDotComMarketing,
        authenticatedUser,
        routes,
    } = props

    const minimizableNavItems = getMinimizableNavItems(props)
    const anonymousUserNavItems = getAnonymousUserNavItems(props)

    return (
        <ul className="nav-links nav align-items-center pl-2 pr-1">
            {/* Show "Search" link on small screens when GlobalNavbar hides the SearchNavbarItem. */}
            {location.pathname !== '/search' && (
                <li className="nav-item d-sm-none">
                    <Link className="nav-link" to="/search">
                        Search
                    </Link>
                </li>
            )}
            <WebActionsNavItems {...props} menu={ContributableMenu.GlobalNav} />
            {activation && (
                <li className="nav-item">
                    <ActivationDropdown activation={activation} history={history} />
                </li>
            )}
            {!minimalNavLinks && (
                <>
                    {React.Children.map(minimizableNavItems, item => (
                        <li className="nav-item d-none d-lg-block">{item}</li>
                    ))}
                    <li className="nav-item nav-item--dropdown d-lg-none">
                        <MenuNavItem>
                            {minimizableNavItems}
                            {!authenticatedUser && anonymousUserNavItems}
                        </MenuNavItem>
                    </li>
                </>
            )}
            {authenticatedUser && (
                <li className="nav-item">
                    <FeedbackPrompt history={history} routes={routes} />
                </li>
            )}
            {!authenticatedUser &&
                React.Children.map(anonymousUserNavItems, link => (
                    <li key={link.key} className="nav-item d-none d-lg-block">
                        {link}
                    </li>
                ))}
            {/* show status messages if authenticated user is admin or opted-in with a user tag  */}
            {(authenticatedUser?.siteAdmin || authenticatedUser?.tags?.includes('AllowUserExternalServicePublic')) && (
                <li className="nav-item">
                    <StatusMessagesNavItem isSiteAdmin={authenticatedUser.siteAdmin} history={history} />
                </li>
            )}
            {!minimalNavLinks && (
                <li className="nav-item">
                    <WebCommandListPopoverButton
                        {...props}
                        buttonClassName="nav-link btn btn-link"
                        menu={ContributableMenu.CommandPalette}
                        keyboardShortcutForShow={KEYBOARD_SHORTCUT_SHOW_COMMAND_PALETTE}
                    />
                </li>
            )}
            {authenticatedUser && (
                <li className="nav-item">
                    <UserNavItem
                        {...props}
                        authenticatedUser={authenticatedUser}
                        showDotComMarketing={showDotComMarketing}
                        codeHostIntegrationMessaging={
                            (!isErrorLike(settingsCascade.final) &&
                                settingsCascade.final?.['alerts.codeHostIntegrationMessaging']) ||
                            'browser-extension'
                        }
                        keyboardShortcutForSwitchTheme={KEYBOARD_SHORTCUT_SWITCH_THEME}
                    />
                </li>
            )}
            {!authenticatedUser && (
                <>
                    <li className="nav-item mx-1">
                        <Link className="nav-link btn btn-secondary" to="/sign-in">
                            Log in
                        </Link>
                    </li>
                    <li className="nav-item mx-1">
                        <Link className="nav-link btn btn-primary" to="/sign-up">
                            Sign up
                        </Link>
                    </li>
                </>
            )}
        </ul>
    )
}
