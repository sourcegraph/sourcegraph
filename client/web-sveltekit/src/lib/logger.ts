// We want to limit the number of imported modules as much as possible
/* eslint-disable no-restricted-imports */

import { onMount } from 'svelte'

// Dev build breaks if this import is moved to `$lib/web` ¯\_(ツ)_/¯
import type { EventLogger } from '@sourcegraph/web/src/tracking/eventLogger'

import { PUBLIC_ENABLE_EVENT_LOGGER } from '$env/static/public'

/**
 * Can only be called during component initialization. It logs a view event when
 * the component is mounted (and event logging is enabled).
 */
export function logViewEvent(...args: Parameters<EventLogger['logViewEvent']>): void {
    if (PUBLIC_ENABLE_EVENT_LOGGER) {
        onMount(() => {
            // TODO: Implement event logging
            console.log('logViewEvent', args)
        })
    }
}
