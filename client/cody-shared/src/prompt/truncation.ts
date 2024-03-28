import { CHARS_PER_TOKEN } from './constants'

/**
 * Truncates text to the given number of tokens, keeping the start of the text.
 */
export function truncateText(text: string, maxTokens: number): string {
    const maxLength = maxTokens * CHARS_PER_TOKEN
    return text.length <= maxLength ? text : text.slice(0, maxLength)
}

/**
 * Truncates text to the given number of tokens, keeping the end of the text.
 */
export function truncateTextStart(text: string, maxTokens: number): string {
    const maxLength = maxTokens * CHARS_PER_TOKEN
    return text.length <= maxLength ? text : text.slice(-maxLength - 1)
}
