// If the bot message ends with some prefix of the `Human:` stop
// sequence, trim if from the end.
const STOP_SEQUENCE_REGEXP = /(H|Hu|Hum|Huma|Human|Human:)$/

export function reformatBotMessage(text: string, prefix: string): string {
    let reformattedMessage = prefix + text.trimEnd()

    const stopSequenceMatch = reformattedMessage.match(STOP_SEQUENCE_REGEXP)
    if (stopSequenceMatch) {
        reformattedMessage = reformattedMessage.slice(0, stopSequenceMatch.index)
    }
    // TODO: Detect if bot sent unformatted code without a markdown block.
    return fixOpenMarkdownCodeBlock(reformattedMessage)
}

function fixOpenMarkdownCodeBlock(text: string): string {
    const occurrences = text.split('```').length - 1
    if (occurrences % 2 === 1) {
        return text + '\n```'
    }
    return text
}
