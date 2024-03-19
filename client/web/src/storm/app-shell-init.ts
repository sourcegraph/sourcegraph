import type { GraphQLClient } from '@sourcegraph/http-client'
import { TemporarySettingsStorage } from '@sourcegraph/shared/src/settings/temporary/TemporarySettingsStorage'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'

import { getWebGraphQLClient } from '../backend/graphql'
import { useDeveloperSettings } from '../stores'

export interface AppShellInit extends TelemetryV2Props {
    graphqlClient: GraphQLClient
    temporarySettingsStorage: TemporarySettingsStorage
}

export async function initAppShell(): Promise<AppShellInit> {
    const graphqlClient = await getWebGraphQLClient()
    const temporarySettingsStorage = new TemporarySettingsStorage(
        graphqlClient,
        window.context.isAuthenticatedUser,
        process.env.NODE_ENV === 'development' || useDeveloperSettings.getState().enabled
    )

    const platformContext = window.context
    const telemetryRecorder = platformContext.telemetryRecorder

    return {
        graphqlClient,
        temporarySettingsStorage,
        telemetryRecorder,
    }
}
