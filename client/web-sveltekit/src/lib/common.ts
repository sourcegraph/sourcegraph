// We want to limit the number of imported modules as much as possible
/* eslint-disable no-restricted-imports */
export type { ErrorLike } from '@sourcegraph/common/src/errors/types'
export { isErrorLike } from '@sourcegraph/common/src/errors/utils'
export { createAggregateError, asError } from '@sourcegraph/common/src/errors/errors'
export { memoizeObservable, resetAllMemoizationCaches } from '@sourcegraph/common/src/util/rxjs/memoizeObservable'
export { encodeURIPathComponent } from '@sourcegraph/common/src/util/url'
export { pluralize, numberWithCommas } from '@sourcegraph/common/src/util/strings'
export { renderMarkdown, type RenderMarkdownOptions } from '@sourcegraph/common/src/util/markdown/markdown'
export { highlightNodeMultiline, highlightNode } from '@sourcegraph/common/src/util/highlightNode'
export { logger } from '@sourcegraph/common/src/util/logger'
export { isSafari } from '@sourcegraph/common/src/util/browserDetection'
export { isExternalLink, type LineOrPositionOrRange, SourcegraphURL } from '@sourcegraph/common/src/util/url'
export { parseJSONCOrError } from '@sourcegraph/common/src/util/jsonc'
export {
    isWindowsPlatform,
    isMacPlatform,
    isLinuxPlatform,
    getPlatform,
} from '@sourcegraph/common/src/util/browserDetection'
export { dirname, basename } from '@sourcegraph/common/src/util/path'
export { isDefined } from '@sourcegraph/common/src/types/utils'

let highlightingLoaded = false

export function loadMarkdownSyntaxHighlighting(): void {
    if (!highlightingLoaded) {
        highlightingLoaded = true

        import('@sourcegraph/common/src/util/markdown/contributions')
            .then(({ registerHighlightContributions }) => registerHighlightContributions()) // no way to unregister these
            .catch(() => {})
    }
}
