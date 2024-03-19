import type { MockedResponse } from '@apollo/client/testing'
import type { Meta, StoryFn } from '@storybook/react'

import { getDocumentNode } from '@sourcegraph/http-client'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import { WebStory } from '../../../components/WebStory'
import type {
    GetInstanceOwnStatsResult,
    GetInstanceOwnStatsVariables,
    GetOwnSignalConfigurationsResult,
    GetOwnSignalConfigurationsVariables,
} from '../../../graphql-operations'

import { OwnAnalyticsPage } from './OwnAnalyticsPage'
import { GET_INSTANCE_OWN_STATS, GET_OWN_JOB_CONFIGURATIONS } from './query'

const config: Meta = {
    title: 'web/enterprise/own/admin-ui/OwnAnalyticsPage',
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

const ownAnalyticsDisabled: MockedResponse<GetOwnSignalConfigurationsResult, GetOwnSignalConfigurationsVariables> = {
    request: {
        query: getDocumentNode(GET_OWN_JOB_CONFIGURATIONS),
    },
    result: {
        data: {
            ownSignalConfigurations: [
                {
                    __typename: 'OwnSignalConfiguration',
                    name: 'analytics',
                    description: 'unused',
                    isEnabled: false,
                    excludedRepoPatterns: [],
                },
            ],
        },
    },
}

export const AnalyticsDisabled: StoryFn = () => (
    <WebStory mocks={[ownAnalyticsDisabled]}>
        {() => <OwnAnalyticsPage telemetryRecorder={noOpTelemetryRecorder} />}
    </WebStory>
)
AnalyticsDisabled.storyName = 'AnalyticsDisabled - need to enable own analytics in site admin'

const ownAnalyticsEnabled: MockedResponse<GetOwnSignalConfigurationsResult, GetOwnSignalConfigurationsVariables> = {
    request: {
        query: getDocumentNode(GET_OWN_JOB_CONFIGURATIONS),
    },
    result: {
        data: {
            ownSignalConfigurations: [
                {
                    __typename: 'OwnSignalConfiguration',
                    name: 'analytics',
                    description: 'unused',
                    isEnabled: true,
                    excludedRepoPatterns: [],
                },
            ],
        },
    },
}

const presentStats: MockedResponse<GetInstanceOwnStatsResult, GetInstanceOwnStatsVariables> = {
    request: {
        query: getDocumentNode(GET_INSTANCE_OWN_STATS),
    },
    result: {
        data: {
            instanceOwnershipStats: {
                totalFiles: 375311,
                totalCodeownedFiles: 5404,
                totalOwnedFiles: 5528,
                totalAssignedOwnershipFiles: 200,
                updatedAt: '2023-06-20T12:46:54Z',
                __typename: 'OwnershipStats',
            },
        },
    },
}

export const PresentStats: StoryFn = () => (
    <WebStory mocks={[ownAnalyticsEnabled, presentStats]}>
        {() => <OwnAnalyticsPage telemetryRecorder={noOpTelemetryRecorder} />}
    </WebStory>
)
PresentStats.storyName = 'PresentStats - statistics available'

const zeroStats: MockedResponse<GetInstanceOwnStatsResult, GetInstanceOwnStatsVariables> = {
    request: {
        query: getDocumentNode(GET_INSTANCE_OWN_STATS),
    },
    result: {
        data: {
            instanceOwnershipStats: {
                totalFiles: 0,
                totalCodeownedFiles: 0,
                totalOwnedFiles: 0,
                totalAssignedOwnershipFiles: 0,
                updatedAt: null,
                __typename: 'OwnershipStats',
            },
        },
    },
}

export const ZeroStats: StoryFn = () => (
    <WebStory mocks={[ownAnalyticsEnabled, zeroStats]}>
        {() => <OwnAnalyticsPage telemetryRecorder={noOpTelemetryRecorder} />}
    </WebStory>
)
ZeroStats.storyName = 'ZeroStats - no statistics computed yet'
