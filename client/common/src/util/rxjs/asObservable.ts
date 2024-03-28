import { type Observable, isObservable, of, throwError } from 'rxjs'

/**
 * Calls a function and returns the result as an Observable.
 */
export function asObservable<T>(function_: () => Observable<T> | T): Observable<T> {
    try {
        const value = function_()
        if (isObservable(value)) {
            return value
        }
        return of(value)
    } catch (error) {
        return throwError(() => error)
    }
}
