import { asapScheduler, type ObservableInput, Observable, of, zip, from, Subscription } from 'rxjs'

/**
 * Like {@link combineLatest}, except that it does not wait for all Observables to emit before emitting an initial
 * value. It emits whenever any of the source Observables emit.
 *
 * If {@link defaultValue} is provided, it will be used to represent any source Observables
 * that have not yet emitted in the emitted array. If it is not provided, source Observables
 * that have not yet emitted will not be represented in the emitted array.
 *
 * Also unlike {@link combineLatest}, if the source Observables array is empty, it emits an empty array and
 * completes.
 *
 * This behavior is useful for the common pattern of combining providers: we don't want to block on the slowest
 * provider for the initial emission, and an empty array of providers should yield an empty array (instead of
 * yielding an Observable that never completes).
 *
 * @see {@link combineLatest}
 *
 * @todo Consider renaming this to combineProviders and making it also catchError from each Observable (and return
 * the error as a value).
 *
 * @param observables The source Observables.
 * @param defaultValue The value to emit for a source Observable if it has not yet emitted a value by the time
 * another Observable has emitted a value.
 * @returns An Observable of an array of the most recent values from each input Observable (or
 * {@link defaultValue}).
 */
export function combineLatestOrDefault<T>(observables: ObservableInput<T>[], defaultValue?: T): Observable<T[]> {
    switch (observables.length) {
        case 0: {
            // No source observables: emit an empty array and complete
            return of([])
        }
        case 1: {
            // Only one source observable: no need to handle emission accumulation or default values
            return zip(...observables)
        }
        default: {
            return new Observable<T[]>(subscriber => {
                // The array of the most recent values from each input Observable.
                // If a source Observable has not yet emitted a value, it will be represented by the
                // defaultValue (if provided) or not at all (if not provided).
                const values: T[] = defaultValue !== undefined ? observables.map(() => defaultValue) : []

                // Whether the emission of the values array has been scheduled
                let scheduled = false
                let scheduledWork: Subscription | undefined
                // The number of source Observables that have not yet completed
                // (so that we know when to complete the output Observable)
                let activeObservables = observables.length

                // When everything is done, clean up the values array
                subscriber.add(() => {
                    values.length = 0
                })

                // Subscribe to each source Observable. The index of the source Observable is used to
                // keep track of the most recent value from that Observable in the values array.
                for (let index = 0; index < observables.length; index++) {
                    subscriber.add(
                        from(observables[index]).subscribe({
                            next: value => {
                                values[index] = value
                                if (activeObservables === 1) {
                                    // If only one source Observable is active, emit the values array immediately
                                    // Abort any scheduled emission
                                    scheduledWork?.unsubscribe()
                                    scheduled = false
                                    subscriber.next(values.slice())
                                } else if (!scheduled) {
                                    scheduled = true
                                    // Use asapScheduler to emit the values array, so that all
                                    // next values that are emitted at the same time are emitted together.
                                    // This makes tests (using expectObservable) easier to write.
                                    scheduledWork = asapScheduler.schedule(() => {
                                        if (!subscriber.closed) {
                                            subscriber.next(values.slice())
                                            scheduled = false
                                            if (activeObservables === 0) {
                                                subscriber.complete()
                                            }
                                        }
                                    })
                                }
                            },
                            error: error => subscriber.error(error),
                            complete: () => {
                                activeObservables--
                                if (activeObservables === 0 && !scheduled) {
                                    subscriber.complete()
                                }
                            },
                        })
                    )
                }

                // When everything is done, clean up the values array
                return () => {
                    values.length = 0
                }
            })
        }
    }
}
