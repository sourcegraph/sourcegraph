import { Observable, isObservable } from 'rxjs'
import { type Subscribable } from 'sourcegraph'

/**
 * Converts a Sourcegraph {@link Subscribable} to an {@link Observable}.
 */
export function fromSubscribable<T>(value: Subscribable<T>): Observable<T> {
    if (isObservable(value)) {
        // type casting should be fine since we already know that
        // value is at least a Subscribable<T>
        return value as Observable<T>
    }
    return new Observable<T>(subscriber => value.subscribe(subscriber))
}
