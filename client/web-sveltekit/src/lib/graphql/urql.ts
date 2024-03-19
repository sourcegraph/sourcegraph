import {
    Client,
    cacheExchange,
    fetchExchange,
    mapExchange,
    type Exchange,
    makeOperation,
    type AnyVariables,
    type OperationResult,
    createRequest,
    type DocumentInput,
} from '@urql/core'
import type { OperationDefinitionNode } from 'graphql'
import { once } from 'lodash'
import { from, isObservable, Subject, type Observable, concat, of } from 'rxjs'
import { map, switchMap, scan, startWith } from 'rxjs/operators'
import { type Readable, readable, get } from 'svelte/store'

import type { GraphQLResult } from '@sourcegraph/http-client'

import { GRAPHQL_URI } from '$lib/http-client'

import { getHeaders } from './shared'

export type GraphQLClient = Client

export { gql, createRequest, type DocumentInput, type OperationResult } from '@urql/core'

/**
 * This exchange appends the operation name to the URL for each operation.
 */
const appendOperationName: Exchange = mapExchange({
    onOperation: op => {
        const operationName = op.query.definitions.find(
            (def): def is OperationDefinitionNode => def.kind === 'OperationDefinition'
        )?.name?.value
        if (operationName) {
            return makeOperation(op.kind, op, {
                ...op.context,
                url: `${op.context.url}?${operationName}`,
            })
        }
        return op
    },
})

export const getGraphQLClient = once((): Client => {
    return new Client({
        url: GRAPHQL_URI,
        fetchOptions: () => ({
            headers: getHeaders(),
        }),
        exchanges: [cacheExchange, appendOperationName, fetchExchange],
    })
})

// TODO: Refactor to eliminate the need for this function
/**
 * @deprecated Initiated GraphQL requests in data loader functions instead
 */
export function query<TData = any, TVariables extends AnyVariables = AnyVariables>(
    query: DocumentInput<TData, TVariables>,
    variables: TVariables
): Promise<OperationResult<TData, TVariables>> {
    return getGraphQLClient().query<TData, TVariables>(query, variables).toPromise()
}

interface InfinityQueryArgs<TData = any, TVariables extends AnyVariables = AnyVariables> {
    /**
     * The {@link Client} instance to use for the query.
     */
    client: Client

    /**
     * The GraphQL query to execute.
     */
    query: DocumentInput<TData, TVariables>

    /**
     * The initial variables to use for the query.
     */
    variables: TVariables | Observable<TVariables>

    /**
     * A function that returns the next set of variables to use for the query.
     *
     * @param previousResult - The previous result of the query.
     *
     * @remarks
     * `nextVariables` is called when {@link InfinityQueryStore.fetchMore} is called to get the next set
     * of variables to fetch the next page of data. This function to extract the cursor for the next
     * page from the previous result.
     */
    nextVariables: (previousResult: OperationResult<TData, TVariables>) => Partial<TVariables> | undefined

    /**
     * A function to combine the previous result with the next result.
     *
     * @param previousResult - The previous result of the query.
     * @param nextResult - The next result of the query.
     * @returns The combined result of the query.
     *
     * @remarks
     * `combine` is called when the next result is received to merge the previous result with the new
     * result. This function is used to append the new data to the previous data.
     */
    combine: (
        previousResult: OperationResultState<TData, TVariables>,
        nextResult: OperationResultState<TData, TVariables>
    ) => OperationResultState<TData, TVariables>
}

interface OperationResultState<TData = any, TVariables extends AnyVariables = AnyVariables>
    extends OperationResult<TData, TVariables> {
    /**
     * Whether a GraphQL request is currently in flight.
     */
    fetching: boolean
    /**
     * Whether the store is currently restoring data.
     */
    restoring: boolean
}

interface InfinityQueryStore<TData = any, TVariables extends AnyVariables = AnyVariables>
    extends Readable<OperationResultState<TData, TVariables>> {
    /**
     * Reruns the query with the next set of variables returned by {@link InfinityQueryArgs.nextVariables}.
     *
     * @remarks
     * A new query will only be executed if there is no query currently in flight and {@link InfinityQueryArgs.nextVariables}
     * returns a value different from `undefined`.
     */
    fetchMore: () => void

    /**
     * Fetches more data until the given restoreHandler returns `false`.
     *
     * @param restoreHandler - A function that returns `true` if more data should be fetched.
     *
     * @remarks
     * When navigating back to a page that was previously fetched with `infinityQuery`, the page
     * should call `restore` until the previous data state is restored.
     */
    restore: (restoreHandler: (result: OperationResultState<TData, TVariables>) => boolean) => Promise<void>
}

/**
 * Function to create a store to manage "infinity scroll" style queries.
 *
 * @param args - a {@link InfinityQueryArgs} object to pass a `query, `variables` and other options to manage infinity scroll.
 * @returns a {@link InfinityQueryStore} of query results.
 *
 * @remarks
 * `infinityQuery` uses {@link InfinityQueryArgs.client} to execute {@link InfinityQueryArgs.query}
 * with the given {@link InfinityQueryArgs.variables}.
 *
 * The caller can call {@link InfinityQueryStore.fetchMore} to fetch more data. The store will
 * call {@link InfinityQueryArgs.nextVariables} to get the next set of variables to use for the query
 * and merge it into the initial variables.
 * When the result is received, the store will call {@link InfinityQueryArgs.combine} to merge the
 * previous result with the new result.
 *
 * Calling this function will prefetch the initial data, i.e. the data is fetch before the store is
 * subscribed to.
 */
export function infinityQuery<TData = any, TVariables extends AnyVariables = AnyVariables>(
    args: InfinityQueryArgs<TData, TVariables>
): InfinityQueryStore<TData, TVariables> {
    // This is a hacky workaround to create an initialState. The empty object is
    // invalid but the request will never be executed with these variables anyway.
    const initialVariables = isObservable(args.variables) ? args.variables : of(args.variables)
    const operation = args.client.createRequestOperation(
        'query',
        isObservable(args.variables)
            ? createRequest(args.query, {} as TVariables)
            : createRequest(args.query, args.variables)
    )
    const initialState: OperationResultState<TData, TVariables> = {
        operation,
        error: undefined,
        data: undefined,
        extensions: undefined,
        stale: false,
        fetching: false,
        restoring: false,
        hasNext: false,
    }
    const nextVariables = new Subject<Partial<TVariables>>()
    let shouldRestore: ((result: OperationResultState<TData, TVariables>) => boolean) | null = null

    // Prefetch data. We don't want to wait until the store is subscribed to. That allows us to use this function
    // inside a data loader and the data will be prefetched before the component is rendered.
    initialVariables.subscribe(variables => {
        void args.client.query(args.query, variables).toPromise()
    })

    const result = readable(initialState, set => {
        const subscription = initialVariables
            .pipe(
                switchMap(initialVariables =>
                    nextVariables.pipe(
                        startWith(initialVariables), // nextVaribles will not emit until the first fetchMore is called
                        switchMap(variables => {
                            const operation = args.client.createRequestOperation(
                                'query',
                                createRequest(args.query, { ...initialVariables, ...variables })
                            )
                            return concat(
                                of({ fetching: true, stale: false, restoring: false }),
                                from(args.client.executeRequestOperation(operation).toPromise()).pipe(
                                    map(({ data, stale, operation, error, extensions }) => ({
                                        fetching: false,
                                        data,
                                        stale: !!stale,
                                        operation,
                                        error,
                                        extensions,
                                    }))
                                )
                            )
                        })
                    )
                ),
                scan((result, update) => {
                    const newResult = { ...result, ...update }
                    return update.fetching ? newResult : args.combine(result, newResult)
                }, initialState)
            )
            .subscribe(result => {
                if (shouldRestore) {
                    result.restoring = Boolean(
                        (result.data || result.error) && shouldRestore(result) && args.nextVariables(result)
                    )
                }
                set(result)
            })

        return () => subscription.unsubscribe()
    })

    return {
        ...result,
        fetchMore: () => {
            const current = get(result)
            if (current.fetching || current.restoring) {
                return
            }
            const newVariables = args.nextVariables(current)
            if (!newVariables) {
                return
            }
            nextVariables.next(newVariables)
        },
        restore: restoreHandler => {
            shouldRestore = result => {
                return Boolean((result.data || result.error) && restoreHandler(result) && args.nextVariables(result))
            }
            return new Promise(resolve => {
                const unsubscribe = result.subscribe(result => {
                    if (result.fetching) {
                        return
                    }
                    if (result.data || result.error) {
                        const newVariables = args.nextVariables(result)
                        if (restoreHandler(result) && newVariables) {
                            shouldRestore = restoreHandler
                            nextVariables.next(newVariables)
                        } else {
                            unsubscribe()
                            shouldRestore = null
                            resolve()
                        }
                    }
                })
            })
        },
    }
}

/**
 * Converts an OperationResult (urlql) to a GraphQLResult (sourcegraph/http-client).
 */
export function toGraphQLResult<TData = any, TVariables extends AnyVariables = AnyVariables>(
    result: OperationResult<TData, TVariables>
): GraphQLResult<TData> {
    return result.error || !result.data
        ? {
              ...result,
              data: result.data ?? null,
              errors: result.error?.graphQLErrors ?? [],
          }
        : {
              ...result,
              data: result.data,
              errors: undefined,
          }
}

/**
 * Given an {@link OperationResult}, this function returns the mapped data or throws the error.
 * To be used toghether with a promise that resolves to a GraphQL response. This ensures
 * that the promise rejects when the GraphQL response contains an error.
 *
 * @param mapper - A function to map the data from the result.
 */
export function mapOrThrow<T extends OperationResult, U>(mapper: (result: T) => U): (result: T) => U {
    return (result: T) => {
        if (result.error) {
            throw result.error
        }
        return mapper(result)
    }
}
