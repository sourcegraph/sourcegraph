import { asyncScheduler, defer, from, Observable, OperatorFunction, Subscription } from 'rxjs'
import { concatAll, filter, mergeMap, observeOn, tap } from 'rxjs/operators'
import { isDefined, isInstanceOf } from '../../../../../shared/src/util/types'
import { MutationRecordLike, querySelectorAllOrSelf } from '../../util/dom'

interface View {
    element: HTMLElement
}

export type ViewWithSubscriptions<V extends View> = V & {
    /**
     * Maintains subscriptions to resources that should be freed when the view is removed.
     */
    subscriptions: Subscription
}

export type CustomSelectorFunction = (
    target: HTMLElement
) => ReturnType<ParentNode['querySelectorAll']> | HTMLElement[] | null | undefined

/**
 * Finds and resolves elements matched by a MutationObserver to views.
 *
 * @template V The type of view, such as a code view.
 */
export interface ViewResolver<V extends View> {
    /**
     * The element selector (used with {@link Window#querySelectorAll}) that matches candidate
     * elements to be passed to {@link ViewResolver#resolveView}.
     */
    selector: string | CustomSelectorFunction

    /**
     * Resolve an element matched by {@link ViewResolver#selector} to a view, or `null` if it's not
     * a a valid view upon further examination.
     */
    resolveView: (element: HTMLElement) => V | null
}

/**
 * Find all the views (e.g., code views) on a page using view resolvers (defined in
 * {@link CodeHost}).
 *
 * Emits every view that gets added as a {@link ViewWithSubscriptions},
 * and frees a view's resources when it gets removed from the page.
 *
 * At any given time, there can be any number of views on a page.
 *
 * @template V The type of view, such as a code view.
 */
export function trackViews<V extends View>(
    viewResolvers: ViewResolver<V>[]
): OperatorFunction<MutationRecordLike[], ViewWithSubscriptions<V>> {
    return mutations =>
        defer(() => {
            const viewStates = new Map<HTMLElement, ViewWithSubscriptions<V>>()
            return mutations.pipe(
                observeOn(asyncScheduler),
                concatAll(),
                // Inspect removed nodes for known views
                tap(({ removedNodes }) => {
                    for (const node of removedNodes) {
                        if (!(node instanceof HTMLElement)) {
                            continue
                        }
                        const view = viewStates.get(node)
                        if (view) {
                            view.subscriptions.unsubscribe()
                            viewStates.delete(node)
                            continue
                        }
                        for (const [viewElement, view] of viewStates.entries()) {
                            if (node.contains(viewElement)) {
                                view.subscriptions.unsubscribe()
                                viewStates.delete(viewElement)
                            }
                        }
                    }
                }),
                mergeMap(mutation =>
                    // Find all new code views within the added nodes
                    // (MutationObservers don't emit all descendant nodes of an addded node recursively)
                    from(mutation.addedNodes).pipe(
                        filter(isInstanceOf(HTMLElement)),
                        mergeMap(addedElement =>
                            from(viewResolvers).pipe(
                                mergeMap(({ selector, resolveView }) =>
                                    [...queryWithSelector(addedElement, selector)].map(
                                        (element): ViewWithSubscriptions<V> | null => {
                                            const view = resolveView(element)
                                            return (
                                                view && {
                                                    ...view,
                                                    subscriptions: new Subscription(),
                                                }
                                            )
                                        }
                                    )
                                ),
                                filter(isDefined),
                                filter(view => !viewStates.has(view.element)),
                                tap(view => {
                                    viewStates.set(view.element, view)
                                })
                            )
                        )
                    )
                )
            )
        })
}

/**
 * Find elements in the subtree of the target element using either a string selector or a custom selector function.
 */
export function queryWithSelector(
    target: HTMLElement,
    selector: string | CustomSelectorFunction
): ArrayLike<HTMLElement> & Iterable<HTMLElement> {
    if (typeof selector === 'string') {
        return querySelectorAllOrSelf<HTMLElement>(target, selector)
    }
    const selectorResult = selector(target) as NodeListOf<HTMLElement>
    return selectorResult || []
}

export type IntersectionObserverCallbackLike = (
    entries: Pick<IntersectionObserverEntry, 'target' | 'isIntersecting'>[],
    observer: Pick<IntersectionObserver, 'unobserve'>
) => void

export type IntersectionObserverLike = Pick<IntersectionObserver, 'observe' | 'unobserve' | 'disconnect'>

/**
 * An operator function that delays emitting views until they intersect with the viewport.
 *
 */
export function delayUntilIntersecting<T extends View>(
    options: IntersectionObserverInit,
    createIntersectionObserver = (
        callback: IntersectionObserverCallbackLike,
        options: IntersectionObserverInit
    ): IntersectionObserverLike => new IntersectionObserver(callback, options)
): OperatorFunction<ViewWithSubscriptions<T>, ViewWithSubscriptions<T>> {
    return views =>
        new Observable(viewObserver => {
            const subscriptions = new Subscription()
            const delayedViews = new Map<HTMLElement, ViewWithSubscriptions<T>>()
            const intersectionObserver = createIntersectionObserver((entries, observer) => {
                for (const entry of entries) {
                    const target = entry.target as HTMLElement
                    if (entry.isIntersecting && delayedViews.get(target)) {
                        viewObserver.next(delayedViews.get(target))
                        observer.unobserve(entry.target)
                        delayedViews.delete(target)
                    }
                }
            }, options)
            subscriptions.add(() => intersectionObserver.disconnect())
            subscriptions.add(
                views.subscribe(view => {
                    delayedViews.set(view.element, view)
                    intersectionObserver.observe(view.element)
                    view.subscriptions.add(() => {
                        delayedViews.delete(view.element)
                        intersectionObserver.unobserve(view.element)
                    })
                })
            )
            return subscriptions
        })
}
