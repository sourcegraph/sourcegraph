import type { Decorator, Meta, StoryFn } from '@storybook/react'
import { subHours, subSeconds } from 'date-fns'
import { MATCH_ANY_PARAMETERS, WildcardMockLink } from 'wildcard-mock-link'

import { getDocumentNode } from '@sourcegraph/http-client'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { WebStory } from '../components/WebStory'
import { GitserversResult } from '../graphql-operations'

import { GITSERVERS } from './backend'
import { SiteAdminGitserversPage } from './SiteAdminGitserversPage'

const decorator: Decorator = story => <div className="p-3 container">{story()}</div>

const config: Meta = {
    title: 'web/site-admin/Gitservers',
    decorators: [decorator],
}

export default config

const mocks = new WildcardMockLink([
    {
        request: {
            query: getDocumentNode(GITSERVERS),
            variables: MATCH_ANY_PARAMETERS,
        },
        result: {
            data: {
                __typename: 'Query',
                site: {
                    __typename: 'Site',
                    gitserverDiskUsageWarningThreshold: 90,
                },
                gitservers: {
                    __typename: 'GitserverInstanceConnection',
                    nodes: [
                        {
                            __typename: 'GitserverInstance',
                            id: 'R2l0c2VAuMTozNTAxIg==',
                            address: 'gitserver-1',
                            freeDiskSpaceBytes: '839517454336',
                            totalDiskSpaceBytes: '994662584320',
                            repositoryJobs: {
                                stats: {
                                    queued: 17,
                                    processing: 4,
                                    longestQueuedTime: subHours(new Date(), 1).toISOString(),
                                },
                            },
                        },
                        {
                            __typename: 'GitserverInstance',
                            id: 'R2l0c2VydmTozNTAyIg==',
                            address: 'gitserver-2',
                            freeDiskSpaceBytes: '6005174543',
                            totalDiskSpaceBytes: '994662584320',
                            repositoryJobs: {
                                stats: {
                                    queued: 3,
                                    processing: 1,
                                    longestQueuedTime: subSeconds(new Date(), 1055).toISOString(),
                                },
                            },
                        },
                        {
                            __typename: 'GitserverInstance',
                            id: 'R2l0c2VAuMTozNTssAxIg==',
                            address: 'gitserver-3',
                            freeDiskSpaceBytes: '93059174543',
                            totalDiskSpaceBytes: '994662584320',
                            repositoryJobs: {
                                stats: {
                                    queued: 0,
                                    processing: 1,
                                    longestQueuedTime: null,
                                },
                            },
                        },
                    ],
                },
            } as GitserversResult,
        },
        nMatches: Number.POSITIVE_INFINITY,
    },
])

export const GitserversPage: StoryFn = () => (
    <WebStory>
        {() => (
            <MockedTestProvider link={mocks}>
                <SiteAdminGitserversPage telemetryService={NOOP_TELEMETRY_SERVICE} />
            </MockedTestProvider>
        )}
    </WebStory>
)

GitserversPage.storyName = 'Site Admin Gitservers Page'
