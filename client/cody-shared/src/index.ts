export type { ChatContextStatus } from './chat/context'
export {
    useClient,
    type CodyClient,
    type CodyClientScope,
    type CodyClientConfig,
    type CodyClientEvent,
} from './chat/useClient'
export { renderCodyMarkdown } from './chat/markdown'
export type { ChatButton, ChatMessage } from './chat/transcript/messages'
export type { TranscriptJSON, TranscriptJSONScope } from './chat/transcript'
export { Transcript } from './chat/transcript'
export type { ContextFile, PreciseContext } from './codebase-context/messages'
export { basename, dedupeWith, isDefined, pluralize } from './common'
export { NoopEditor } from './editor'
export type { ActiveTextEditorSelectionRange } from './editor'
export type { Attribution, Guardrails } from './guardrails'
export { RateLimitError } from './sourcegraph-api/errors'
export type { CodyPrompt } from './chat/prompts'

export type {
    Editor,
    ActiveTextEditor,
    ActiveTextEditorDiagnostic,
    ActiveTextEditorSelection,
    ActiveTextEditorVisibleContent,
} from './editor'

export { TranslateToLanguage } from './chat/recipes/translate'
