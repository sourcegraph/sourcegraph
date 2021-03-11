import { SourcegraphContext } from '../jscontext'

export const siteID = 'TestSiteID'
export const siteGQLID = 'TestGQLSiteID'

export const builtinAuthProvider = {
    serviceType: 'builtin' as const,
    serviceID: '',
    clientID: '',
    displayName: 'Builtin username-password authentication',
    isBuiltin: true,
    authenticationURL: '',
}

export const createJsContext = ({ sourcegraphBaseUrl }: { sourcegraphBaseUrl: string }): SourcegraphContext => ({
    externalURL: sourcegraphBaseUrl,
    accessTokensAllow: 'all-users-create',
    allowSignup: false,
    batchChangesEnabled: true,
    codeIntelAutoIndexingEnabled: true,
    externalServicesUserModeEnabled: false,
    productResearchPageEnabled: true,
    csrfToken: 'test-csrf-token',
    assetsRoot: new URL('/.assets', sourcegraphBaseUrl).href,
    deployType: 'dev',
    debug: true,
    emailEnabled: false,
    experimentalFeatures: {},
    isAuthenticatedUser: true,
    likelyDockerOnMac: false,
    needServerRestart: false,
    needsSiteInit: false,
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
})
