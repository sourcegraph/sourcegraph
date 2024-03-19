import { gql, useQuery } from '@sourcegraph/http-client'

import type { OrgFeatureFlagOverridesResult, OrgFeatureFlagOverridesVariables } from '../graphql-operations'

// A union of all feature flags we currently have.
export const FEATURE_FLAGS = [
    'admin-analytics-cache-disabled',
    'admin-onboarding',
    'auditlog-expansion',
    'blob-page-switch-areas-shortcuts',
    'clone-progress-logging',
    'cody-chat-mock-test',
    'cody-web-search',
    'contrast-compliant-syntax-highlighting',
    'enable-ownership-panels',
    'enable-simple-search',
    'web-next',
    'web-next-rollout',
    'web-next-toggle',
    'end-user-onboarding',
    'insight-polling-enabled',
    'opencodegraph',
    'own-analytics',
    'own-promote',
    'plg-enable-add-codehost-widget',
    'quick-start-tour-for-authenticated-users',
    'repository-metadata',
    'search-content-based-lang-detection',
    'search-debug',
    'signup-survey-enabled',
    'sourcegraph-operator-site-admin-hide-maintenance',
    'cody-payments-testing-mode',
] as const

export type FeatureFlagName = typeof FEATURE_FLAGS[number]

interface OrgFlagOverride {
    orgID: string
    flagName: string
    value: boolean
}

/**
 * // TODO: clarify why to use this if GQL already takes care of overrides?
 * Fetches all feature flag overrides for organizations that the current user is a member of
 */
export function useFlagsOverrides(): { data: OrgFlagOverride[]; loading: boolean } {
    const { data, loading } = useQuery<OrgFeatureFlagOverridesResult, OrgFeatureFlagOverridesVariables>(
        gql`
            query OrgFeatureFlagOverrides {
                organizationFeatureFlagOverrides {
                    namespace {
                        id
                    }
                    targetFlag {
                        ... on FeatureFlagBoolean {
                            name
                        }
                        ... on FeatureFlagRollout {
                            name
                        }
                    }
                    value
                }
            }
        `,
        { fetchPolicy: 'cache-and-network' }
    )

    if (!data) {
        return { data: [], loading }
    }

    return {
        data: data?.organizationFeatureFlagOverrides.map(value => ({
            orgID: value.namespace.id,
            flagName: value.targetFlag.name,
            value: value.value,
        })),
        loading,
    }
}
