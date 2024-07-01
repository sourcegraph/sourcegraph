import { Suspense, useMemo, type FC } from 'react'

import { Route, Routes, useParams } from 'react-router-dom'

import { gql, useQuery } from '@sourcegraph/http-client'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { LoadingSpinner } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import type { BatchChangesProps } from '../../batches'
import type { BreadcrumbSetters, BreadcrumbsProps } from '../../components/Breadcrumbs'
import { RouteError } from '../../components/ErrorBoundary'
import { NotFoundPage } from '../../components/HeroPage'
import { Page } from '../../components/Page'
import type {
    UserAreaUserFields,
    UserAreaUserProfileResult,
    UserAreaUserProfileVariables,
} from '../../graphql-operations'
import type { RouteV6Descriptor } from '../../util/contributions'
import { isAccessTokenCallbackPage } from '../settings/accessTokens/UserSettingsCreateAccessTokenCallbackPage'
import type { UserSettingsAreaRoute } from '../settings/UserSettingsArea'
import type { UserSettingsSidebarItems } from '../settings/UserSettingsSidebar'

import { UserAreaHeader, type UserAreaHeaderNavItem } from './UserAreaHeader'

/**
 * GraphQL fragment for the User fields needed by UserArea.
 *
 * These fields must be publicly available on the User, as there are features
 * that require ordinary users to access pages within the user area for another
 * user, most notably Batch Changes. User area components that wish to access
 * privileged fields must do so in another query, as is done in
 * UserSettingsArea.
 */
export const UserAreaGQLFragment = gql`
    fragment UserAreaUserFields on User {
        __typename
        id
        username
        displayName
        url
        settingsURL
        avatarURL
        viewerCanAdminister
        builtinAuth
        createdAt
        emails @skip(if: $isSourcegraphDotCom) {
            email
            isPrimary
        }
        roles @skip(if: $isSourcegraphDotCom) {
            nodes {
                name
                system
            }
        }
    }
`

export const USER_AREA_USER_PROFILE = gql`
    query UserAreaUserProfile($username: String!, $isSourcegraphDotCom: Boolean!) {
        user(username: $username) {
            ...UserAreaUserFields
        }
    }
    ${UserAreaGQLFragment}
`

export interface UserAreaRoute extends RouteV6Descriptor<UserAreaRouteContext> {
    /** When true, the header is not rendered and the component is not wrapped in a container. */
    fullPage?: boolean
}

interface UserAreaProps
    extends PlatformContextProps,
        SettingsCascadeProps,
        TelemetryProps,
        BreadcrumbsProps,
        BreadcrumbSetters,
        BatchChangesProps {
    userAreaRoutes: readonly UserAreaRoute[]
    userAreaHeaderNavItems: readonly UserAreaHeaderNavItem[]
    userSettingsSideBarItems: UserSettingsSidebarItems
    userSettingsAreaRoutes: readonly UserSettingsAreaRoute[]

    /**
     * The currently authenticated user, NOT the user whose username is specified in the URL's "username" route
     * parameter.
     */
    authenticatedUser: AuthenticatedUser | null

    isSourcegraphDotCom: boolean
}

/**
 * Properties passed to all page components in the user area.
 */
export interface UserAreaRouteContext
    extends PlatformContextProps,
        SettingsCascadeProps,
        TelemetryProps,
        BreadcrumbsProps,
        BreadcrumbSetters,
        BatchChangesProps {
    /** The user area main URL. */
    url: string

    /**
     * The user who is the subject of the page.
     */
    user: UserAreaUserFields

    /**
     * The currently authenticated user, NOT (necessarily) the user who is the subject of the page.
     *
     * For example, if Alice is viewing a user area page about Bob, then the authenticatedUser is Alice and the
     * user is Bob.
     */
    authenticatedUser: AuthenticatedUser | null
    userSettingsSideBarItems: UserSettingsSidebarItems
    userSettingsAreaRoutes: readonly UserSettingsAreaRoute[]

    isSourcegraphDotCom: boolean
}

/**
 * A user's public profile area.
 */
export const UserArea: FC<UserAreaProps> = ({ useBreadcrumb, userAreaRoutes, isSourcegraphDotCom, ...props }) => {
    const { username } = useParams()
    const userAreaMainUrl = `/users/${username}`

    const { data, error, loading, previousData } = useQuery<UserAreaUserProfileResult, UserAreaUserProfileVariables>(
        USER_AREA_USER_PROFILE,
        {
            variables: { username: username!, isSourcegraphDotCom },
        }
    )

    const childBreadcrumbSetters = useBreadcrumb(
        useMemo(
            () =>
                data?.user
                    ? {
                          key: 'UserArea',
                          link: { to: data.user.url, label: data.user.username },
                      }
                    : null,
            [data]
        )
    )

    // Accept stale data if recently updated, avoids unmounting components due to a brief lack of data
    const user = data?.user ?? previousData?.user

    if (loading && !user) {
        return (
            <div className="w-100 text-center">
                <LoadingSpinner className="m-2" />
            </div>
        )
    }

    if (error) {
        throw new Error(error.message)
    }

    if (!user) {
        return <NotFoundPage pageType="user" />
    }

    // Since the access token callback page is rendered in a nested route, we can't use the
    // `fullPage` route prop to determine whether to render the header. Instead, we check
    // whether the current page is the access token callback page.
    const isFullscreenPage = isAccessTokenCallbackPage()

    const context: UserAreaRouteContext = {
        ...props,
        url: userAreaMainUrl,
        user,
        namespace: user,
        ...childBreadcrumbSetters,
        isSourcegraphDotCom,
        telemetryRecorder: props.platformContext.telemetryRecorder,
    }

    return (
        <Suspense
            fallback={
                <div className="w-100 text-center">
                    <LoadingSpinner className="m-2" />
                </div>
            }
        >
            <Routes>
                {userAreaRoutes.map(
                    ({ path, render, condition = () => true, fullPage }) =>
                        condition(context) && (
                            <Route
                                errorElement={<RouteError />}
                                element={
                                    fullPage || isFullscreenPage ? (
                                        render(context)
                                    ) : (
                                        <Page>
                                            <UserAreaHeader
                                                {...props}
                                                {...context}
                                                className="mb-3"
                                                navItems={props.userAreaHeaderNavItems}
                                            />
                                            <div className="container">{render(context)}</div>
                                        </Page>
                                    )
                                }
                                path={path}
                                key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                            />
                        )
                )}
                <Route path="*" element={<NotFoundPage pageType="user" />} />
            </Routes>
        </Suspense>
    )
}
