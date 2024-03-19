import {
    type ProxyMarked,
    transferHandlers,
    releaseProxy,
    type TransferHandler,
    type Remote,
    proxyMarker,
} from 'comlink'
import { type Unsubscribable, Observable, type Observer, type PartialObserver, Subscription } from 'rxjs'
import { Subscribable } from 'sourcegraph'

import { hasProperty, AbortError } from '@sourcegraph/common'

import type { ProxySubscribable } from './extension/api/common'

/**
 * Tests whether a value is a WHATWG URL object.
 */
export const isURL = (value: unknown): value is URL =>
    typeof value === 'object' &&
    value !== null &&
    hasProperty('href')(value) &&
    hasProperty('toString')(value) &&
    typeof value.toString === 'function' &&
    // eslint-disable-next-line @typescript-eslint/no-base-to-string
    value.href === value.toString()

/**
 * Registers global comlink transfer handlers.
 * This needs to be called before using comlink.
 * Idempotent.
 */
export function registerComlinkTransferHandlers(): void {
    const urlTransferHandler: TransferHandler<URL, string> = {
        canHandle: isURL,
        serialize: url => [url.href, []],
        deserialize: urlString => new URL(urlString),
    }
    transferHandlers.set('URL', urlTransferHandler)
}

/**
 * Creates a synchronous Subscription that will unsubscribe the given proxied Subscription asynchronously.
 *
 * @param subscriptionPromise A Promise for a Subscription proxied from the other thread
 */
export const syncRemoteSubscription = (
    subscriptionPromise: Promise<Remote<Unsubscribable & ProxyMarked>>
): Subscription =>
    // We cannot pass the proxy subscription directly to Rx because it is a Proxy that looks like a function

    new Subscription(async () => {
        const subscriptionProxy = await subscriptionPromise
        await subscriptionProxy.unsubscribe()
        if (typeof subscriptionProxy[releaseProxy] === 'function') {
            subscriptionProxy[releaseProxy]()
        }
    })

/**
 * Creates a synchronous Subscription that will unsubscribe the given Promise<Subscription> asynchronously.
 *
 * @param subscriptionPromise A Promise for a Subscription
 */
export const syncPromiseSubscription = (subscriptionPromise: Promise<Unsubscribable>): Subscription =>
    new Subscription(() => subscriptionPromise.then(subscription => subscription.unsubscribe()))

/**
 * Runs f and returns a resolved promise with its value or a rejected promise with its exception,
 * regardless of whether it returns a promise or not.
 */
export const tryCatchPromise = async <T>(function_: () => T | Promise<T>): Promise<T> => function_()

/**
 * Reports whether value is a Promise.
 */
export const isPromiseLike = (value: unknown): value is PromiseLike<unknown> =>
    typeof value === 'object' && value !== null && hasProperty('then')(value) && typeof value.then === 'function'

/**
 * Reports whether value is a {@link Subscribable}.
 */
export const isSubscribable = (value: unknown): value is Subscribable<unknown> =>
    typeof value === 'object' &&
    value !== null &&
    hasProperty('subscribe')(value) &&
    typeof value.subscribe === 'function'

/**
 * Reports whether the value is an AsyncIterable
 */
export const isAsyncIterable = (value: unknown): value is AsyncIterable<unknown> =>
    typeof value === 'object' && value !== null && typeof (value as any)[Symbol.asyncIterator] === 'function'

/**
 * Convert an async iterable into an observable.
 *
 * @param iterable The source iterable.
 */
export const observableFromAsyncIterable = <T>(iterable: AsyncIterable<T>): Observable<T> =>
    new Observable((observer: Observer<T>) => {
        const iterator = iterable[Symbol.asyncIterator]()
        let unsubscribed = false
        let iteratorDone = false
        function next(): void {
            iterator.next().then(
                result => {
                    if (unsubscribed) {
                        return
                    }
                    if (result.done) {
                        iteratorDone = true
                        observer.complete()
                    } else {
                        observer.next(result.value)
                        return next()
                    }
                },
                error => {
                    observer.error(error)
                }
            )
        }
        next()
        return () => {
            unsubscribed = true
            if (!iteratorDone && iterator.throw) {
                iterator.throw(new AbortError()).catch(() => {
                    // ignore
                })
            }
        }
    })

/**
 * Promisifies method calls and objects if specified, throws otherwise if there is no stub provided
 * NOTE: it does not handle ProxyMethods and callbacks yet
 * NOTE2: for testing purposes only!!
 */
export const pretendRemote = <T>(object: Partial<T>): Remote<T> =>
    new Proxy(object, {
        get: (a, property) => {
            if (property === 'then') {
                // Promise.resolve(pretendRemote(..)) checks if this is a Promise
                // we will let it know that no, this is not a Promise
                return undefined
            }
            if (property in a) {
                if (typeof (a as any)[property] !== 'function') {
                    return Promise.resolve((a as any)[property])
                }

                return (...args: any[]) => Promise.resolve((a as any)[property](...args))
            }
            throw new Error(`unspecified property in the stub: "${property.toString()}"`)
        },
    }) as unknown as Remote<T>

/**
 * For proxySubscribables to be passed as stubs to pretendRemote.
 *
 * In unit tests, callers of `proxySubscribable` and `wrapRemoteObservable` will actually
 * be on the same thread, so comlink won't be involved to intercept symbol methods (e.g. [releaseProxy]).
 * We have to add them ourselves to prevent TypeErrors when unsubscribing from proxySubscribables.
 */
export const pretendProxySubscribable = <T>(subscribable: Subscribable<T>): ProxySubscribable<T> => ({
    [proxyMarker]: true,
    subscribe(observer): Unsubscribable & ProxyMarked {
        const subscription = subscribable.subscribe(observer as unknown as PartialObserver<T>)

        return {
            [proxyMarker]: true,
            unsubscribe: () => subscription.unsubscribe(),
            [releaseProxy]: () => undefined,
        } as Unsubscribable & ProxyMarked
    },
})
