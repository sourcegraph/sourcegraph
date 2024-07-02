import { gql } from '@sourcegraph/http-client'
import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import type { TelemetryEventInput, TelemetryExporter } from '@sourcegraph/telemetry'

// todo(dan) update with new recordeventresult type?
import type { logEventResult } from '../../graphql-operations'

/**
 * GraphQLTelemetryExporter exports events via the new Sourcegraph telemetry
 * framework: https://sourcegraph.com/docs/dev/background-information/telemetry
 */
export class GraphQLTelemetryExporter implements TelemetryExporter {
    constructor(private requestGraphQL: PlatformContext['requestGraphQL']) {}

    public exportEvents(events: TelemetryEventInput[]): Promise<void> {
        const req = this.requestGraphQL<logEventResult>({
            request: gql`
                mutation ExportTelemetryEventsFromBrowserExtension($events: [TelemetryEventInput!]!) {
                    telemetry {
                        recordEvents(events: $events) {
                            alwaysNil
                        }
                    }
                }
            `,
            variables: { events },
            mightContainPrivateInfo: false,
        })
        // eslint-disable-next-line rxjs/no-ignored-subscription
        req.subscribe({
            error: _ => {
                // Swallow errors. If a Sourcegraph instance isn't upgraded, this request may fail.
                // However, end users shouldn't experience this failure, as their admin is
                // responsible for updating the instance, and has been (or will be) notified
                // that an upgrade is available via site-admin messaging.
            },
        })
        return Promise.resolve()
    }
}
