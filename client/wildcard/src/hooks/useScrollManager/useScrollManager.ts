import { useEffect, type RefObject } from 'react'

import { debounce } from 'lodash'
import { useLocation, useNavigationType } from 'react-router-dom'

import { logger } from '@sourcegraph/common'

import {
    type MutationObserverError,
    type MutationObserverPromise,
    mutationObserverWithTimeout,
} from './mutationObserverWithTimeout'

/**
 * Object containing maps of saved scroll locations for each path name, which are keyed by the scrollable container they belong to.
 * This will allow us to easily support scroll preservation in multiple containers, even when they are used in the same page.
 */
const SAVED_SCROLL_POSITIONS: { [k: string]: Map<string, number> } = {}

/**
 * Object containing maps of cached scroll attempts for each path name, which are keyed by the scrollable container they belong to.
 * This can be used to cancel a scroll attempt if a new attempt comes in for the same path.
 */
const CACHED_SCROLL_ATTEMPTS: { [k: string]: Map<string, MutationObserverPromise> } = {}

/** The length of the timeout in milliseconds until we stop trying to scroll. */
const SCROLL_RETRY_TIMEOUT = 3000

/**
 * This hook will preserve the scroll state of a provided container between history changes ("forward" and "back" navigation, specifically).
 *
 * @param containerKey A unique key to identify the container where scrolling will be managed. Usually the component name.
 * @param containerRef A React ref object of the container where scrolling will be managed.
 */
export function useScrollManager(containerKey: string, containerRef: RefObject<HTMLElement>): void {
    const location = useLocation()
    const navigationType = useNavigationType()

    // Set up the maps for this containerKey if they haven't been created yet
    useEffect(() => {
        if (!SAVED_SCROLL_POSITIONS[containerKey]) {
            SAVED_SCROLL_POSITIONS[containerKey] = new Map<string, number>()
        }

        if (!CACHED_SCROLL_ATTEMPTS[containerKey]) {
            CACHED_SCROLL_ATTEMPTS[containerKey] = new Map<string, MutationObserverPromise>()
        }
    }, [containerKey])

    // Attach scroll listener, make sure to detach on unmount
    useEffect(() => {
        const container = containerRef.current

        const saveScrollPosition = debounce(() => {
            if (container) {
                SAVED_SCROLL_POSITIONS[containerKey].set(location.pathname, container.scrollTop)
            }
        }, 200)

        container?.addEventListener('scroll', saveScrollPosition)

        return () => container?.removeEventListener('scroll', saveScrollPosition)
    }, [location.pathname, containerKey, containerRef])

    // Handle changes to pathname and try to scroll to the saved position at that path, if it exists
    useEffect(() => {
        const { pathname } = location

        // Cancel any existing cached scroll attempts for this pathname
        if (CACHED_SCROLL_ATTEMPTS[containerKey].has(pathname)) {
            CACHED_SCROLL_ATTEMPTS[containerKey].get(pathname)?.cancel()
            CACHED_SCROLL_ATTEMPTS[containerKey].delete(pathname)
        }

        // Attempt a scroll if we have a saved position for the pathname; if the scroll doesn't work, set up an observer to
        // retry up until a given timeout
        if (navigationType === 'POP' && SAVED_SCROLL_POSITIONS[containerKey].has(pathname)) {
            const scrollPosition = SAVED_SCROLL_POSITIONS[containerKey].get(pathname) ?? 0

            const attemptScroll = (): boolean => {
                containerRef.current?.scrollTo(0, scrollPosition)
                return containerRef.current?.scrollTop === scrollPosition
            }

            if (!attemptScroll()) {
                const promiseToCache = mutationObserverWithTimeout(
                    attemptScroll,
                    SCROLL_RETRY_TIMEOUT,
                    containerRef.current
                )
                CACHED_SCROLL_ATTEMPTS[containerKey].set(pathname, promiseToCache)

                promiseToCache
                    .then(() =>
                        // Delete the cached scroll for this path once it resolves
                        CACHED_SCROLL_ATTEMPTS[containerKey].delete(pathname)
                    )
                    .catch((error: MutationObserverError) => {
                        if (!error.cancelled) {
                            logger.error(
                                `Failed to scroll to position '${scrollPosition}' for pathname '${pathname}' in container '${containerKey}'.`
                            )
                        }
                    })
            }
        } else if (navigationType === 'PUSH') {
            // In the case of pushing new history (e.g. clicking a navigation link), make sure we always start at the top of the container
            containerRef.current?.scrollTo(0, 0)
        }

        // This should only run when `pathname`/`action` change; ignore changes to `containerKey` or `containerRef` as those should not trigger a scroll restoration
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [location.pathname, navigationType])
}
