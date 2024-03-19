import type { Decorator, Meta, StoryFn } from '@storybook/react'

import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { updateJSContextBatchChangesLicense } from '@sourcegraph/shared/src/testing/batches'

import type { AuthenticatedUser } from '../../auth'
import { WebStory } from '../../components/WebStory'
import type { GlobalNavbar, GlobalNavbarProps } from '../GlobalNavbar'

import { NewGlobalNavigationBar } from './NewGlobalNavigationBar'

const decorator: Decorator<GlobalNavbarProps> = Story => {
    updateJSContextBatchChangesLicense('full')

    return <WebStory>{() => <Story />}</WebStory>
}

const config: Meta<typeof GlobalNavbar> = {
    title: 'web/nav/GlobalNav',
    decorators: [decorator],
}

export default config

export const NewGlobalNavigationBarDemo: StoryFn = () => (
    <NewGlobalNavigationBar
        routes={[]}
        isSourcegraphDotCom={true}
        notebooksEnabled={true}
        searchContextsEnabled={true}
        codeMonitoringEnabled={true}
        showSearchBox={true}
        codeInsightsEnabled={true}
        batchChangesEnabled={true}
        authenticatedUser={
            {
                username: 'alice',
                organizations: {
                    nodes: [
                        {
                            __typename: 'Org',
                            id: 'acme',
                            name: 'acme',
                            displayName: 'Acme',
                            url: 'https://example.com',
                            settingsURL: null,
                        },
                    ],
                },
                siteAdmin: true,
            } as AuthenticatedUser
        }
        selectedSearchContextSpec=""
        telemetryService={NOOP_TELEMETRY_SERVICE}
        showFeedbackModal={() => {}}
    />
)
