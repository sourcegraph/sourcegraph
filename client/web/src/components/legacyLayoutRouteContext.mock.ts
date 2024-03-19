import { of } from 'rxjs'

import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { NOOP_PLATFORM_CONTEXT } from '@sourcegraph/shared/src/testing/searchTestHelpers'

import type {
    LegacyLayoutRouteContext,
    LegacyRouteComputedContext,
    LegacyRouteStaticInjections,
} from '../LegacyRouteContext'
import type { DynamicSourcegraphWebAppContext, StaticSourcegraphWebAppContext } from '../SourcegraphWebApp'
import type { StaticInjectedAppConfig, StaticWindowContextComputedAppConfig } from '../staticAppConfig'

export const windowContextConfig = {
    isSourcegraphDotCom: false,
    needsRepositoryConfiguration: false,
    batchChangesWebhookLogsEnabled: true,
    batchChangesEnabled: true,
    codeIntelligenceEnabled: true,
    codeInsightsEnabled: true,
    searchContextsEnabled: true,
    notebooksEnabled: true,
    codeMonitoringEnabled: true,
    searchAggregationEnabled: true,
    ownEnabled: true,
} satisfies StaticWindowContextComputedAppConfig

export const injectedAppConfig = {} as unknown as StaticInjectedAppConfig

export const staticWebAppConfig = {
    setSelectedSearchContextSpec: () => {},
    platformContext: NOOP_PLATFORM_CONTEXT as PlatformContext,
} satisfies StaticSourcegraphWebAppContext

export const dynamicWebAppConfig = {
    selectedSearchContextSpec: '',
    authenticatedUser: null,
    settingsCascade: {
        final: null,
        subjects: null,
    },
    viewerSubject: {
        id: 'TEST_ID',
        viewerCanAdminister: false,
    },
} satisfies DynamicSourcegraphWebAppContext

export const legacyRouteComputedContext = {
    batchChangesExecutionEnabled: true,
    isMacPlatform: true,
} satisfies LegacyRouteComputedContext

export const legacyRouteInjectedContext = {
    getUserSearchContextNamespaces: () => [],
    fetchSearchContexts: () => of({}),
    fetchSearchContextBySpec: () => of({}),
    fetchSearchContext: () => of({}),
    createSearchContext: () => of({}),
    updateSearchContext: () => of({}),
    deleteSearchContext: () => of({}),
    isSearchContextSpecAvailable: () => of({}),
    streamSearch: () => of({}),
    fetchHighlightedFileLineRanges: () => of({}),
    telemetryService: NOOP_TELEMETRY_SERVICE,
} as Record<keyof LegacyRouteStaticInjections, unknown> as LegacyRouteStaticInjections

export const legacyLayoutRouteContextMock = {
    ...windowContextConfig,
    ...injectedAppConfig,
    ...staticWebAppConfig,
    ...dynamicWebAppConfig,
    ...legacyRouteComputedContext,
    ...legacyRouteInjectedContext,
    licenseFeatures: {
        isCodeSearchEnabled: true,
        isCodyEnabled: true,
    },
} satisfies LegacyLayoutRouteContext
