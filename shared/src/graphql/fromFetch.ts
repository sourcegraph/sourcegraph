// TODO: Taken from https://github.com/ReactiveX/rxjs/pull/5306. This should be switched back to rxjs fromFetch once the PR is merged.
import { Observable, ObservableInput, from, Subscription } from 'rxjs'

export function fromFetch(input: string | Request, init?: RequestInit): Observable<Response>

export function fromFetch<T>(
    input: string | Request,
    init: RequestInit | undefined,
    selector: (response: Response) => ObservableInput<T>
): Observable<T>

/**
 * Uses [the Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API) to
 * make an HTTP request.
 *
 * **WARNING** Parts of the fetch API are still experimental. `AbortController` is
 * required for this implementation to work and use cancellation appropriately.
 *
 * Will automatically set up an internal [AbortController](https://developer.mozilla.org/en-US/docs/Web/API/AbortController)
 * in order to teardown the internal `fetch` when the subscription tears down.
 *
 * If a `signal` is provided via the `init` argument, it will behave like it usually does with
 * `fetch`. If the provided `signal` aborts, the error that `fetch` normally rejects with
 * in that scenario will be emitted as an error from the observable.
 *
 * ### Basic Use
 *
 * ```ts
 * import { of } from 'rxjs';
 * import { fromFetch } from 'rxjs/fetch';
 * import { switchMap, catchError } from 'rxjs/operators';
 *
 * const data$ = fromFetch('https://api.github.com/users?per_page=5').pipe(
 *  switchMap(response => {
 *    if (response.ok) {
 *      // OK return data
 *      return response.json();
 *    } else {
 *      // Server is returning a status requiring the client to try something else.
 *      return of({ error: true, message: `Error ${response.status}` });
 *    }
 *  }),
 *  catchError(err => {
 *    // Network or other error, handle appropriately
 *    console.error(err);
 *    return of({ error: true, message: err.message })
 *  })
 * );
 *
 * data$.subscribe({
 *  next: result => console.log(result),
 *  complete: () => console.log('done')
 * })
 * ```
 *
 * @param input The resource you would like to fetch. Can be a url or a request object.
 * @param init A configuration object for the fetch.
 * [See MDN for more details](https://developer.mozilla.org/en-US/docs/Web/API/WindowOrWorkerGlobalScope/fetch#Parameters)
 * @returns An Observable, that when subscribed to performs an HTTP request using the native `fetch`
 * function. The {@link Subscription} is tied to an `AbortController` for the the fetch.
 */
export function fromFetch<T>(
    input: string | Request,
    init?: RequestInit,
    selector?: (response: Response) => ObservableInput<T>
): Observable<Response | T> {
    return new Observable<Response | T>(subscriber => {
        const controller = new AbortController()
        const signal = controller.signal
        let abortable = true
        let unsubscribed = false

        const subscription = new Subscription()
        subscription.add(() => {
            unsubscribed = true
            if (abortable) {
                controller.abort()
            }
        })

        let perSubscriberInit: RequestInit
        if (init) {
            // If a signal is provided, just have it teardown. It's a cancellation token, basically.
            if (init.signal) {
                if (init.signal.aborted) {
                    controller.abort()
                } else {
                    const outerSignal = init.signal
                    const outerSignalHandler = (): void => {
                        if (!signal.aborted) {
                            controller.abort()
                        }
                    }
                    outerSignal.addEventListener('abort', outerSignalHandler)
                    subscription.add(() => outerSignal.removeEventListener('abort', outerSignalHandler))
                }
            }
            // init cannot be mutated or reassigned as it's closed over by the
            // subscriber callback and is shared between subscribers.
            perSubscriberInit = { ...init, signal }
        } else {
            perSubscriberInit = { signal }
        }

        fetch(input, perSubscriberInit)
            .then(response => {
                if (selector) {
                    subscription.add(
                        from(selector(response)).subscribe(
                            value => subscriber.next(value),
                            error => {
                                abortable = false
                                if (!unsubscribed) {
                                    // Only forward the error if it wasn't an abort.
                                    subscriber.error(error)
                                }
                            },
                            () => {
                                abortable = false
                                subscriber.complete()
                            }
                        )
                    )
                } else {
                    abortable = false
                    subscriber.next(response)
                    subscriber.complete()
                }
            })
            .catch(error => {
                abortable = false
                if (!unsubscribed) {
                    // Only forward the error if it wasn't an abort.
                    subscriber.error(error)
                }
            })

        return subscription
    })
}
