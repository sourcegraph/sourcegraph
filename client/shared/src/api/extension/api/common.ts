import { Remote, ProxyMarked, proxy, proxyMarker, UnproxyOrClone } from 'comlink'
import { Observer, Subscribable, Unsubscribable } from 'rxjs'

/**
 * A Subscribable that can be exposed by comlink to the other thread.
 * Only allows full object Observers to avoid complex type checking against proxies.
 */
export interface ProxySubscribable<T> extends ProxyMarked {
    subscribe(observer: Remote<Observer<T> & ProxyMarked>): Unsubscribable & ProxyMarked
}

/**
 * Wraps a given Subscribable so that it is exposed by comlink to the other thread.
 *
 * @param subscribable A normal Subscribable (from this thread)
 */
export const proxySubscribable = <T>(subscribable: Subscribable<T>): ProxySubscribable<T> => ({
    [proxyMarker]: true,
    subscribe(observer): Unsubscribable & ProxyMarked {
        return proxy(
            // Don't pass the proxy to Rx directly because it will try to
            // access Symbol properties that cannot be proxied
            subscribable.subscribe({
                next: value => {
                    // eslint-disable-next-line @typescript-eslint/no-floating-promises
                    observer.next(value as UnproxyOrClone<T>)
                },
                error: error => {
                    // Only pass a few well-known Error properties
                    // TODO should pass all properties serialized recursively, best handled on comlink level
                    // eslint-disable-next-line @typescript-eslint/no-floating-promises
                    observer.error(error && { message: error.message, name: error.name, stack: error.stack })
                },
                complete: () => {
                    // eslint-disable-next-line @typescript-eslint/no-floating-promises
                    observer.complete()
                },
            })
        )
    },
})
