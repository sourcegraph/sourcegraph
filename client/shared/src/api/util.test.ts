/* eslint-disable @typescript-eslint/no-unsafe-assignment */
/* eslint-disable @typescript-eslint/no-explicit-any */
import pTimeout from 'p-timeout'
import { Subject } from 'rxjs'

import { isAsyncIterable, isPromiseLike, isSubscribable, observableFromAsyncIterable, tryCatchPromise } from './util'

// TODO(sqs): see what else can be removed from this file after bext and vscode/jetbrains extensions are ported over

describe('tryCatchPromise', () => {
    test('returns a resolved promise with the synchronous result', async () =>
        expect(await tryCatchPromise(() => 1)).toBe(1))

    test('returns a resolved promise with the asynchronous result', async () =>
        expect(await tryCatchPromise(() => Promise.resolve(1))).toBe(1))

    const ERROR = new Error('x')
    test('returns a rejected promise with the synchronous exception', () => {
        const promise = tryCatchPromise(() => {
            throw ERROR
        })
        let rejected: any
        return promise
            .then(undefined, error => {
                rejected = error
            })
            .then(() => {
                expect(rejected).toBe(ERROR)
            })
    })

    test('returns a rejected promise with the asynchronous error', () => {
        const promise = tryCatchPromise(() => Promise.reject(ERROR))
        let rejected: any
        return promise
            .then(undefined, error => {
                rejected = error
            })
            .then(() => {
                expect(rejected).toBe(ERROR)
            })
    })
})

describe('isPromise', () => {
    test('returns true for promises', () =>
        expect(
            isPromiseLike(
                new Promise<any>(() => {
                    /* noop */
                })
            )
        ).toBe(true))
    test('returns false for non-promises', () => {
        expect(isPromiseLike(1)).toBe(false)
        expect(isPromiseLike({ then: 1 })).toBe(false)
        expect(isPromiseLike(new Subject<any>())).toBe(false)
    })
})

describe('isSubscribable', () => {
    test('returns true for subscribables', () => expect(isSubscribable(new Subject<any>())).toBe(true))
    test('returns false for non-subscribables', () => {
        expect(isSubscribable(1)).toBe(false)
        expect(isSubscribable({ subscribe: 1 })).toBe(false)
        expect(
            isSubscribable(
                new Promise<any>(() => {
                    /* noop */
                })
            )
        ).toBe(false)
    })
})

describe('isAsyncIterable', () => {
    test('returns true for AsyncIterables', () => {
        async function* provideHover() {
            yield 1
            await Promise.resolve()
            yield 2
            return
        }
        const providerResult = provideHover()

        expect(isAsyncIterable(providerResult)).toBe(true)
    })

    test('returns false for non-AsyncIterables', () => {
        expect(isAsyncIterable(1)).toBe(false)
        expect(
            isAsyncIterable(
                new Promise<any>(() => {
                    /* noop */
                })
            )
        ).toBe(false)
        expect(
            isAsyncIterable(
                (function* () {
                    yield 1
                    yield 2
                })()
            )
        ).toBe(false)
    })
})

describe('observableFromAsyncIterable', () => {
    test('result is a valid subscribable', () => {
        expect(
            isSubscribable(
                observableFromAsyncIterable(
                    (async function* () {
                        yield 1
                        await Promise.resolve()
                        yield 2
                        return 'return value'
                    })()
                )
            )
        ).toBe(true)
    })

    it('returned observable emits yielded values and return value', async () => {
        const observable = observableFromAsyncIterable(
            (async function* () {
                await Promise.resolve()
                yield 1
                yield 2
                yield 3
                yield 4
                yield 5
            })()
        )

        const values: number[] = []
        await new Promise<void>(complete => observable.subscribe({ next: value => values.push(value), complete }))
        expect(values).toStrictEqual([1, 2, 3, 4, 5])
    })

    it('stops emitting and throws AbortError on unsubscription', async () => {
        let abortErrorThrown = false
        async function* test() {
            await Promise.resolve()
            yield 1
            yield 2
            try {
                yield 3
            } catch {
                abortErrorThrown = true
            }
            yield 4
            yield 5
        }

        const observable = observableFromAsyncIterable(test())

        const collectedValues: number[] = []
        await new Promise<void>(resolve => {
            const subscription = observable.subscribe({
                next: value => {
                    collectedValues.push(value)
                    if (value === 3) {
                        unsubscribe()
                    }
                },
                error: resolve,
                complete: resolve,
            })
            function unsubscribe() {
                subscription.unsubscribe()
                resolve()
            }
        })

        expect(collectedValues).toStrictEqual([1, 2, 3])
        // Assert that not only has the observable stopped emitting, that an `AbortError` was thrown as well
        expect(abortErrorThrown).toBe(true)
    })

    it('throws iterator error', async () => {
        const observable = observableFromAsyncIterable(
            (async function* () {
                await Promise.resolve()
                yield 1
                yield 2
                yield 3
                throw new Error('oops')
            })()
        )

        const error = await pTimeout(
            new Promise(error => observable.subscribe({ error })),
            1000,
            'Expected observable to throw error'
        )

        expect(error).toStrictEqual(new Error('oops'))
    })
})
