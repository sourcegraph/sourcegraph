import { useCallback, useEffect, useRef, useState } from 'react'

import {
    of,
    from,
    Subject,
    type ObservableInput,
    type Observable,
    asyncScheduler,
    scheduled,
    type Unsubscribable,
} from 'rxjs'
import { mergeMap, map, takeUntil, take, catchError, takeWhile, switchMap, share } from 'rxjs/operators'

import { type ErrorLike, asError, isErrorLike } from '@sourcegraph/common'

interface Request<T> {
    /**
     * Request factory (return promise or subscription like request)
     */
    request: () => ObservableInput<T>

    /**
     * Calls whenever request resolves
     *
     * @param result - fetch result from the request
     */
    onComplete: (result: T | ErrorLike) => void

    /**
     * Cancel request stream in order to prevent execution of ongoing  stream
     */
    cancel: Observable<boolean>
}

export interface FetchResult<T> {
    data: T | undefined
    error: ErrorLike | undefined
    loading: boolean
}

export enum LazyQueryStatus {
    Loading,
    Data,
    Error,
}

export type LazyQueryState<T> =
    | { status: LazyQueryStatus.Loading }
    | { status: LazyQueryStatus.Data; data: T }
    | { status: LazyQueryStatus.Error; error: ErrorLike }

export interface LazyQueryResult<T> {
    state: LazyQueryState<T>
    query: (request: () => ObservableInput<T>) => Unsubscribable
}

const MAX_PARALLEL_QUERIES = 3

/**
 * Parallel requests hooks factory. This factory/function generates special
 * fetching hooks for code insights cards. These hooks are connected to
 * the inner requests pipeline that is responsible for parallelization and
 * scheduling requests execution.
 *
 * Since this factory generates hooks and this happens in this module's runtime
 * it's safe to disable the rules of hooks here. This factory is also used in
 * these hooks unit tests.
 */
/* eslint-disable react-hooks/rules-of-hooks */
// eslint-disable-next-line @typescript-eslint/explicit-function-return-type
export function createUseParallelRequestsHook<T>({ maxRequests } = { maxRequests: MAX_PARALLEL_QUERIES }) {
    const requests = new Subject<Request<T>>()
    const cancelledRequests = new Set<Request<T>>()

    // Global pipeline query/request manager
    requests
        .pipe(
            // Merge map is used here for the concurrent logic over gql request operations.
            // Only N (be default 2) requests can be run in parallel
            mergeMap(event => {
                const { request, onComplete, cancel } = event

                // Makes this stream async to be able to stop further execution in takeWhile
                // by add event to the cancelledRequests set.
                return scheduled([null], asyncScheduler).pipe(
                    takeWhile(() => {
                        if (cancelledRequests.has(event)) {
                            // Make sure we do not have a memory leak in cancelledRequests set.
                            cancelledRequests.delete(event)

                            return false
                        }

                        return true
                    }),
                    switchMap(() =>
                        from(request()).pipe(
                            // In order to be able to cancel this ongoing stream/request
                            takeUntil(cancel),
                            map(payload => ({ payload, onComplete })),
                            // In order to close observable and free up space for other queued requests
                            // in merge map queue. Consider moving this into consumers request calls
                            take(1),
                            catchError(error =>
                                of({
                                    payload: asError(error),
                                    onComplete,
                                })
                            )
                        )
                    )
                )
            }, maxRequests)
        )
        // eslint-disable-next-line rxjs/no-ignored-subscription
        .subscribe(event => {
            const { payload, onComplete } = event

            onComplete(payload)
        })

    return {
        /**
         * Runs your request in parallel with other request that have been made with
         * useParallelRequests request calls.
         *
         * @param request - request factory (observer, promise, subscribable like)
         */
        query: <D>(request: () => ObservableInput<D>): FetchResult<D> => {
            const [state, setState] = useState<FetchResult<D>>({
                data: undefined,
                error: undefined,
                loading: true,
            })

            useEffect(() => {
                const cancelStream = new Subject<boolean>()

                setState({ data: undefined, loading: true, error: undefined })

                const event: Request<D> = {
                    request,
                    // Makes cancel stream a hot observable
                    cancel: cancelStream.pipe(
                        share({
                            resetOnError: false,
                            resetOnComplete: false,
                            resetOnRefCountZero: false,
                        })
                    ),
                    onComplete: result => {
                        if (isErrorLike(result)) {
                            return setState({ data: undefined, loading: false, error: result })
                        }

                        setState({ data: result, loading: false, error: undefined })
                    },
                }

                requests.next(event as unknown as Request<T>)

                return () => {
                    // Cancel scheduled stream
                    cancelledRequests.add(event as unknown as Request<T>)

                    // Stop/cancel ongoing/started request stream
                    cancelStream.next(true)
                }
            }, [request])

            return state
        },
        /**
         * This provides query methods that allow to you run your request in parallel with
         * other request that have been made with useParallelRequests request calls.
         */
        lazyQuery: <D>(): LazyQueryResult<D> => {
            const [state, setState] = useState<LazyQueryState<D>>({ status: LazyQueryStatus.Loading })

            const localRequestPool = useRef<Request<D>[]>([])

            useEffect(
                () => () => {
                    for (const request of localRequestPool.current) {
                        // Cancel scheduled stream
                        cancelledRequests.add(request as unknown as Request<T>)
                    }
                },
                []
            )

            const query = useCallback((request: () => ObservableInput<D>) => {
                const cancelStream = new Subject<boolean>()

                setState({ status: LazyQueryStatus.Loading })

                const event: Request<D> = {
                    request,
                    // Makes cancel stream a hot observable
                    cancel: cancelStream.pipe(
                        share({
                            resetOnError: false,
                            resetOnComplete: false,
                            resetOnRefCountZero: false,
                        })
                    ),
                    onComplete: result => {
                        localRequestPool.current = localRequestPool.current.filter(request => request !== event)

                        if (isErrorLike(result)) {
                            return setState({ status: LazyQueryStatus.Error, error: result })
                        }

                        setState({ status: LazyQueryStatus.Data, data: result })
                    },
                }

                localRequestPool.current.push(event)
                requests.next(event as unknown as Request<T>)

                return {
                    unsubscribe: () => {
                        // Cancel scheduled stream
                        cancelledRequests.add(event as unknown as Request<T>)

                        // Stop/cancel ongoing/started request stream
                        cancelStream.next(true)

                        localRequestPool.current = localRequestPool.current.filter(request => request !== event)
                    },
                }
            }, [])

            return { state, query }
        },
    }
}

// Export useParallelRequests hook with global request manager for all
// consumers.
const parallelRequestAPI = createUseParallelRequestsHook<unknown>()

export const useParallelRequests = parallelRequestAPI.query
export const useLazyParallelRequest = parallelRequestAPI.lazyQuery
