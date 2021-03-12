import * as H from 'history'
import * as React from 'react'
import { Subscription } from 'rxjs'
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
import { CampaignsNavItem } from '../enterprise/campaigns/global/nav/CampaignsNavItem'
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

interface Props
    extends SettingsCascadeProps<Settings>,
        KeyboardShortcutsProps,
        ExtensionsControllerProps<'executeCommand' | 'extHostAPI'>,
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
    showCampaigns: boolean
    isSourcegraphDotCom: boolean
    minimalNavLinks?: boolean
    routes: readonly LayoutRouteProps<{}>[]
}

export class NavLinks extends React.PureComponent<Props> {
    private subscriptions = new Subscription()

    private navItems(): JSX.Element[] {
        const items = []
        if (!isErrorLike(this.props.settingsCascade.final)) {
            if (this.props.settingsCascade.final?.experimentalFeatures?.codeInsights) {
                items.push(<InsightsNavItem />)
            }
            if (this.props.settingsCascade.final?.experimentalFeatures?.codeMonitoring) {
                items.push(<CodeMonitoringNavItem />)
            }
        }
        return items
    }

    private items = this.navItems()

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <ul className="nav-links nav align-items-center pl-2 pr-1">
                {/* Show "Search" link on small screens when GlobalNavbar hides the SearchNavbarItem. */}
                {this.props.location.pathname !== '/search' && (
                    <li className="nav-item d-sm-none">
                        <Link className="nav-link" to="/search">
                            Search
                        </Link>
                    </li>
                )}
                <WebActionsNavItems {...this.props} menu={ContributableMenu.GlobalNav} />
                {this.props.activation && (
                    <li className="nav-item">
                        <ActivationDropdown activation={this.props.activation} history={this.props.history} />
                    </li>
                )}
                {!this.props.minimalNavLinks && (
                    <>
                        {React.Children.map(this.items, item => (
                            <li className="nav-item d-none d-lg-block">{item}</li>
                        ))}
                        {this.props.showCampaigns && (
                            <li className="nav-item d-none d-lg-block">
                                <CampaignsNavItem />
                            </li>
                        )}
                        <li className="nav-item d-lg-none">
                            <MenuNavItem>
                                {this.items}
                                {this.props.showCampaigns && <CampaignsNavItem />}
                            </MenuNavItem>
                        </li>
                    </>
                )}
                {this.props.authenticatedUser && (
                    <li className="nav-item">
                        <FeedbackPrompt history={this.props.history} routes={this.props.routes} />
                    </li>
                )}
                {!this.props.authenticatedUser && (
                    <>
                        {this.props.location.pathname !== '/sign-in' && (
                            <li className="nav-item mx-1">
                                <Link className="nav-link btn btn-primary" to="/sign-in">
                                    Sign in
                                </Link>
                            </li>
                        )}
                        <li className="nav-item">
                            <Link to="/help" className="nav-link" target="_blank" rel="noopener">
                                Docs
                            </Link>
                        </li>
                        {this.props.showDotComMarketing && (
                            <li className="nav-item">
                                <a
                                    href="https://about.sourcegraph.com"
                                    className="nav-link"
                                    target="_blank"
                                    rel="noopener"
                                >
                                    About
                                </a>
                            </li>
                        )}
                    </>
                )}

                {/* show status messages if authenticated user is admin or opted-in with a user tag  */}
                {(this.props.authenticatedUser?.siteAdmin ||
                    this.props.authenticatedUser?.tags?.includes('AllowUserExternalServicePublic')) && (
                    <li className="nav-item">
                        <StatusMessagesNavItem
                            isSiteAdmin={this.props.authenticatedUser.siteAdmin}
                            history={this.props.history}
                        />
                    </li>
                )}
                {!this.props.minimalNavLinks && (
                    <li className="nav-item">
                        <WebCommandListPopoverButton
                            {...this.props}
                            buttonClassName="nav-link btn btn-link"
                            menu={ContributableMenu.CommandPalette}
                            keyboardShortcutForShow={KEYBOARD_SHORTCUT_SHOW_COMMAND_PALETTE}
                        />
                    </li>
                )}
                {this.props.authenticatedUser && (
                    <li className="nav-item">
                        <UserNavItem
                            {...this.props}
                            authenticatedUser={this.props.authenticatedUser}
                            showDotComMarketing={this.props.showDotComMarketing}
                            codeHostIntegrationMessaging={
                                (!isErrorLike(this.props.settingsCascade.final) &&
                                    this.props.settingsCascade.final?.['alerts.codeHostIntegrationMessaging']) ||
                                'browser-extension'
                            }
                            keyboardShortcutForSwitchTheme={KEYBOARD_SHORTCUT_SWITCH_THEME}
                        />
                    </li>
                )}
            </ul>
        )
    }
}
