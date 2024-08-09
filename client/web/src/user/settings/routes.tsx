import type { FC } from 'react'

import { Navigate } from 'react-router-dom'

import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { Text } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { canWriteBatchChanges } from '../../batches/utils'
import type { ExecutorsUserAreaProps } from '../../enterprise/executors/ExecutorsUserArea'
import type { UserEventLogsPageProps } from '../../enterprise/user/settings/UserEventLogsPage'
import type { UserSettingsAreaUserFields } from '../../graphql-operations'
import { SiteAdminAlert } from '../../site-admin/SiteAdminAlert'

import type { UserSettingsAreaRoute, UserSettingsAreaRouteContext } from './UserSettingsArea'

const ExecutorsUserArea = lazyComponent<ExecutorsUserAreaProps, 'ExecutorsUserArea'>(
    () => import('../../enterprise/executors/ExecutorsUserArea'),
    'ExecutorsUserArea'
)

const SettingsArea = lazyComponent(() => import('../../settings/SettingsArea'), 'SettingsArea')

const UserSettingsSecurityPage = lazyComponent(
    () => import('./auth/UserSettingsSecurityPage'),
    'UserSettingsSecurityPage'
)

const shouldRenderBatchChangesPage = ({
    batchChangesEnabled,
    user: { viewerCanAdminister },
    authenticatedUser,
}: UserSettingsAreaRouteContext): boolean =>
    batchChangesEnabled && viewerCanAdminister && canWriteBatchChanges(authenticatedUser)

export const userSettingsAreaRoutes: readonly UserSettingsAreaRoute[] = [
    {
        path: '',
        render: props => <UserSettingAreaIndexPage {...props} />,
    },
    {
        path: 'profile',
        render: lazyComponent(() => import('./profile/UserSettingsProfilePage'), 'UserSettingsProfilePage'),
    },
    {
        path: 'password',
        render: () => <Navigate to="../security" replace={true} />,
    },
    {
        path: 'quota',
        render: lazyComponent(() => import('./quota/UserQuotaProfilePage'), 'UserQuotaProfilePage'),
        condition: ({ authenticatedUser }) => authenticatedUser.siteAdmin,
    },
    {
        path: 'emails',
        render: lazyComponent(() => import('./emails/UserSettingsEmailsPage'), 'UserSettingsEmailsPage'),
    },
    {
        path: 'tokens/*',
        render: lazyComponent(() => import('./accessTokens/UserSettingsTokensArea'), 'UserSettingsTokensArea'),
        condition: () => window.context.accessTokensAllow !== 'none',
    },
    // future GA Cloud routes
    {
        path: 'security',
        render: props => <UserSettingsSecurityPage {...props} context={window.context} />,
    },
    {
        path: 'product-research',
        render: lazyComponent(() => import('./research/ProductResearch'), 'ProductResearchPage'),
        condition: () => window.context.productResearchPageEnabled,
    },
    {
        path: 'permissions',
        render: lazyComponent(
            () => import('../../enterprise/user/settings/auth/UserSettingsPermissionsPage'),
            'UserSettingsPermissionsPage'
        ),
    },
    {
        path: 'event-log',
        render: lazyComponent<UserEventLogsPageProps, 'UserEventLogsPage'>(
            () => import('../../enterprise/user/settings/UserEventLogsPage'),
            'UserEventLogsPage'
        ),
    },
    {
        path: 'executors/*',
        render: props => (
            <ExecutorsUserArea
                {...props}
                telemetryRecorder={props.platformContext.telemetryRecorder}
                namespaceID={props.user.id}
            />
        ),
        condition: shouldRenderBatchChangesPage,
    },
    {
        path: 'batch-changes',
        render: lazyComponent(
            () => import('../../enterprise/batches/settings/BatchChangesSettingsArea'),
            'BatchChangesSettingsArea'
        ),
        condition: shouldRenderBatchChangesPage,
    },
    {
        path: 'github-apps/*',
        render: lazyComponent(() => import('./UserGitHubAppsArea'), 'UserGitHubAppsArea'),
        // GitHub Apps are currently only relevant for users who use them with batch changes. If they are used for other things too, you can remove this condition.
        condition: shouldRenderBatchChangesPage,
    },
]

interface UserSettingAreaIndexPageProps extends PlatformContextProps, SettingsCascadeProps, TelemetryProps {
    isSourcegraphDotCom: boolean
    authenticatedUser: AuthenticatedUser
    user: UserSettingsAreaUserFields
    extraHeader?: JSX.Element
    className?: string
}

const UserSettingAreaIndexPage: FC<UserSettingAreaIndexPageProps> = props => {
    const { isSourcegraphDotCom, authenticatedUser, user } = props
    const isLightTheme = useIsLightTheme()

    if (isSourcegraphDotCom && authenticatedUser && user.id !== authenticatedUser.id) {
        return (
            <SiteAdminAlert className="sidebar__alert" variant="danger">
                Only the user may access their individual settings.
            </SiteAdminAlert>
        )
    }
    return (
        <SettingsArea
            {...props}
            subject={props.user}
            isLightTheme={isLightTheme}
            extraHeader={
                <>
                    {authenticatedUser && user.id !== authenticatedUser.id && (
                        <SiteAdminAlert className="sidebar__alert">
                            Viewing settings for <strong>{user.username}</strong>
                        </SiteAdminAlert>
                    )}
                    <Text>User settings override global and organization settings.</Text>
                </>
            }
        />
    )
}
