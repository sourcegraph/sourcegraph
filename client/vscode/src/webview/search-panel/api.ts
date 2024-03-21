import { of } from 'rxjs'

import { proxySubscribable } from '@sourcegraph/shared/src/api/extension/api/common'

import type { SearchPanelAPI } from '../../contract'

export const searchPanelAPI: SearchPanelAPI = {
    ping: () => {
        console.log('ping called')
        return proxySubscribable(of('pong'))
    },
    focusSearchBox: () => {
        // Call dynamic `focusSearchBox`.
        focusSearchBox()
    },
}
let focusSearchBox = (): void => {
    // Initially a noop. Waiting for search box init.
}

// TODO move to api.ts file
export const setFocusSearchBox = (replacementFocusSearchBox: (() => void) | null): void => {
    focusSearchBox = replacementFocusSearchBox || (() => {})
}
