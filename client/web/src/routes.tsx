import { useEffect } from 'react'

import { Navigate, useNavigate, type RouteObject } from 'react-router-dom'

import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import { communitySearchContextsRoutes } from './communitySearchContexts/routes'
import { type LegacyLayoutRouteContext, LegacyRoute } from './LegacyRouteContext'
import { PageRoutes } from './routes.constants'
import { isSearchJobsEnabled } from './search-jobs/utility'

const SiteAdminArea = lazyComponent(() => import('./site-admin/SiteAdminArea'), 'SiteAdminArea')
const SearchConsolePage = lazyComponent(() => import('./search/SearchConsolePage'), 'SearchConsolePage')
const SignInPage = lazyComponent(() => import('./auth/SignInPage'), 'SignInPage')
const RequestAccessPage = lazyComponent(() => import('./auth/RequestAccessPage'), 'RequestAccessPage')
const SignUpPage = lazyComponent(() => import('./auth/SignUpPage'), 'SignUpPage')
const UnlockAccountPage = lazyComponent(() => import('./auth/UnlockAccount'), 'UnlockAccountPage')
const SiteInitPage = lazyComponent(() => import('./site-admin/init/SiteInitPage'), 'SiteInitPage')
const RedirectToUserSettings = lazyComponent(
    () => import('./user/settings/RedirectToUserSettings'),
    'RedirectToUserSettings'
)
const RedirectToUserPage = lazyComponent(() => import('./user/settings/RedirectToUserPage'), 'RedirectToUserPage')
const OrgsArea = lazyComponent(() => import('./org/OrgsArea'), 'OrgsArea')
const ResetPasswordPage = lazyComponent(() => import('./auth/ResetPasswordPage'), 'ResetPasswordPage')
const ApiConsole = lazyComponent(() => import('./api/ApiConsole'), 'ApiConsole')
const UserArea = lazyComponent(() => import('./user/area/UserArea'), 'UserArea')
const SurveyPage = lazyComponent(() => import('./marketing/page/SurveyPage'), 'SurveyPage')
const RepoContainer = lazyComponent(() => import('./repo/RepoContainer'), 'RepoContainer')
const TeamsArea = lazyComponent(() => import('./team/TeamsArea'), 'TeamsArea')
const CodySidebarStoreProvider = lazyComponent(() => import('./cody/sidebar/Provider'), 'CodySidebarStoreProvider')
const GetCodyPage = lazyComponent(() => import('./get-cody/GetCodyPage'), 'GetCodyPage')
const PostSignUpPage = lazyComponent(() => import('./auth/PostSignUpPage'), 'PostSignUpPage')

const GlobalNotebooksArea = lazyComponent(() => import('./notebooks/GlobalNotebooksArea'), 'GlobalNotebooksArea')
const GlobalBatchChangesArea = lazyComponent(
    () => import('./enterprise/batches/global/GlobalBatchChangesArea'),
    'GlobalBatchChangesArea'
)
const GlobalCodeMonitoringArea = lazyComponent(
    () => import('./enterprise/code-monitoring/global/GlobalCodeMonitoringArea'),
    'GlobalCodeMonitoringArea'
)
const CodeInsightsRouter = lazyComponent(() => import('./enterprise/insights/CodeInsightsRouter'), 'CodeInsightsRouter')
const SearchContextsListPage = lazyComponent(
    () => import('./enterprise/searchContexts/SearchContextsListPage'),
    'SearchContextsListPage'
)
const CreateSearchContextPage = lazyComponent(
    () => import('./enterprise/searchContexts/CreateSearchContextPage'),
    'CreateSearchContextPage'
)
const EditSearchContextPage = lazyComponent(
    () => import('./enterprise/searchContexts/EditSearchContextPage'),
    'EditSearchContextPage'
)
const SearchContextPage = lazyComponent(
    () => import('./enterprise/searchContexts/SearchContextPage'),
    'SearchContextPage'
)
const SearchUpsellPage = lazyComponent(() => import('./search/upsell/SearchUpsellPage'), 'SearchUpsellPage')
const SearchPageWrapper = lazyComponent(() => import('./search/SearchPageWrapper'), 'SearchPageWrapper')
const CodySearchPage = lazyComponent(() => import('./cody/search/CodySearchPage'), 'CodySearchPage')
const CodyChatPage = lazyComponent(() => import('./cody/chat/CodyChatPage'), 'CodyChatPage')
const CodyManagementPage = lazyComponent(() => import('./cody/management/CodyManagementPage'), 'CodyManagementPage')
const CodySubscriptionPage = lazyComponent(
    () => import('./cody/subscription/CodySubscriptionPage'),
    'CodySubscriptionPage'
)
const CodyUpsellPage = lazyComponent(() => import('./cody/upsell/CodyUpsellPage'), 'CodyUpsellPage')
const CodyDashboardPage = lazyComponent(() => import('./cody/dashboard/CodyDashboardPage'), 'CodyDashboardPage')
const SearchJob = lazyComponent(() => import('./enterprise/search-jobs/SearchJobsPage'), 'SearchJobsPage')

const Index = lazyComponent(() => import('./Index'), 'IndexPage')

// Force a hard reload so that we delegate to the serverside HTTP handler for a route.
const PassThroughToServer: React.FC = () => {
    useEffect(() => {
        window.location.reload()
    })
    return null
}

/**
 * Holds all top-level routes for the app because both the navbar and the main content area need to
 * switch over matched path.
 *
 * See https://reacttraining.com/react-router/web/example/sidebar
 */
export const routes: RouteObject[] = [
    {
        path: PageRoutes.GetCody,
        element: <LegacyRoute render={props => <GetCodyPage {...props} context={window.context} />} />,
    },
    {
        path: PageRoutes.PostSignUp,
        element: <LegacyRoute render={() => <PostSignUpPage />} />,
    },
    {
        path: PageRoutes.Index,
        element: <Index />,
    },
    {
        path: PageRoutes.SignIn,
        element: (
            <LegacyRoute
                render={props => (
                    <SignInPage
                        {...props}
                        context={window.context}
                        telemetryRecorder={props.platformContext.telemetryRecorder}
                    />
                )}
            />
        ),
    },
    {
        path: PageRoutes.RequestAccess,
        element: (
            <LegacyRoute
                render={props => <RequestAccessPage telemetryRecorder={props.platformContext.telemetryRecorder} />}
            />
        ),
    },
    {
        path: PageRoutes.SignUp,
        element: (
            <LegacyRoute
                render={props => (
                    <SignUpPage
                        {...props}
                        context={window.context}
                        telemetryRecorder={props.platformContext.telemetryRecorder}
                    />
                )}
            />
        ),
    },
    {
        path: PageRoutes.UnlockAccount,
        element: <LegacyRoute render={props => <UnlockAccountPage {...props} context={window.context} />} />,
    },
    {
        path: PageRoutes.BatchChanges,
        element: (
            <LegacyRoute
                render={props => (
                    <GlobalBatchChangesArea {...props} telemetryRecorder={props.platformContext.telemetryRecorder} />
                )}
                // We also render this route on sourcegraph.com as a precaution in case anyone
                // follows an in-app link to /batch-changes from sourcegraph.com; the component
                // will just redirect the visitor to the marketing page
                condition={({ batchChangesEnabled, isSourcegraphDotCom }) => batchChangesEnabled || isSourcegraphDotCom}
            />
        ),
    },
    {
        path: PageRoutes.CodeMonitoring,
        element: (
            <LegacyRoute
                render={props => <GlobalCodeMonitoringArea {...props} />}
                condition={({ isSourcegraphDotCom, licenseFeatures }) =>
                    !isSourcegraphDotCom && licenseFeatures.isCodeSearchEnabled
                }
            />
        ),
    },
    {
        path: PageRoutes.Insights,
        element: (
            <LegacyRoute
                render={props => <CodeInsightsRouter {...props} />}
                condition={({ codeInsightsEnabled }) => !!codeInsightsEnabled}
            />
        ),
    },
    {
        path: PageRoutes.SearchJobs,
        element: (
            <LegacyRoute
                render={props => (
                    <SearchJob
                        isAdmin={props.authenticatedUser?.siteAdmin ?? false}
                        telemetryService={props.telemetryService}
                    />
                )}
                condition={isSearchJobsEnabled}
            />
        ),
    },
    {
        path: PageRoutes.Contexts,
        element: (
            <LegacyRoute
                render={props => <SearchContextsListPage {...props} />}
                condition={({ licenseFeatures }) => licenseFeatures.isCodeSearchEnabled}
            />
        ),
    },
    {
        path: PageRoutes.CreateContext,
        element: (
            <LegacyRoute
                render={props => <CreateSearchContextPage {...props} />}
                condition={({ licenseFeatures }) => licenseFeatures.isCodeSearchEnabled}
            />
        ),
    },
    {
        path: PageRoutes.EditContext,
        element: (
            <LegacyRoute
                render={props => <EditSearchContextPage {...props} />}
                condition={({ licenseFeatures }) => licenseFeatures.isCodeSearchEnabled}
            />
        ),
    },
    {
        path: PageRoutes.Context,
        element: (
            <LegacyRoute
                render={props => <SearchContextPage {...props} />}
                condition={({ licenseFeatures }) => licenseFeatures.isCodeSearchEnabled}
            />
        ),
    },
    {
        path: PageRoutes.SearchNotebook,
        element: <Navigate to={PageRoutes.Notebooks} replace={true} />,
    },
    {
        path: PageRoutes.Notebooks + '/*',
        element: (
            <LegacyRoute
                render={props => <GlobalNotebooksArea {...props} />}
                condition={({ licenseFeatures }) => licenseFeatures.isCodeSearchEnabled}
            />
        ),
    },
    {
        path: PageRoutes.SearchConsole,
        element: (
            <LegacyRoute
                render={props => <SearchConsolePageOrRedirect {...props} />}
                condition={({ licenseFeatures }) => licenseFeatures.isCodeSearchEnabled}
            />
        ),
    },
    {
        path: PageRoutes.Welcome,
        // This route is deprecated after we removed the post-sign-up page experimental feature, but we keep it for now to not break links.
        element: <Navigate replace={true} to={PageRoutes.Search} />,
    },
    {
        path: PageRoutes.Settings,
        element: <LegacyRoute render={props => <RedirectToUserSettings {...props} />} />,
    },
    {
        path: PageRoutes.User,
        element: <LegacyRoute render={props => <RedirectToUserPage {...props} />} />,
    },
    {
        path: PageRoutes.Teams,
        element: <LegacyRoute render={props => <TeamsArea {...props} />} />,
    },
    {
        path: PageRoutes.Organizations,
        element: <LegacyRoute render={props => <OrgsArea {...props} />} />,
    },
    {
        path: PageRoutes.SiteAdminInit,
        element: <LegacyRoute render={props => <SiteInitPage {...props} context={window.context} />} />,
    },
    {
        path: PageRoutes.SiteAdmin,
        element: (
            <LegacyRoute
                render={props => (
                    <SiteAdminArea
                        {...props}
                        routes={props.siteAdminAreaRoutes}
                        sideBarGroups={props.siteAdminSideBarGroups}
                        overviewComponents={props.siteAdminOverviewComponents}
                        codeInsightsEnabled={window.context.codeInsightsEnabled}
                        telemetryRecorder={props.platformContext.telemetryRecorder}
                    />
                )}
            />
        ),
    },
    {
        path: PageRoutes.PasswordReset,
        element: (
            <LegacyRoute
                render={props => (
                    <ResetPasswordPage
                        {...props}
                        context={window.context}
                        telemetryRecorder={props.platformContext.telemetryRecorder}
                    />
                )}
            />
        ),
    },
    {
        path: PageRoutes.ApiConsole,
        element: (
            <LegacyRoute render={props => <ApiConsole telemetryRecorder={props.platformContext.telemetryRecorder} />} />
        ),
    },
    {
        path: PageRoutes.Search,
        element: <LegacyRoute render={props => <SearchPageOrUpsellPage {...props} />} />,
    },
    {
        path: PageRoutes.UserArea,
        element: <LegacyRoute render={props => <UserArea {...props} />} />,
    },
    {
        path: PageRoutes.Survey,
        element: <LegacyRoute render={props => <SurveyPage {...props} />} />,
    },
    {
        path: PageRoutes.Help,
        element: <PassThroughToServer />,
    },
    {
        path: PageRoutes.Debug,
        element: <PassThroughToServer />,
    },
    {
        path: PageRoutes.CodySearch,
        element: (
            <LegacyRoute
                render={props => (
                    <CodySearchPage {...props} telemetryRecorder={props.platformContext.telemetryRecorder} />
                )}
                condition={({ licenseFeatures }) => licenseFeatures.isCodyEnabled}
            />
        ),
    },
    // TODO: [TEMPORARY] remove this redirect route when the marketing page is added.
    {
        path: `${PageRoutes.Cody}/*`,
        element: (
            <LegacyRoute
                render={() => {
                    const chatID = window.location.pathname.split('/').pop()
                    const navigate = useNavigate()

                    useEffect(() => {
                        navigate(`/cody/chat/${chatID}`)
                    }, [navigate, chatID])

                    return <div />
                }}
                condition={({ licenseFeatures }) =>
                    !window.location.pathname.startsWith('/cody/chat') && licenseFeatures.isCodyEnabled
                }
            />
        ),
    },
    {
        path: PageRoutes.CodyChat + '/*',
        element: (
            <LegacyRoute
                render={props => (
                    <CodyChatPage
                        {...props}
                        context={window.context}
                        telemetryRecorder={props.platformContext.telemetryRecorder}
                    />
                )}
                condition={({ licenseFeatures }) => licenseFeatures.isCodyEnabled}
            />
        ),
    },
    {
        path: PageRoutes.CodyManagement,
        element: (
            <LegacyRoute
                render={props => (
                    <CodyManagementPage {...props} telemetryRecorder={props.platformContext.telemetryRecorder} />
                )}
                condition={({ licenseFeatures }) => licenseFeatures.isCodyEnabled}
            />
        ),
    },
    {
        path: PageRoutes.CodySubscription,
        element: (
            <LegacyRoute
                render={props => (
                    <CodySubscriptionPage {...props} telemetryRecorder={props.platformContext.telemetryRecorder} />
                )}
                condition={({ licenseFeatures }) => licenseFeatures.isCodyEnabled}
            />
        ),
    },
    ...communitySearchContextsRoutes,
    {
        path: PageRoutes.Cody,
        element: <LegacyRoute render={props => <CodyDashboardOrUpsellPage {...props} />} />,
    },
    // this should be the last route to be regustered because it's a catch all route
    // when the instance has the code search feature.
    {
        path: PageRoutes.RepoContainer,
        element: (
            <LegacyRoute
                render={props => (
                    <CodySidebarStoreProvider
                        authenticatedUser={props.authenticatedUser}
                        telemetryRecorder={props.platformContext.telemetryRecorder}
                    >
                        <RepoContainer {...props} />
                    </CodySidebarStoreProvider>
                )}
                condition={({ licenseFeatures }) => licenseFeatures.isCodeSearchEnabled}
            />
        ),
        // In RR6, the useMatches hook will only give you the location that is matched
        // by the path rule and not the path rule instead. Since we need to be able to
        // detect if we're inside the repo container reliably inside the Layout, we
        // expose this information in the handle object instead.
        handle: { isRepoContainer: true },
    },
]

function SearchConsolePageOrRedirect(props: LegacyLayoutRouteContext): JSX.Element {
    const showMultilineSearchConsole = useExperimentalFeatures(features => features.showMultilineSearchConsole)

    return showMultilineSearchConsole ? (
        <SearchConsolePage {...props} />
    ) : (
        <Navigate replace={true} to={PageRoutes.Search} />
    )
}

function SearchPageOrUpsellPage(props: LegacyLayoutRouteContext): JSX.Element {
    const { isCodeSearchEnabled } = props.licenseFeatures
    if (!isCodeSearchEnabled) {
        return <SearchUpsellPage />
    }
    return <SearchPageWrapper {...props} />
}

function CodyDashboardOrUpsellPage(props: LegacyLayoutRouteContext): JSX.Element {
    const { isCodyEnabled } = props.licenseFeatures
    if (!isCodyEnabled) {
        return <CodyUpsellPage />
    }
    return <CodyDashboardPage {...props} telemetryRecorder={props.platformContext.telemetryRecorder} />
}
