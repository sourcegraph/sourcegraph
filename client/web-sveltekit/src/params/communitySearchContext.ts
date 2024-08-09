import type { ParamMatcher } from '@sveltejs/kit'

import { browser } from '$app/environment'
import { communities } from '$lib/search/communityPages'

const names: readonly string[] = communities

export const match: ParamMatcher = param => {
    return browser && window.context.sourcegraphDotComMode && names.includes(param.toLowerCase())
}
