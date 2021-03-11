import MapSearchIcon from 'mdi-react/MapSearchIcon'
import React, { useLayoutEffect, useRef } from 'react'
import { Route, RouteComponentProps, Switch, useLocation } from 'react-router'
import { ActivationProps } from '../../../shared/src/components/activation/Activation'
import * as GQL from '../../../shared/src/graphql/schema'
import { PlatformContextProps } from '../../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../../shared/src/settings/settings'
import { withAuthenticatedUser } from '../auth/withAuthenticatedUser'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { HeroPage } from '../components/HeroPage'
import { RouteDescriptor } from '../util/contributions'
import { SiteAdminSidebar, SiteAdminSideBarGroups } from './SiteAdminSidebar'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { TelemetryProps } from '../../../shared/src/telemetry/telemetryService'
import { AuthenticatedUser } from '../auth'
import { PageHeader } from '../components/PageHeader'
import { Page } from '../components/Page'

const NotFoundPage: React.ComponentType<{}> = () => (
    <HeroPage
        icon={MapSearchIcon}
        title="404: Not Found"
        subtitle="Sorry, the requested site admin page was not found."
    />
)

const NotSiteAdminPage: React.ComponentType<{}> = () => (
    <HeroPage icon={MapSearchIcon} title="403: Forbidden" subtitle="Only site admins are allowed here." />
)

export interface SiteAdminAreaRouteContext
    extends PlatformContextProps,
        SettingsCascadeProps,
        ActivationProps,
        TelemetryProps {
    site: Pick<GQL.ISite, '__typename' | 'id'>
    authenticatedUser: AuthenticatedUser
    isLightTheme: boolean

    /** This property is only used by {@link SiteAdminOverviewPage}. */
    overviewComponents: readonly React.ComponentType[]
}

export interface SiteAdminAreaRoute extends RouteDescriptor<SiteAdminAreaRouteContext> {}

interface SiteAdminAreaProps
    extends RouteComponentProps<{}>,
        PlatformContextProps,
        SettingsCascadeProps,
        ActivationProps,
        TelemetryProps {
    routes: readonly SiteAdminAreaRoute[]
    sideBarGroups: SiteAdminSideBarGroups
    overviewComponents: readonly React.ComponentType[]
    authenticatedUser: AuthenticatedUser
    isLightTheme: boolean
}

const AuthenticatedSiteAdminArea: React.FunctionComponent<SiteAdminAreaProps> = props => {
    const { pathname } = useLocation()

    const reference = useRef<HTMLDivElement>(null)

    useLayoutEffect(() => {
        if (reference.current) {
            reference.current.scrollIntoView()
        }
    }, [pathname])

    // If not site admin, redirect to sign in.
    if (!props.authenticatedUser.siteAdmin) {
        return <NotSiteAdminPage />
    }

    const context: SiteAdminAreaRouteContext = {
        authenticatedUser: props.authenticatedUser,
        platformContext: props.platformContext,
        settingsCascade: props.settingsCascade,
        isLightTheme: props.isLightTheme,
        activation: props.activation,
        site: { __typename: 'Site' as const, id: window.context.siteGQLID },
        overviewComponents: props.overviewComponents,
        telemetryService: props.telemetryService,
    }

    return (
        <Page>
            <PageHeader path={[{ text: 'Site Admin' }]} />
            <div className="site-admin-area d-flex my-3" ref={reference}>
                <SiteAdminSidebar className="sidebar flex-0 mr-3" groups={props.sideBarGroups} />
                <div className="flex-bounded">
                    <ErrorBoundary location={props.location}>
                        <React.Suspense fallback={<LoadingSpinner className="icon-inline m-2" />}>
                            <Switch>
                                {props.routes.map(
                                    /* eslint-disable react/jsx-no-bind */
                                    ({ render, path, exact, condition = () => true }) => (
                                        <Route
                                            // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                                            key="hardcoded-key"
                                            path={props.match.url + path}
                                            exact={exact}
                                            render={routeComponentProps =>
                                                render({ ...context, ...routeComponentProps })
                                            }
                                        />
                                    )
                                    /* eslint-enable react/jsx-no-bind */
                                )}
                                <Route component={NotFoundPage} />
                            </Switch>
                        </React.Suspense>
                    </ErrorBoundary>
                </div>
            </div>
        </Page>
    )
}

/**
 * Renders a layout of a sidebar and a content area to display site admin information.
 */
export const SiteAdminArea = withAuthenticatedUser(AuthenticatedSiteAdminArea)
