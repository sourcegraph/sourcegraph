import * as React from 'react'
import { Redirect, RouteComponentProps } from 'react-router'
import { APIConsole } from './api/APIConsole'
import { ResetPasswordPage } from './auth/ResetPasswordPage'
import { SignInPage } from './auth/SignInPage'
import { SignUpPage } from './auth/SignUpPage'
import { DiscussionsPage } from './discussions/DiscussionsPage'
import { DocSitePage } from './docSite/DocSitePage'
import { ExploreArea } from './explore/ExploreArea'
import { ExtensionsArea } from './extensions/ExtensionsArea'
import { LayoutProps } from './Layout'
import { SurveyPage } from './marketing/SurveyPage'
import { OpenPage } from './open/OpenPage'
import { OrgsArea } from './org/OrgsArea'
import { RepoContainer } from './repo/RepoContainer'
import { parseSearchURLQuery } from './search'
import { MainPage } from './search/input/MainPage'
import { ScopePage } from './search/input/ScopePage'
import { SearchPage } from './search/input/SearchPage'
import { SearchResults } from './search/results/SearchResults'
import { SavedQueriesPage } from './search/saved-queries/SavedQueries'
import { SiteAdminArea } from './site-admin/SiteAdminArea'
import { SiteInitPage } from './site-admin/SiteInitPage'
import { RedirectToUserPage } from './user/account/RedirectToUserPage'
import { RedirectToUserSettings } from './user/account/RedirectToUserSettings'
import { UserArea } from './user/area/UserArea'

export interface LayoutRouteComponentProps extends RouteComponentProps<any>, LayoutProps {}

export interface LayoutRouteProps {
    path: string
    exact?: boolean
    render: (props: LayoutRouteComponentProps) => React.ReactNode

    /**
     * Whether or not to force the width of the page to be narrow.
     */
    forceNarrowWidth?: boolean
}

/**
 * Holds properties for repository+ routes.
 */
export const repoRevRoute: LayoutRouteProps = {
    path: '/:repoRevAndRest+',
    render: props => <RepoContainer {...props} />,
}

/**
 * Holds all top-level routes for the app because both the navbar and the main content area need to
 * switch over matched path.
 *
 * See https://reacttraining.com/react-router/web/example/sidebar
 */
export const routes: ReadonlyArray<LayoutRouteProps> = [
    {
        path: '/',
        render: (props: any) =>
            window.context.sourcegraphDotComMode && !props.user ? <Redirect to="/start" /> : <Redirect to="/search" />,
        exact: true,
    },
    {
        path: '/start',
        render: (props: any) => <MainPage {...props} />,
        exact: true,
    },
    {
        path: '/search',
        render: (props: any) =>
            parseSearchURLQuery(props.location.search) ? <SearchResults {...props} /> : <SearchPage {...props} />,
        exact: true,
    },
    {
        path: '/search/searches',
        render: props => <SavedQueriesPage {...props} />,
        exact: true,
        forceNarrowWidth: true,
    },
    {
        path: '/open',
        render: props => <OpenPage {...props} />,
        exact: true,
        forceNarrowWidth: true,
    },
    {
        path: '/sign-in',
        render: props => <SignInPage {...props} />,
        exact: true,
        forceNarrowWidth: true,
    },
    {
        path: '/sign-up',
        render: props => <SignUpPage {...props} />,
        exact: true,
        forceNarrowWidth: true,
    },
    {
        path: '/settings',
        render: props => <RedirectToUserSettings {...props} />,
    },
    {
        path: '/user',
        render: props => <RedirectToUserPage {...props} />,
    },
    {
        path: '/organizations',
        render: props => <OrgsArea {...props} />,
    },
    {
        path: '/search',
        render: props => <SearchResults {...props} />,
        exact: true,
    },
    {
        path: '/site-admin/init',
        exact: true,
        render: props => <SiteInitPage {...props} />,
        forceNarrowWidth: false,
    },
    {
        path: '/site-admin',
        render: props => (
            <SiteAdminArea
                {...props}
                routes={props.siteAdminAreaRoutes}
                sideBarGroups={props.siteAdminSideBarGroups}
                overviewComponents={props.siteAdminOverviewComponents}
            />
        ),
    },
    {
        path: '/password-reset',
        render: props => <ResetPasswordPage {...props} />,
        exact: true,
        forceNarrowWidth: true,
    },
    {
        path: '/explore',
        render: props => <ExploreArea {...props} />,
        exact: true,
    },
    {
        path: '/discussions',
        render: props => <DiscussionsPage {...props} />,
        exact: true,
    },
    {
        path: '/search/scope/:id',
        render: props => <ScopePage {...props} />,
        exact: true,
    },
    {
        path: '/api/console',
        render: props => <APIConsole {...props} />,
        exact: true,
    },
    {
        path: '/users/:username',
        render: props => <UserArea {...props} />,
    },
    {
        path: '/survey/:score?',
        render: props => <SurveyPage {...props} />,
    },
    {
        path: '/extensions',
        render: props => <ExtensionsArea {...props} routes={props.extensionsAreaRoutes} />,
    },
    {
        path: '/help',
        render: props => <DocSitePage {...props} path={props.location.pathname.slice(props.match.path.length + 1)} />,
    },
    repoRevRoute,
]
