import { type Readable, writable, readonly } from 'svelte/store'

/**
 * Successful result of a promise.
 */
interface ResultSuccess<T> {
    value: T
    error: null
    pending: false
}

/**
 * Rejected result of a promise.
 */
interface ResultError<E> {
    value: null
    error: E
    pending: false
}

/**
 * Pending result of a promise. `value` and `error` can contain the
 * latest resolved value or rejection error.
 */
interface ResultPending<T, E> {
    value: T | null
    error: E | null
    pending: true
}

type Result<T, E = Error> = ResultSuccess<T> | ResultError<E> | ResultPending<T, E>

interface PromiseStore<D, E = Error> extends Readable<Result<D | null, E>> {
    /**
     * Sets the passed promise as the current promise and tracks its status.
     * Does nothing if the same promise as the current one is passed. The argument
     * is optional to make it easier to work with optional data coming from loaders.
     */
    set: (promise?: PromiseLike<D> | null) => void
}

/**
 * Returns multiple stores to track the promises state, resolved value and rejection error.
 * The store ensures that `value` is updated with latest resolved promise.
 */
export function createPromiseStore<D, E = Error>(): PromiseStore<D, E> {
    let currentPromise: PromiseLike<D> | null | undefined

    const resultStore = writable<Result<D | null, E>>({ value: null, error: null, pending: true })

    function resolve(promise?: PromiseLike<D> | null) {
        currentPromise = promise
        if (!promise) {
            resultStore.set({ value: null, error: null, pending: false })
            return
        }

        resultStore.update($result => ({ ...$result, pending: true }))
        promise.then(
            result => {
                if (currentPromise === promise) {
                    resultStore.update(() => ({ value: result, error: null, pending: false }))
                }
            },
            error => {
                if (currentPromise === promise) {
                    resultStore.update(() => ({ value: null, error: error, pending: false }))
                }
            }
        )
    }

    resolve(currentPromise)

    return {
        ...readonly(resultStore),
        set: promise => {
            if (promise !== currentPromise) {
                resolve(promise)
            }
        },
    }
}

/**
 * Returns a store that publishes updates when the promise is resolved or rejected.
 */
export function toReadable<D, E = Error>(promise: PromiseLike<D>): Readable<Result<D, E>> {
    const resultStore = writable<Result<D, E>>({ value: null, error: null, pending: true })
    promise.then(
        result => {
            resultStore.set({ value: result, error: null, pending: false })
        },
        error => {
            resultStore.set({ value: null, error: error, pending: false })
        }
    )
    return resultStore
}
