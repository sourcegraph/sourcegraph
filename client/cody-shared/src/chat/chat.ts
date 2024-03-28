import { ANSWER_TOKENS } from '../prompt/constants'
import type { Message } from '../sourcegraph-api'
import type { SourcegraphCompletionsClient } from '../sourcegraph-api/completions/client'
import type { CompletionCallbacks, CompletionParameters } from '../sourcegraph-api/completions/types'

type ChatParameters = Omit<CompletionParameters, 'messages'>

const DEFAULT_CHAT_COMPLETION_PARAMETERS: ChatParameters = {
    temperature: 0.2,
    maxTokensToSample: ANSWER_TOKENS,
    topK: -1,
    topP: -1,
}

export class ChatClient {
    constructor(private completions: SourcegraphCompletionsClient) {}

    public chat(messages: Message[], cb: CompletionCallbacks, params?: Partial<ChatParameters>): () => void {
        const isLastMessageFromHuman = messages.length > 0 && messages.at(-1)!.speaker === 'human'
        const augmentedMessages = isLastMessageFromHuman ? messages.concat([{ speaker: 'assistant' }]) : messages

        return this.completions.stream(
            {
                ...DEFAULT_CHAT_COMPLETION_PARAMETERS,
                ...params,
                messages: augmentedMessages,
            },
            cb
        )
    }
}
