import type { Meta, StoryFn } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import type { AuthenticatedUser } from '../../../auth'
import { WebStory } from '../../../components/WebStory'
import type { SourcegraphContext } from '../../../jscontext'

import { CodyMarketingPage } from './CodyMarketingPage'

const config: Meta = {
    title: 'web/src/cody/CodyMarketingPage',
}

export default config

const context: Pick<SourcegraphContext, 'externalURL'> = {
    externalURL: 'https://sourcegraph.test:3443',
}

export const SourcegraphDotCom: StoryFn = () => (
    <WebStory>
        {() => (
            <CodyMarketingPage
                context={context}
                isSourcegraphDotCom={true}
                authenticatedUser={null}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        )}
    </WebStory>
)
export const Enterprise: StoryFn = () => (
    <WebStory>
        {() => (
            <CodyMarketingPage
                context={context}
                isSourcegraphDotCom={false}
                authenticatedUser={null}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        )}
    </WebStory>
)

export const EnterpriseSiteAdmin: StoryFn = () => (
    <WebStory>
        {() => (
            <CodyMarketingPage
                context={context}
                isSourcegraphDotCom={false}
                authenticatedUser={{ siteAdmin: true } as AuthenticatedUser}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        )}
    </WebStory>
)
