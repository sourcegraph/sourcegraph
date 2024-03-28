import type { Decorator, Meta, StoryFn } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { updateJSContextBatchChangesLicense } from '@sourcegraph/shared/src/testing/batches'
import {
    mockFetchSearchContexts,
    mockGetUserSearchContextNamespaces,
} from '@sourcegraph/shared/src/testing/searchContexts/testHelpers'
import { Grid, H3 } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { WebStory } from '../components/WebStory'

import { GlobalNavbar, type GlobalNavbarProps } from './GlobalNavbar'

const defaultProps: GlobalNavbarProps = {
    isSourcegraphDotCom: false,
    settingsCascade: {
        final: null,
        subjects: null,
    },
    telemetryService: NOOP_TELEMETRY_SERVICE,
    telemetryRecorder: noOpTelemetryRecorder,
    platformContext: {} as any,
    selectedSearchContextSpec: '',
    setSelectedSearchContextSpec: () => undefined,
    searchContextsEnabled: false,
    batchChangesEnabled: false,
    batchChangesExecutionEnabled: false,
    batchChangesWebhookLogsEnabled: false,
    routes: [],
    fetchSearchContexts: mockFetchSearchContexts,
    getUserSearchContextNamespaces: mockGetUserSearchContextNamespaces,
    showKeyboardShortcutsHelp: () => undefined,
    showSearchBox: false,
    authenticatedUser: null,
    setFuzzyFinderIsVisible: () => {},
    notebooksEnabled: true,
    codeMonitoringEnabled: true,
    ownEnabled: true,
    showFeedbackModal: () => undefined,
}

const allNavItemsProps: Partial<GlobalNavbarProps> = {
    searchContextsEnabled: true,
    batchChangesEnabled: true,
    batchChangesExecutionEnabled: true,
    batchChangesWebhookLogsEnabled: true,
    codeInsightsEnabled: true,
}

const allAuthenticatedNavItemsProps: Partial<GlobalNavbarProps> = {
    authenticatedUser: {
        username: 'alice',
        organizations: { nodes: [{ id: 'acme', name: 'acme' }] },
        siteAdmin: true,
    } as AuthenticatedUser,
}

const decorator: Decorator<GlobalNavbarProps> = Story => {
    updateJSContextBatchChangesLicense('full')

    return (
        <WebStory>
            {() => (
                <div className="mt-3">
                    <Story args={defaultProps} />
                </div>
            )}
        </WebStory>
    )
}

const config: Meta<typeof GlobalNavbar> = {
    title: 'web/nav/GlobalNav',
    decorators: [decorator],
    parameters: {
        chromatic: {
            disableSnapshot: false,
            viewports: [
                // 320, // TODO: Mobile size detection is not working in Storybook
                576, 978,
            ],
        },
    },
}

export default config

export const Default: StoryFn<GlobalNavbarProps> = props => {
    window.context.licenseInfo = { features: { codeSearch: true, cody: true } }
    return (
        <Grid columnCount={1}>
            <div>
                <H3 className="ml-2">Anonymous viewer</H3>
                <GlobalNavbar {...props} />
            </div>
            <div>
                <H3 className="ml-2">Anonymous viewer with all possible nav items</H3>
                <GlobalNavbar {...props} {...allNavItemsProps} />
            </div>
            <div>
                <H3 className="ml-2">Authenticated user with all possible nav items</H3>
                <GlobalNavbar {...props} {...allNavItemsProps} {...allAuthenticatedNavItemsProps} />
            </div>
            <div>
                <H3 className="ml-2">Authenticated user with all possible nav items and search input</H3>
                <GlobalNavbar
                    {...props}
                    {...allNavItemsProps}
                    {...allAuthenticatedNavItemsProps}
                    showSearchBox={true}
                />
            </div>
        </Grid>
    )
}

export const CodyOnlyLicense: StoryFn<GlobalNavbarProps> = props => {
    window.context.licenseInfo = { features: { codeSearch: false, cody: true } }
    return (
        <Grid columnCount={1}>
            <div>
                <H3 className="ml-2">Anonymous viewer</H3>
                <GlobalNavbar {...props} />
            </div>
            <div>
                <H3 className="ml-2">Anonymous viewer with all possible nav items</H3>
                <GlobalNavbar {...props} {...allNavItemsProps} />
            </div>
            <div>
                <H3 className="ml-2">Authenticated user with all possible nav items</H3>
                <GlobalNavbar {...props} {...allNavItemsProps} {...allAuthenticatedNavItemsProps} />
            </div>
            <div>
                <H3 className="ml-2">Authenticated user with all possible nav items and search input</H3>
                <GlobalNavbar
                    {...props}
                    {...allNavItemsProps}
                    {...allAuthenticatedNavItemsProps}
                    showSearchBox={true}
                />
            </div>
        </Grid>
    )
}

export const CodeSearchOnlyLicense: StoryFn<GlobalNavbarProps> = props => {
    window.context.licenseInfo = { features: { codeSearch: true, cody: false } }
    return (
        <Grid columnCount={1}>
            <div>
                <H3 className="ml-2">Anonymous viewer</H3>
                <GlobalNavbar {...props} />
            </div>
            <div>
                <H3 className="ml-2">Anonymous viewer with all possible nav items</H3>
                <GlobalNavbar {...props} {...allNavItemsProps} />
            </div>
            <div>
                <H3 className="ml-2">Authenticated user with all possible nav items</H3>
                <GlobalNavbar {...props} {...allNavItemsProps} {...allAuthenticatedNavItemsProps} />
            </div>
            <div>
                <H3 className="ml-2">Authenticated user with all possible nav items and search input</H3>
                <GlobalNavbar
                    {...props}
                    {...allNavItemsProps}
                    {...allAuthenticatedNavItemsProps}
                    showSearchBox={true}
                />
            </div>
        </Grid>
    )
}
