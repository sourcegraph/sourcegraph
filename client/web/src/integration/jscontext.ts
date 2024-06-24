import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { currentUserMock } from '@sourcegraph/shared/src/testing/integration/graphQlResults'

import type { SourcegraphContext } from '../jscontext'

export const siteID = 'TestSiteID'
export const siteGQLID = 'TestGQLSiteID'

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

export const createJsContext = ({ sourcegraphBaseUrl }: { sourcegraphBaseUrl: string }): SourcegraphContext => ({
    currentUser: currentUserMock,
    temporarySettings: null,
    externalURL: sourcegraphBaseUrl,
    accessTokensAllow: 'all-users-create',
    accessTokensAllowNoExpiration: false,
    accessTokensExpirationDaysDefault: 60,
    accessTokensExpirationDaysOptions: [7, 30, 60, 90],
    allowSignup: false,
    batchChangesEnabled: true,
    batchChangesDisableWebhooksWarning: false,
    batchChangesWebhookLogsEnabled: true,
    codeInsightsEnabled: true,
    executorsEnabled: true,
    codyEnabled: true,
    codeIntelligenceEnabled: true,
    codeMonitoringEnabled: true,
    notebooksEnabled: true,
    searchAggregationEnabled: true,
    searchContextsEnabled: true,
    ownEnabled: true,
    codyEnabledForCurrentUser: true,
    codyRequiresVerifiedEmail: false,
    extsvcConfigAllowEdits: false,
    extsvcConfigFileExists: false,
    codeIntelAutoIndexingEnabled: true,
    codeIntelAutoIndexingAllowGlobalPolicies: true,
    codeIntelRankingDocumentReferenceCountsEnabled: false,
    productResearchPageEnabled: true,
    assetsRoot: new URL('/.assets', sourcegraphBaseUrl).href,
    deployType: 'dev',
    debug: true,
    emailEnabled: false,
    experimentalFeatures: {},
    isAuthenticatedUser: true,
    licenseInfo: {
        batchChanges: {
            maxNumChangesets: -1,
            unrestricted: true,
        },
        features: {
            codeSearch: true,
            cody: true,
        },
    },
    needServerRestart: false,
    needsSiteInit: false,
    needsRepositoryConfiguration: false,
    resetPasswordEnabled: false,
    sentryDSN: null,
    site: {},
    siteID,
    siteGQLID,
    sourcegraphDotComMode: false,
    userAgentIsBot: false,
    version: '0.0.0',
    xhrHeaders: {},
    authProviders: [builtinAuthProvider],
    authMinPasswordLength: 12,
    runningOnMacOS: true,
    primaryLoginProvidersCount: 5,
    // use noOpTelemetryRecorder since this jsContext is only used for integration tests.
    telemetryRecorder: noOpTelemetryRecorder,
})
