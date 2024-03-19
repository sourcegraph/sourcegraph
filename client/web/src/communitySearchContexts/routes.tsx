import type { RouteObject } from 'react-router-dom'

import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import { LegacyRoute } from '../LegacyRouteContext'
import { CommunityPageRoutes } from '../routes.constants'

const KubernetesCommunitySearchContextPage = lazyComponent(
    () => import('./Kubernetes'),
    'KubernetesCommunitySearchContextPage'
)
const StackstormCommunitySearchContextPage = lazyComponent(
    () => import('./StackStorm'),
    'StackStormCommunitySearchContextPage'
)
const TemporalCommunitySearchContextPage = lazyComponent(
    () => import('./Temporal'),
    'TemporalCommunitySearchContextPage'
)
const O3deCommunitySearchContextPage = lazyComponent(() => import('./o3de'), 'O3deCommunitySearchContextPage')
const ChakraUICommunitySearchContextPage = lazyComponent(
    () => import('./chakraui'),
    'ChakraUICommunitySearchContextPage'
)
const StanfordCommunitySearchContextPage = lazyComponent(
    () => import('./Stanford'),
    'StanfordCommunitySearchContextPage'
)
const CncfCommunitySearchContextPage = lazyComponent(() => import('./cncf'), 'CncfCommunitySearchContextPage')
const JuliaCommunitySearchContextPage = lazyComponent(() => import('./Julia'), 'JuliaCommunitySearchContextPage')
const BackstageCommunitySearchContextPage = lazyComponent(
    () => import('./Backstage'),
    'BackstageCommunitySearchContextPage'
)

// Hack! Hardcode these routes into cmd/frontend/internal/app/ui/router.go
export const communitySearchContextsRoutes: readonly RouteObject[] = [
    {
        path: CommunityPageRoutes.Kubernetes,
        element: (
            <LegacyRoute
                render={props => <KubernetesCommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
    {
        path: CommunityPageRoutes.Stackstorm,
        element: (
            <LegacyRoute
                render={props => <StackstormCommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
    {
        path: CommunityPageRoutes.Temporal,
        element: (
            <LegacyRoute
                render={props => <TemporalCommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
    {
        path: CommunityPageRoutes.O3de,
        element: (
            <LegacyRoute
                render={props => <O3deCommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
    {
        path: CommunityPageRoutes.ChakraUI,
        element: (
            <LegacyRoute
                render={props => <ChakraUICommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
    {
        path: CommunityPageRoutes.Stanford,
        element: (
            <LegacyRoute
                render={props => <StanfordCommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
    {
        path: CommunityPageRoutes.Cncf,
        element: (
            <LegacyRoute
                render={props => <CncfCommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
    {
        path: CommunityPageRoutes.Julia,
        element: (
            <LegacyRoute
                render={props => <JuliaCommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
    {
        path: CommunityPageRoutes.Backstage,
        element: (
            <LegacyRoute
                render={props => <BackstageCommunitySearchContextPage {...props} />}
                condition={({ isSourcegraphDotCom }) => isSourcegraphDotCom}
            />
        ),
    },
]
