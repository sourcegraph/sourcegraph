import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import type { SourcegraphContext } from '../../src/jscontext'

import { ENVIRONMENT_CONFIG } from './environment-config'
import { getSiteConfig } from './get-site-config'

// TODO: share with `client/web/src/integration/jscontext` which is not included into `tsconfig.json` now.
export const builtinAuthProvider = {
    serviceType: 'builtin' as const,
    serviceID: '',
    clientID: '',
    displayName: 'Builtin username-password authentication',
    isBuiltin: true,
    authenticationURL: '',
    noSignIn: false,
    requiredForAuthz: false,
}

// Create dummy JS context that will be added to index.html when `WEB_BUILDER_SERVE_INDEX` is set to true.
export const createJsContext = ({ sourcegraphBaseUrl }: { sourcegraphBaseUrl: string }): SourcegraphContext => {
    const siteConfig = getSiteConfig()

    if (siteConfig?.authProviders) {
        siteConfig.authProviders.unshift(builtinAuthProvider)
    }

    const jsContext: SourcegraphContext = {
        currentUser: null,
        temporarySettings: null,
        externalURL: sourcegraphBaseUrl,
        accessTokensAllow: 'all-users-create',
        accessTokensAllowNoExpiration: false,
        accessTokensExpirationDaysDefault: 90,
        accessTokensExpirationDaysOptions: [7, 14, 30, 60, 90],
        allowSignup: true,
        batchChangesEnabled: true,
        batchChangesDisableWebhooksWarning: false,
        batchChangesWebhookLogsEnabled: true,
        executorsEnabled: false,
        codyEnabled: true,
        codyEnabledForCurrentUser: true,
        codyRequiresVerifiedEmail: false,
        codeIntelAutoIndexingEnabled: false,
        codeIntelAutoIndexingAllowGlobalPolicies: false,
        codeIntelligenceEnabled: true,
        codeIntelRankingDocumentReferenceCountsEnabled: false,
        codeInsightsEnabled: true,
        codeMonitoringEnabled: true,
        productResearchPageEnabled: true,
        assetsRoot: '/.assets',
        deployType: 'dev',
        debug: true,
        emailEnabled: false,
        experimentalFeatures: {},
        extsvcConfigAllowEdits: false,
        extsvcConfigFileExists: false,
        isAuthenticatedUser: true,
        needServerRestart: false,
        needsSiteInit: false,
        needsRepositoryConfiguration: false,
        notebooksEnabled: true,
        ownEnabled: true,
        resetPasswordEnabled: true,
        runningOnMacOS: true,
        searchAggregationEnabled: true,
        searchContextsEnabled: true,
        sentryDSN: null,
        site: {
            'update.channel': 'release',
        },
        siteID: 'TestSiteID',
        siteGQLID: 'TestGQLSiteID',
        sourcegraphDotComMode: ENVIRONMENT_CONFIG.SOURCEGRAPHDOTCOM_MODE,
        userAgentIsBot: false,
        version: '0.0.0',
        xhrHeaders: {},
        authProviders: [builtinAuthProvider],
        authMinPasswordLength: 12,
        authPasswordPolicy: {
            enabled: false,
            numberOfSpecialCharacters: 2,
            requireAtLeastOneNumber: true,
            requireUpperandLowerCase: true,
        },
        openTelemetry: {
            endpoint: ENVIRONMENT_CONFIG.CLIENT_OTEL_EXPORTER_OTLP_ENDPOINT,
        },
        telemetryRecorder: noOpTelemetryRecorder,
        primaryLoginProvidersCount: 5,
        // Site-config overrides default JS context
        ...siteConfig,
    }

    return jsContext
}
