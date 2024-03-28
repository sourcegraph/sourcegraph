/**
 * A service to log telemetry data.
 */
export interface TelemetryService {
    /**
     * Log a telemetry event.
     *
     * PRIVACY: Do NOT include any potentially private information in `eventProperties`. These
     * properties may get sent to analytics tools, so must not include private information, such as
     * search queries or repository names.
     *
     * @param eventName The name of the event.
     * @param properties Event properties. Do NOT include any private information, such as full URLs
     * that may contain private repository names or search queries.
     */
    log(eventName: string, properties?: TelemetryEventProperties): void
}

/**
 * Properties related to a telemetry event.
 */
export interface TelemetryEventProperties {
    [key: string]:
        | string
        | number
        | boolean
        | null
        | undefined
        | string[]
        | { [key: string]: string | number | boolean | null | undefined }
}

/** For testing. */
export const NOOP_TELEMETRY_SERVICE: TelemetryService = {
    log() {
        /* noop */
    },
}
