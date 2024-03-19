import { useMemo, useState, useCallback } from 'react'

import { compact, head } from 'lodash'
import { combineLatest, concat, EMPTY, type Observable, of, ReplaySubject, zip } from 'rxjs'
import { catchError, map, switchMap, tap, debounceTime } from 'rxjs/operators'

import { asError } from '@sourcegraph/common'
import { useEventObservable } from '@sourcegraph/wildcard'

/**
 * Configuration used by `useInputValidation`
 */
export interface ValidationOptions {
    /**
     * Initial value of the input element.
     *
     * If an initial value is provided, it will be validated
     * against built-in and provided validators as soon
     * as the input element is rendered.
     */
    initialValue?: string

    /**
     * Optional array of synchronous input validators.
     *
     * If there's no problem with the input, return undefined. Else,
     * return with the reason the input is invalid.
     */
    synchronousValidators?: ((value: string) => ValidationResult)[]

    /**
     * Optional array of asynchronous input validators. These must return
     * observables created with `fromFetch` for easy cancellation in `switchMap`.
     *
     * If there's no problem with the input, emit undefined. Else,
     * return with the reason the input is invalid.
     */
    asynchronousValidators?: ((value: string) => Observable<ValidationResult>)[]
}

type ValidationResult = string | undefined

export type InputValidationState = { value: string } & (
    | { kind: 'NOT_VALIDATED' | 'LOADING' | 'VALID' }
    | { kind: 'INVALID'; reason: string }
)

/**
 * An event that updates input element state and (optionally) triggers validation.
 */
interface InputValidationEvent {
    /** The value to set input state to */
    value: string

    /** Whether to validate the new value */
    validate?: boolean
}

/**
 * Minimal interface of an HTMLElement that implements the Constraint validation API
 */
type ValidatingHTMLElement = Pick<HTMLInputElement, 'checkValidity' | 'setCustomValidity' | 'validationMessage'>

/**
 * React hook to manage validation of a single form input field.
 * `useInputValidation` helps with coordinating the constraint validation API
 * and custom synchronous and asynchronous validators.
 *
 * ### Limitations:
 * - If you set state when the input element is not rendered, the latest value
 * cannot be validated, since the bulk of validation is done by calling
 * Constrant Validation API methods on the input element.
 *
 * @param options Config object that declares sync + async validators
 */
export function useInputValidation(
    options: ValidationOptions
): [
    InputValidationState,
    (eventOrValue: React.ChangeEvent<HTMLInputElement> | string) => void,
    (inputElement: ValidatingHTMLElement | null) => void,
    (override: InputValidationEvent) => void
] {
    const [inputState, setInputState] = useState<InputValidationState>({
        kind: 'NOT_VALIDATED',
        value: options.initialValue ?? '',
    })

    // We use a ref callback instead of a mutable ref object so we have a
    // 'notifier' observable that emits when the input reference changes.
    // This is important because the input element can be conditionally rendered,
    // and we can only validate through the Constraint validation API when we
    // have a reference to an HTMLElement. See this case when a consumer specifies
    // an initial value while the input element isn't rendered yet:
    //
    // values:     initialValue - - - - -
    // inputRefs:  null - - - - - - element
    // validation: - - - - - - - - - - x
    //
    // With a ref object (no validation when element is rendered, false negative on initial validation):
    // values:     initialValue - - - - -
    // inputRefs:  null - - - - - - element
    // validation: x - - - - - - - - - -

    // The validation pipeline is subscribed to after initial render (`useEffect` in `useObservable`),
    // so after the ref callback is called. Buffer 1 emission to counteract this.
    const inputReferences = useMemo(() => new ReplaySubject<ValidatingHTMLElement | null>(1), [])
    const nextInputReference = useCallback(
        (input: ValidatingHTMLElement | null) => {
            // React calls ref callbacks when the element is rendered (calls with the element)
            // and when it leaves the DOM (calls with null). Unless this changes, we don't have to
            // track the input reference ourselves or use `distinctUntilChanged`; `inputReferences`
            // already only emits when the input reference changes
            inputReferences.next(input)
        },
        [inputReferences]
    )

    const validationPipeline = useMemo(
        () => createValidationPipeline(options, inputReferences, setInputState),
        [options, inputReferences]
    )

    const [nextInputValidationEvent] = useEventObservable(validationPipeline)

    // "Adapter" for React change events to input validation events
    const nextChangeEvent = useCallback(
        (eventOrValue: React.ChangeEvent<HTMLInputElement> | string): void => {
            let value
            if (typeof eventOrValue === 'string') {
                value = eventOrValue
            } else {
                eventOrValue.preventDefault()
                value = eventOrValue.target.value
            }
            // Always validate on change events
            nextInputValidationEvent({ value, validate: true })
        },
        [nextInputValidationEvent]
    )

    // "Adapter" for consumer overrides to input validation events
    const overrideState = useCallback(
        (inputValidationEvent: InputValidationEvent) => {
            nextInputValidationEvent(inputValidationEvent)
        },
        [nextInputValidationEvent]
    )

    return [inputState, nextChangeEvent, nextInputReference, overrideState]
}

/**
 * Derives className based on validation state. Use with `useInputValidation`.
 *
 * @param inputState
 */
export function deriveInputClassName(inputState: InputValidationState): string {
    if (inputState.kind === 'LOADING' || inputState.kind === 'NOT_VALIDATED') {
        return ''
    }
    return inputState.kind === 'INVALID' ? 'is-invalid' : 'is-valid'
}

export const VALIDATION_DEBOUNCE_TIME = 500

/**
 * Exported for testing
 */
export function createValidationPipeline(
    { asynchronousValidators, synchronousValidators, initialValue }: ValidationOptions,
    inputReferences: Observable<ValidatingHTMLElement | null>,
    onValidationUpdate: (
        validationState: InputValidationState | ((validationState: InputValidationState) => InputValidationState)
    ) => void
) {
    return (inputValidationEvents: Observable<InputValidationEvent>): Observable<unknown> =>
        // Emit the latest input validation event along with the latest input element reference whenever
        // either observable emits.
        combineLatest([
            // Validate immediately if the user has provided an initial input value
            concat(
                initialValue !== undefined ? of({ value: initialValue, validate: true }) : EMPTY,
                inputValidationEvents
            ),
            inputReferences,
        ]).pipe(
            tap(([{ value, validate }, inputReference]) => {
                inputReference?.setCustomValidity('')
                onValidationUpdate({ value, kind: validate && inputReference ? 'LOADING' : 'NOT_VALIDATED' })
            }),
            // Debounce everything.
            // This is to allow immediate validation on type but at the same time not flag invalid input as it's being typed.
            debounceTime(VALIDATION_DEBOUNCE_TIME),
            switchMap(([{ value, validate }, inputReference]) => {
                // if the input element isn't rendered right now, we can't/shouldn't validate input.
                if (!inputReference || !validate) {
                    return EMPTY
                }

                // check validity (synchronous)
                const valid = inputReference.checkValidity()
                if (!valid) {
                    onValidationUpdate({
                        value,
                        kind: 'INVALID',
                        reason: inputReference.validationMessage ?? '',
                    })
                    return EMPTY
                }

                // check custom sync validators
                const syncReason = head(compact(synchronousValidators?.map(validator => validator(value))))
                if (syncReason) {
                    inputReference.setCustomValidity(syncReason)
                    onValidationUpdate({
                        value,
                        kind: 'INVALID',
                        reason: syncReason,
                    })
                    return EMPTY
                }

                if (!asynchronousValidators || asynchronousValidators.length === 0) {
                    // clear possible custom sync validation error from previous value
                    inputReference.setCustomValidity('')
                    onValidationUpdate({
                        value,
                        kind: 'VALID',
                    })
                    return EMPTY
                }

                // check async validators
                return zip(...(asynchronousValidators?.map(validator => validator(value)) ?? [])).pipe(
                    map(values => head(compact(values))),
                    tap(reason => {
                        inputReference.setCustomValidity(reason ?? '')
                        onValidationUpdate(reason ? { kind: 'INVALID', value, reason } : { kind: 'VALID', value })
                    })
                )
            }),
            catchError(error => {
                onValidationUpdate(({ value }) => ({
                    value,
                    kind: 'INVALID',
                    reason: asError(error).message || 'Unknown error',
                }))
                return EMPTY
            })
        )
}
