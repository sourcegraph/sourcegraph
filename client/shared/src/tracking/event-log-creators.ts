/**
 * Returns an [EventName, argument, publicArgument] for a "code copied" to clipboard event
 *
 * @param page page or component from where copied
 */
export const codeCopiedEvent = (page: string): [string, { page: string }, { page: string }] => [
    'CodeCopied',
    { page },
    { page },
]
