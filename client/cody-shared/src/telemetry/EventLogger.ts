import type { ConfigurationWithAccessToken } from '../configuration'
import { SourcegraphGraphQLAPIClient } from '../sourcegraph-api/graphql'
import { isError } from '../utils'

import type { TelemetryEventProperties } from '.'

export interface ExtensionDetails {
    ide: 'VSCode' | 'JetBrains' | 'Neovim' | 'Emacs'
    ideExtensionType: 'Cody' | 'CodeSearch'

    /** Version number for the extension. */
    version: string
}

export class EventLogger {
    private gqlAPIClient: SourcegraphGraphQLAPIClient
    private client: string
    private siteIdentification?: { siteid: string; hashedLicenseKey: string }

    constructor(
        private serverEndpoint: string,
        private extensionDetails: ExtensionDetails,
        private config: ConfigurationWithAccessToken
    ) {
        this.gqlAPIClient = new SourcegraphGraphQLAPIClient(this.config)
        // eslint-disable-next-line no-console
        this.setSiteIdentification().catch(error => console.error(error))
        if (this.extensionDetails.ide === 'VSCode' && this.extensionDetails.ideExtensionType === 'Cody') {
            this.client = 'VSCODE_CODY_EXTENSION'
        } else {
            throw new Error('new extension type not yet accounted for')
        }
    }

    public onConfigurationChange(
        newServerEndpoint: string,
        newExtensionDetails: ExtensionDetails,
        newConfig: ConfigurationWithAccessToken
    ): void {
        this.serverEndpoint = newServerEndpoint
        this.extensionDetails = newExtensionDetails
        this.config = newConfig
        this.gqlAPIClient.onConfigurationChange(newConfig)
        // eslint-disable-next-line no-console
        this.setSiteIdentification().catch(error => console.error(error))
    }

    private async setSiteIdentification(): Promise<void> {
        const siteIdentification = await this.gqlAPIClient.getSiteIdentification()
        if (isError(siteIdentification)) {
            /**
             * Swallow errors. Any instance with a version before https://github.com/sourcegraph/sourcegraph/commit/05184f310f631bb36c6d726792e49ff9d122e4af
             * will return an error here due to it not having new parameters in its GraphQL schema or database schema.
             */
        } else {
            this.siteIdentification = siteIdentification
        }
    }

    /**
     * Log a telemetry event.
     *
     * PRIVACY: Do NOT include any potentially private information in `eventProperties`. These
     * properties may get sent to analytics tools, so must not include private information, such as
     * search queries or repository names.
     * @param eventName The name of the event.
     * @param anonymousUserID The randomly generated unique user ID.
     * @param properties Event properties. Do NOT include any private information, such as full
     * URLs that may contain private repository names or search queries.
     */
    public log(eventName: string, anonymousUserID: string, properties?: TelemetryEventProperties): void {
        const publicArgument = {
            ...properties,
            serverEndpoint: this.serverEndpoint,
            extensionDetails: this.extensionDetails,
            configurationDetails: {
                contextSelection: this.config.useContext,
                chatPredictions: this.config.experimentalChatPredictions,
                inline: this.config.inlineChat,
                nonStop: this.config.experimentalNonStop,
                guardrails: this.config.experimentalGuardrails,
            },
            version: this.extensionDetails.version, // for backcompat
        }
        this.gqlAPIClient
            .logEvent({
                event: eventName,
                userCookieID: anonymousUserID,
                source: 'IDEEXTENSION',
                url: '',
                argument: '{}',
                publicArgument: JSON.stringify(publicArgument),
                client: this.client,
                connectedSiteID: this.siteIdentification?.siteid,
                hashedLicenseKey: this.siteIdentification?.hashedLicenseKey,
            })
            .then(response => {
                if (isError(response)) {
                    // eslint-disable-next-line no-console
                    console.error('Error logging event', response)
                }
            })
            // eslint-disable-next-line no-console
            .catch(error => console.error('Error logging event', error))
    }
}
