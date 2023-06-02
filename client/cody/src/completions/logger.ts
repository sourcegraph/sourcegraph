import { logEvent } from '../event-logger'

interface CompletionEvent {
    type: 'inline' | 'manual'
    multilineMode: null | 'block'
    startedAt: number
    suggestedAt: number | null
}

const displayedCompletions: Map<string, CompletionEvent> = new Map()

export function start(
    { type, multilineMode }: { type: CompletionEvent['type']; multilineMode: CompletionEvent['multilineMode'] } = {
        type: 'inline',
        multilineMode: null,
    }
): string {
    const id = createId()
    displayedCompletions.set(id, {
        type,
        multilineMode,
        startedAt: Date.now(),
        suggestedAt: null,
    })

    const logParams = { type, multilineMode }
    logEvent('CodyVSCodeExtension:completion:started', logParams, logParams)

    return id
}

// Suggested completions will not logged individually. Instead, we log them when we either hide them
// again (they are NOT accepted) or when they ARE accepted. This way, we can calculate the duration
// they were actually visible.
export function suggest(id: string): void {
    const event = displayedCompletions.get(id)
    if (event) {
        event.suggestedAt = Date.now()
    }
}

export function accept(id: string): void {
    const completionEvent = displayedCompletions.get(id)
    const logParams = { type: completionEvent?.type, multilineMode: completionEvent?.multilineMode }
    logSuggestionEvent()
    logEvent('CodyVSCodeExtension:completion:accepted', logParams, logParams)
}

/**
 * This callback should be triggered whenever VS Code tries to highlight a new completion and it's
 * used to measure how long previous completions were visible.
 */
export function clear(): void {
    logSuggestionEvent()
}

function createId(): string {
    return Math.random().toString(36).slice(2, 11)
}

function logSuggestionEvent(): void {
    const now = Date.now()
    for (const completionEvent of displayedCompletions.values()) {
        const { suggestedAt, startedAt, type, multilineMode } = completionEvent

        if (!suggestedAt) {
            continue
        }

        const latency = suggestedAt - startedAt
        const displayDuration = now - suggestedAt
        const logParams = {
            type,
            multilineMode,
            latency,
            displayDuration,
            read: displayDuration >= 750,
        }

        logEvent('CodyVSCodeExtension:completion:suggested', logParams, logParams)
    }
    displayedCompletions.clear()
}
