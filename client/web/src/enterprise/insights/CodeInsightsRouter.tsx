import type { FC } from 'react'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { AuthenticatedUser } from '../../auth'

const CodeInsightsAppLazyRouter = lazyComponent(() => import('./CodeInsightsAppRouter'), 'CodeInsightsAppRouter')

const CodeInsightsDotComGetStartedLazy = lazyComponent(
    () => import('./pages/landing/dot-com-get-started/CodeInsightsDotComGetStarted'),
    'CodeInsightsDotComGetStarted'
)

export interface CodeInsightsRouterProps extends TelemetryProps, TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
}

export const CodeInsightsRouter: FC<CodeInsightsRouterProps> = props => {
    const { authenticatedUser, telemetryService, telemetryRecorder } = props

    if (!window.context?.codeInsightsEnabled) {
        return (
            <CodeInsightsDotComGetStartedLazy
                telemetryService={telemetryService}
                authenticatedUser={authenticatedUser}
                telemetryRecorder={telemetryRecorder}
            />
        )
    }

    return (
        <CodeInsightsAppLazyRouter
            authenticatedUser={authenticatedUser}
            telemetryService={telemetryService}
            telemetryRecorder={telemetryRecorder}
        />
    )
}
