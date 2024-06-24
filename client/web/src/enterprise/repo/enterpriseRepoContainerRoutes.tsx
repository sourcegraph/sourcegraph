import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import { RedirectRoute } from '../../components/RedirectRoute'
import type { RepoContainerRoute } from '../../repo/RepoContainer'
import { repoContainerRoutes } from '../../repo/repoContainerRoutes'

const RepositoryCodeIntelArea = lazyComponent(
    () => import('../codeintel/repo/RepositoryCodeIntelArea'),
    'RepositoryCodeIntelArea'
)

const RepositoryBatchChangesArea = lazyComponent(
    () => import('../batches/repo/RepositoryBatchChangesArea'),
    'RepositoryBatchChangesArea'
)

const RepositoryOwnEditPage = lazyComponent(() => import('../own/RepositoryOwnEditPage'), 'RepositoryOwnEditPage')
const RepositoryOwnPage = lazyComponent(() => import('../own/RepositoryOwnPage'), 'RepositoryOwnPage')

export const enterpriseRepoContainerRoutes: readonly RepoContainerRoute[] = [
    ...repoContainerRoutes,

    {
        path: '/-/code-intelligence/*',
        render: () => (
            <RedirectRoute
                getRedirectURL={({ location }) => location.pathname.replace('/code-intelligence', '/code-graph')}
            />
        ),
    },
    {
        path: '/-/code-graph/*',
        render: context => (
            <RepositoryCodeIntelArea {...context} telemetryRecorder={context.platformContext.telemetryRecorder} />
        ),
    },
    {
        path: '/-/batch-changes',
        condition: ({ batchChangesEnabled }) => batchChangesEnabled,
        render: context => (
            <RepositoryBatchChangesArea {...context} telemetryRecorder={context.platformContext.telemetryRecorder} />
        ),
    },
    {
        path: '/-/own/*',
        condition: ({ isSourcegraphDotCom }) => !isSourcegraphDotCom,
        render: context => (
            <RepositoryOwnPage {...context} telemetryRecorder={context.platformContext.telemetryRecorder} />
        ),
    },
    {
        path: '/-/own/edit',
        condition: ({ isSourcegraphDotCom }) => !isSourcegraphDotCom,
        render: context => (
            <RepositoryOwnEditPage {...context} telemetryRecorder={context.platformContext.telemetryRecorder} />
        ),
    },
]
