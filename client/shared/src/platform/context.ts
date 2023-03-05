import { Endpoint } from 'comlink'
import { isObject } from 'lodash'
import { Observable, Subscribable, Subscription } from 'rxjs'

import { DiffPart } from '@sourcegraph/codeintellify'
import { hasProperty } from '@sourcegraph/common'
import { GraphQLClient, GraphQLResult } from '@sourcegraph/http-client'

import { SettingsEdit } from '../api/client/services/settings'
import { Scalars } from '../graphql-operations'
import { Settings, SettingsCascadeOrError } from '../settings/settings'
import { TelemetryService } from '../telemetry/telemetryService'
import { FileSpec, UIPositionSpec, RawRepoSpec, RepoSpec, RevisionSpec, ViewStateSpec } from '../util/url'

// TODO(sqs): all this endpoint stuff is not really desirable here anymore
export interface EndpointPair {
    /** The endpoint to proxy the API of the other thread from */
    proxy: Endpoint

    /** The endpoint to expose the API of this thread to */
    expose: Endpoint
}

export interface ClosableEndpointPair {
    endpoints: EndpointPair

    /** Destroys worker or iframe depending on the environment. */
    subscription: Subscription
}

const isEndpoint = (value: unknown): value is Endpoint =>
    isObject(value) &&
    hasProperty('addEventListener')(value) &&
    hasProperty('removeEventListener')(value) &&
    hasProperty('postMessage')(value) &&
    typeof value.addEventListener === 'function' &&
    typeof value.removeEventListener === 'function' &&
    typeof value.postMessage === 'function'

export const isEndpointPair = (value: unknown): value is EndpointPair =>
    isObject(value) &&
    hasProperty('proxy')(value) &&
    hasProperty('expose')(value) &&
    isEndpoint(value.proxy) &&
    isEndpoint(value.expose)

/**
 * Context information of an invocation of `urlToFile`
 */
export interface URLToFileContext {
    /**
     * If `urlToFile` is called because of a go to definition invocation on a diff,
     * the part of the diff it was invoked on.
     */
    part: DiffPart | undefined

    isWebURL?: boolean
}

/**
 * Platform-specific data and methods shared by multiple Sourcegraph components.
 *
 * Whenever shared code (in shared/) needs to perform an action or retrieve data that requires different
 * implementations depending on the platform, the shared code should use this value's fields.
 */
export interface PlatformContext {
    /**
     * An observable that emits the settings cascade upon subscription and whenever it changes (including when it
     * changes as a result of a call to {@link PlatformContext#updateSettings}).
     *
     * It should be a cold observable so that it does not trigger a network request upon each subscription.
     *
     * @deprecated Use useSettings instead
     */
    readonly settings: Subscribable<SettingsCascadeOrError<Settings>>

    /**
     * Update the settings for the subject, either by inserting/changing a specific value or by overwriting the
     * entire settings with a new stringified JSON value.
     *
     * This function must ensure that {@link PlatformContext#settings} reflects the updated settings before its
     * returned promise resolves.
     *
     * @todo To make updating settings feel more responsive, make this return an observable that emits twice (by
     * convention): once immediately with the optimistic new value, and once when it is acked by the server (then
     * completes).
     *
     * @param subject The settings subject whose settings to update.
     * @param edit An edit to a specific value, or a stringified JSON value to overwrite the current settings with.
     * @returns A promise that resolves after the update succeeds and {@link PlatformContext#settings} reflects the
     * update.
     */
    updateSettings: (subject: Scalars['ID'], edit: SettingsEdit | string) => Promise<void>

    /**
     * Returns promise that resolves into Apollo Client instance after cache restoration.
     * Only `watchQuery` is available till https://github.com/sourcegraph/sourcegraph/issues/24953 is implemented.
     *
     * @deprecated Use [Apollo](docs.sourcegraph.com/dev/background-information/web/graphql#graphql-client) instead
     */
    getGraphQLClient: () => Promise<Pick<GraphQLClient, 'watchQuery'>>

    /**
     * Sends a request to the Sourcegraph GraphQL API and returns the response.
     *
     * @template R The GraphQL result type
     * could leak private information such as repository names.
     * @returns Observable that emits the result or an error if the HTTP request failed
     *
     * @deprecated Use [Apollo](docs.sourcegraph.com/dev/background-information/web/graphql#graphql-client) instead
     */
    requestGraphQL: <R, V extends { [key: string]: any } = object>(options: {
        /**
         * The GraphQL request (query or mutation)
         */
        request: string
        /**
         * An object whose properties are GraphQL query name-value variable pairs
         */
        variables: V
        /**
         * 🚨 SECURITY: Whether or not sending the GraphQL request to Sourcegraph.com
         * could leak private information such as repository names.
         */
        mightContainPrivateInfo: boolean
    }) => Observable<GraphQLResult<R>>

    /**
     * Constructs the URL (possibly relative or absolute) to the file with the specified options.
     *
     * @param target The specific repository, revision, file, position, and view state to generate the URL for.
     * @param context Contextual information about the context of this invocation.
     * @returns The URL to the file with the specified options.
     */
    urlToFile: (
        target: RepoSpec &
            Partial<RawRepoSpec> &
            RevisionSpec &
            FileSpec &
            Partial<UIPositionSpec> &
            Partial<ViewStateSpec>,
        context: URLToFileContext
    ) => string

    /**
     * The URL to the Sourcegraph site that the user's session is associated with. This refers to
     * Sourcegraph.com (`https://sourcegraph.com`) by default, or a self-hosted instance of
     * Sourcegraph.
     *
     * This is available to extensions in `sourcegraph.internal.sourcegraphURL`.
     *
     * @todo Consider removing this when https://github.com/sourcegraph/sourcegraph/issues/566 is
     * fixed.
     *
     * @example `https://sourcegraph.com`
     */
    sourcegraphURL: string

    /**
     * A telemetry service implementation to log events.
     * Optional because it's currently only used in the web app platform.
     */
    telemetryService?: TelemetryService
}

/**
 * React partial props for components needing the {@link PlatformContext}.
 */
export interface PlatformContextProps<K extends keyof PlatformContext = keyof PlatformContext> {
    platformContext: Pick<PlatformContext, K>
}
