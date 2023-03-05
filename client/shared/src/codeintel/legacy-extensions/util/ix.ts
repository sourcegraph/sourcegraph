import { Observable, Observer } from 'rxjs'

/**
 * An async generator that yields no values.
 */
export const noopAsyncGenerator = async function* <T>(): AsyncGenerator<T, void, undefined> {
    /* no-op */
}

export interface AbortError extends Error {
    name: 'AbortError'
}

/**
 * Creates an Error with name "AbortError"
 */
export function createAbortError(): AbortError {
    return Object.assign(new Error('Aborted'), { name: 'AbortError' as const })
}

/**
 * Convert an async iterator into an observable.
 *
 * @param factory A function returning the source iterator.
 */
export const observableFromAsyncIterator = <T>(factory: () => AsyncIterator<T>): Observable<T> =>
    new Observable((observer: Observer<T>) => {
        const iterator = factory()
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
                        next()
                        return
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
                iterator.throw(createAbortError()).catch(() => {
                    // ignore
                })
            }
        }
    })

/**
 * Modify an async iterable to return an ever-growing list of yielded values. Outputting a changing
 * list will overwrite the previously yielded results. The output generator does not output null
 * values.
 *
 * @param source The source iterable.
 */
export async function* concat<T>(source: AsyncIterable<T[] | null>): AsyncIterable<T[] | null> {
    let allValues: T[] = []
    for await (const values of source) {
        if (!values) {
            continue
        }
        allValues = allValues.concat(values)
        yield allValues
    }
}

/**
 * Converts a function returning a promise into an async generator yielding the
 * resolved value of that promise.
 *
 * @param func The promise function.
 */
export function asyncGeneratorFromPromise<P extends unknown[], R>(
    func: (...args: P) => Promise<R>
): (...args: P) => AsyncGenerator<R, void, unknown> {
    return async function* (...args: P): AsyncGenerator<R, void, unknown> {
        yield await func(...args)
    }
}
