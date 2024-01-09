export { formatRepositoryStarCount } from '@sourcegraph/branded/src/search-ui/util/stars'
export { limitHit, sortBySeverity, getProgressText } from '@sourcegraph/branded/src/search-ui/results/progress/utils'
export { createDefaultSuggestions } from '@sourcegraph/branded/src/search-ui/input/codemirror'
export { parseInputAsQuery } from '@sourcegraph/branded/src/search-ui/input/codemirror/parsedQuery'
export { querySyntaxHighlighting } from '@sourcegraph/branded/src/search-ui/input/codemirror/syntax-highlighting'
export { decorateQuery } from '@sourcegraph/branded/src/search-ui/util/query'
export * from '@sourcegraph/branded/src/search-ui/input/codemirror/multiline'
export * from '@sourcegraph/branded/src/search-ui/input/codemirror/event-handlers'
export { tokenInfo } from '@sourcegraph/branded/src/search-ui/input/codemirror/token-info'
export * from '@sourcegraph/branded/src/search-ui/input/codemirror/diagnostics'
export * from '@sourcegraph/branded/src/search-ui/input/experimental/modes'
export * from '@sourcegraph/branded/src/search-ui/input/experimental/utils'
export * from '@sourcegraph/branded/src/search-ui/input/experimental/suggestionsExtension'
export { placeholder } from '@sourcegraph/branded/src/search-ui/input/codemirror/placeholder'
export { showWhenEmptyWithoutContext } from '@sourcegraph/branded/src/search-ui/input/experimental/placeholder'
export { filterDecoration } from '@sourcegraph/branded/src/search-ui/input/experimental/codemirror/syntax-highlighting'
export { overrideContextOnPaste } from '@sourcegraph/branded/src/search-ui/input/experimental/codemirror/searchcontext'
