import { isObject } from 'lodash'
import { type Observable, from } from 'rxjs'
import { map } from 'rxjs/operators'

import { isDefined } from '@sourcegraph/common'

import type { MaybeLoadingResult } from './loading'

/**
 * Checks if the given value is thenable.
 */
const isPromiseLike = (value: unknown): value is PromiseLike<unknown> =>
    isObject(value) && typeof (value as PromiseLike<unknown>).then === 'function'

/**
 * Converts a provider result, which can be a continuously-updating, maybe-loading Subscribable, or a Promise for a
 * single result, to the same type.
 */
export const toMaybeLoadingProviderResult = <T>(
    value: Observable<MaybeLoadingResult<T>> | PromiseLike<T>
): Observable<MaybeLoadingResult<T>> =>
    isPromiseLike(value) ? from(value).pipe(map(result => ({ isLoading: false, result }))) : value

/**
 * Returns a function that returns `true` if the given `key` of the object is not `null` or `undefined`.
 *
 * I ❤️ TypeScript.
 */
export const propertyIsDefined =
    <T extends object, K extends keyof T>(key: K) =>
    (value: T): value is T & { [k in K]-?: NonNullable<T[k]> } =>
        isDefined(value[key])

/**
 * Scrolls a DOMRect based position to the center if it is out of view.
 * Does nothing if the element is in view. If the rectangle's height is larger
 * than the scroll containers height, the rectangle's top is scrolled into the
 * center instead.
 *
 * @param container The scrollable container (that has `overflow: auto`)
 * @param content The content child that is being scrolled
 * @param targetRectangle The DOMRect that should be scrolled into view
 */
export const scrollRectangleIntoCenterIfNeeded = (
    container: HTMLElement,
    content: HTMLElement,
    targetRectangle: Pick<DOMRect, 'top' | 'bottom' | 'height'>
): void => {
    const containerRectangle = container.getBoundingClientRect()
    if (targetRectangle.top >= containerRectangle.bottom || targetRectangle.bottom <= containerRectangle.top) {
        const contentRectangle = content.getBoundingClientRect()
        const topOffset = targetRectangle.height > containerRectangle.height ? 0 : targetRectangle.height / 2

        container.scrollTop = targetRectangle.top - contentRectangle.top - containerRectangle.height / 2 + topOffset
    }
}

/**
 * Returns a curried function that returns `true` if `e1` and `e2` overlap.
 */
export const elementOverlaps =
    (element1: HTMLElement) =>
    (element2: HTMLElement): boolean => {
        const rectangle1 = element1.getBoundingClientRect()
        const rectangle2 = element2.getBoundingClientRect()
        return !(
            rectangle1.right < rectangle2.left ||
            rectangle1.left > rectangle2.right ||
            rectangle1.bottom < rectangle2.top ||
            rectangle1.top > rectangle2.bottom
        )
    }
