import { logger } from '@sourcegraph/common'
import { getDocumentNode, gql, useQuery } from '@sourcegraph/http-client'

import type { EvaluateFeatureFlagResult, EvaluateFeatureFlagVariables } from '../graphql-operations'

import type { FeatureFlagName } from './featureFlags'
import { getFeatureFlagOverride } from './lib/feature-flag-local-overrides'

export const EVALUATE_FEATURE_FLAG_QUERY = getDocumentNode(gql`
    query EvaluateFeatureFlag($flagName: String!) {
        evaluateFeatureFlag(flagName: $flagName)
    }
`)

type FetchStatus = 'initial' | 'loaded' | 'error'
type FlagName = string
type EvaluateError = unknown

const MINUTE = 60000
const FEATURE_FLAG_CACHE_TTL = MINUTE * 10

const FLAG_STATES = new Map<FlagName, 'new_value' | 'valid' | 'stale' | 'refetch'>()

/**
 * Evaluates the feature flag via GraphQL query and returns the value.
 * Prioritizes feature flag overrides. It uses the value cached by Apollo
 * Client if available and not stale. The hook render starts the timeout,
 * which ensures that we mark the cached value as stale after `cacheTTL`.
 * If the cache TTL is elapsed on the next render, we refetch the value.
 */
export function useFeatureFlag(
    flagName: FeatureFlagName,
    defaultValue = false,
    /**
     * Used for tests only
     */
    cacheTTL = FEATURE_FLAG_CACHE_TTL,
    /**
     * Used for tests only
     */
    flagStates = FLAG_STATES
): [boolean, FetchStatus, EvaluateError?] {
    const overriddenValue = getFeatureFlagOverride(flagName)
    const overriddenValueExists = overriddenValue !== null

    const { data, error, refetch } = useQuery<EvaluateFeatureFlagResult, EvaluateFeatureFlagVariables>(
        EVALUATE_FEATURE_FLAG_QUERY,
        {
            variables: { flagName },
            fetchPolicy: 'cache-first',
            // When updating the feature flag value with refetch, we want to skip the cache and rely on the network.
            nextFetchPolicy: 'network-only',
            skip: overriddenValueExists,
        }
    )

    /**
     * Skip the GraphQL query and return the `overriddenValue` if it exists.
     */
    if (overriddenValueExists) {
        return [overriddenValue, 'loaded']
    }

    if (data) {
        /**
         * Flag states are shared between all instances of the React hook, which
         * eliminates the possibility of race conditions and ensures that each flag
         * can have only one scheduled refetch call.
         *
         * Switch cases are defined in chronological order.
         */
        switch (flagStates.get(flagName)) {
            /**
             * Upon receiving the new value from the API, we start the timer unique
             * for the feature flag, marking it as stale after `cacheTTL`.
             */
            case undefined:
            case 'new_value': {
                flagStates.set(flagName, 'valid')
                setTimeout(() => flagStates.set(flagName, 'stale'), cacheTTL)
                break
            }

            /**
             * Do nothing. The `setTimeout` is in progress and we can use the cached value for now.
             */
            case 'valid': {
                break
            }

            /**
             * If we have the stale value, initiate the refetch with the `network-only`
             * strategy. The hook will be re-rendered only if the value has changed after the refetch.
             * On refetch success, we mark the value as the new one to restart the state cycle.
             */
            case 'stale': {
                flagStates.set(flagName, 'refetch')
                refetch()
                    .then(() => flagStates.set(flagName, 'new_value'))
                    .catch(logger.error)
                break
            }

            /**
             * Do nothing. The `refetch` is in progress and we can use the cached value for now.
             */
            case 'refetch': {
                break
            }
        }
    }

    const value = data?.evaluateFeatureFlag ?? defaultValue
    const status = error ? 'error' : data ? 'loaded' : 'initial'

    return [value, status, error?.networkError]
}
