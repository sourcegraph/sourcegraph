import { isEqual } from 'lodash'
import { type OperatorFunction, merge, combineLatest, of } from 'rxjs'
import { share, startWith, map, filter, delay, endWith, scan, takeUntil, last } from 'rxjs/operators'

export const LOADING = 'loading' as const

/**
 * An emission from a result provider.
 *
 * @template T The type of the result. Should typically include an empty value, or even an error type.
 */
export interface MaybeLoadingResult<T> {
    /**
     * Whether the result provider is currently getting a new result.
     */
    isLoading: boolean

    /**
     * The latest result.
     */
    result: T
}

/**
 * Maps a stream of MaybeLoadingResult (which contains both results and loading states) to a stream of clear
 * instructions on when to show a loader, results or nothing.
 *
 * @param loaderDelay The delay, in milliseconds, after which a loader should be shown if no results have been emitted.
 * @param emptyResultValue The value that represents the absence of results. This will be emitted, and also deep-compared to with `isEqual()`. Example: `null`, `[]`
 *
 * @template TResult The type of the provider result (without `TEmpty`).
 * @template TEmpty The type of the empty value, e.g. `null` or `[]`.
 */
export const emitLoading =
    <TResult, TEmpty>(
        loaderDelay: number,
        emptyResultValue: TEmpty
    ): OperatorFunction<MaybeLoadingResult<TResult | TEmpty>, TResult | TEmpty | typeof LOADING | undefined> =>
    source => {
        const sharedSource = source.pipe(
            // Prevent a loading indicator to be shown forever if the source completes without a result.
            endWith<Partial<MaybeLoadingResult<TResult | TEmpty>>>({ isLoading: false }),
            scan<Partial<MaybeLoadingResult<TResult | TEmpty>>, MaybeLoadingResult<TResult | TEmpty>>(
                (previous, current) => ({ ...previous, ...current }),
                { isLoading: true, result: emptyResultValue }
            ),
            share()
        )
        return merge(
            // `undefined` is used here as opposed to `emptyResultValue` to distinguish between "no result" and the time
            // between invocation and when a loader is shown.
            [undefined],
            // Show a loader if the provider is loading, has no result yet and hasn't emitted after LOADER_DELAY.
            // combineLatest() is used here to block on the loader delay.
            combineLatest([
                sharedSource.pipe(
                    // Consider the provider loading initially.
                    startWith({ isLoading: true, result: emptyResultValue })
                ),
                // Make sure LOADER_DELAY has passed since this token has been hovered
                // (no matter if the source has emitted already)
                of(null).pipe(
                    delay(loaderDelay),
                    // Stop and ignore the timer when the source Observable completes
                    takeUntil(sharedSource.pipe(last(null, null)))
                ),
            ]).pipe(
                // Show the loader when the provider is loading and has no result yet
                filter(([{ isLoading, result }]) => isLoading && isEqual(result, emptyResultValue)),
                map(() => LOADING)
            ),
            // Show the provider results (and no more loader) once the source emitted the first result or is no longer loading.
            sharedSource.pipe(
                filter(({ isLoading, result }) => !isLoading || !isEqual(result, emptyResultValue)),
                map(({ result }) => result)
            )
        )
    }
