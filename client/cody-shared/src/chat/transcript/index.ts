import type { ContextFile, ContextMessage, OldContextMessage, PreciseContext } from '../../codebase-context/messages'
import { CHARS_PER_TOKEN, MAX_AVAILABLE_PROMPT_LENGTH } from '../../prompt/constants'
import { PromptMixin } from '../../prompt/prompt-mixin'
import type { Message } from '../../sourcegraph-api'

import { Interaction, type InteractionJSON } from './interaction'
import type { ChatMessage } from './messages'

export interface TranscriptJSONScope {
    includeInferredRepository: boolean
    includeInferredFile: boolean
    repositories: string[]
}

export interface TranscriptJSON {
    // This is the timestamp of the first interaction.
    id: string
    interactions: InteractionJSON[]
    lastInteractionTimestamp: string
    scope?: TranscriptJSONScope
}

/**
 * The "model" class that tracks the call and response of the Cody chat box.
 * Any "controller" logic belongs outside of this class.
 */
export class Transcript {
    public static fromJSON(json: TranscriptJSON): Transcript {
        return new Transcript(
            json.interactions.map(
                ({
                    humanMessage,
                    assistantMessage,
                    context,
                    fullContext,
                    usedContextFiles,
                    usedPreciseContext,
                    timestamp,
                }) => {
                    if (!fullContext) {
                        fullContext = context || []
                    }
                    return new Interaction(
                        humanMessage,
                        assistantMessage,
                        Promise.resolve(
                            fullContext.map(message => {
                                if (message.file) {
                                    return message
                                }

                                const { fileName } = message as any as OldContextMessage
                                if (fileName) {
                                    return { ...message, file: { fileName } }
                                }

                                return message
                            })
                        ),
                        usedContextFiles || [],
                        usedPreciseContext || [],
                        timestamp || new Date().toISOString()
                    )
                }
            ),
            json.id
        )
    }

    private interactions: Interaction[] = []

    private internalID: string

    constructor(interactions: Interaction[] = [], id?: string) {
        this.interactions = interactions
        this.internalID =
            id ||
            this.interactions.find(({ timestamp }) => !isNaN(new Date(timestamp) as any))?.timestamp ||
            new Date().toISOString()
    }

    public get id(): string {
        return this.internalID
    }

    public get isEmpty(): boolean {
        return this.interactions.length === 0
    }

    public get lastInteractionTimestamp(): string {
        for (let index = this.interactions.length - 1; index >= 0; index--) {
            const { timestamp } = this.interactions[index]

            if (!isNaN(new Date(timestamp) as any)) {
                return timestamp
            }
        }

        return this.internalID
    }

    public addInteraction(interaction: Interaction | null): void {
        if (!interaction) {
            return
        }
        this.interactions.push(interaction)
    }

    public getLastInteraction(): Interaction | null {
        return this.interactions.length > 0 ? this.interactions.at(-1)! : null
    }

    public removeLastInteraction(): void {
        this.interactions.pop()
    }

    public removeInteractionsSince(id: string): void {
        const index = this.interactions.findIndex(({ timestamp }) => timestamp === id)
        if (index >= 0) {
            this.interactions = this.interactions.slice(0, index)
        }
    }

    public addAssistantResponse(text: string, displayText?: string): void {
        this.getLastInteraction()?.setAssistantMessage({
            speaker: 'assistant',
            text,
            displayText: displayText ?? text,
        })
    }

    /**
     * Adds a error div to the assistant response. If the assistant has collected
     * some response before, we will add the error message after it.
     * @param errorText The error TEXT to be displayed. Do not wrap it in HTML tags.
     */
    public addErrorAsAssistantResponse(errorText: string): void {
        const lastInteraction = this.getLastInteraction()
        if (!lastInteraction) {
            return
        }
        // If assistant has responsed before, we will add the error message after it
        const lastAssistantMessage = lastInteraction.getAssistantMessage().displayText || ''
        lastInteraction.setAssistantMessage({
            speaker: 'assistant',
            text: 'Failed to generate a response due to server error.',
            displayText:
                lastAssistantMessage + `<div class="cody-chat-error"><span>Request failed: </span>${errorText}</div>`,
        })
    }

    private async getLastInteractionWithContextIndex(): Promise<number> {
        for (let index = this.interactions.length - 1; index >= 0; index--) {
            const hasContext = await this.interactions[index].hasContext()
            if (hasContext) {
                return index
            }
        }
        return -1
    }

    public async getPromptForLastInteraction(
        preamble: Message[] = [],
        maxPromptLength: number = MAX_AVAILABLE_PROMPT_LENGTH,
        onlyHumanMessages: boolean = false
    ): Promise<{ prompt: Message[]; contextFiles: ContextFile[]; preciseContexts: PreciseContext[] }> {
        if (this.interactions.length === 0) {
            return { prompt: [], contextFiles: [], preciseContexts: [] }
        }

        const lastInteractionWithContextIndex = await this.getLastInteractionWithContextIndex()
        const messages: Message[] = []
        for (let index = 0; index < this.interactions.length; index++) {
            const interaction = this.interactions[index]
            const humanMessage = PromptMixin.mixInto(interaction.getHumanMessage())
            const assistantMessage = interaction.getAssistantMessage()
            const contextMessages = await interaction.getFullContext()
            if (index === lastInteractionWithContextIndex && !onlyHumanMessages) {
                messages.push(...contextMessages, humanMessage, assistantMessage)
            } else {
                messages.push(humanMessage, assistantMessage)
            }
        }

        const preambleTokensUsage = preamble.reduce((acc, message) => acc + estimateTokensUsage(message), 0)
        let truncatedMessages = truncatePrompt(messages, maxPromptLength - preambleTokensUsage)
        // Return what context fits in the window
        const contextFiles: ContextFile[] = []
        const preciseContexts: PreciseContext[] = []
        for (const msg of truncatedMessages) {
            const contextFile = (msg as ContextMessage).file
            if (contextFile) {
                contextFiles.push(contextFile)
            }

            const preciseContext = (msg as ContextMessage).preciseContext
            if (preciseContext) {
                preciseContexts.push(preciseContext)
            }
        }

        // Filter out extraneous fields from ContextMessage instances
        truncatedMessages = truncatedMessages.map(({ speaker, text }) => ({ speaker, text }))

        return {
            prompt: [...preamble, ...truncatedMessages],
            contextFiles,
            preciseContexts,
        }
    }

    public setUsedContextFilesForLastInteraction(
        contextFiles: ContextFile[],
        preciseContexts: PreciseContext[] = []
    ): void {
        if (this.interactions.length === 0) {
            throw new Error('Cannot set context files for empty transcript')
        }
        this.interactions.at(-1)!.setUsedContext(contextFiles, preciseContexts)
    }

    public toChat(): ChatMessage[] {
        return this.interactions.flatMap(interaction => interaction.toChat())
    }

    public async toChatPromise(): Promise<ChatMessage[]> {
        return [...(await Promise.all(this.interactions.map(interaction => interaction.toChatPromise())))].flat()
    }

    public async toJSON(scope?: TranscriptJSONScope): Promise<TranscriptJSON> {
        const interactions = await Promise.all(this.interactions.map(interaction => interaction.toJSON()))

        return {
            id: this.id,
            interactions,
            lastInteractionTimestamp: this.lastInteractionTimestamp,
            scope: scope
                ? {
                      repositories: scope.repositories,
                      includeInferredRepository: scope.includeInferredRepository,
                      includeInferredFile: scope.includeInferredFile,
                  }
                : undefined,
        }
    }

    public toJSONEmpty(scope?: TranscriptJSONScope): TranscriptJSON {
        return {
            id: this.id,
            interactions: [],
            lastInteractionTimestamp: this.lastInteractionTimestamp,
            scope: scope
                ? {
                      repositories: scope.repositories,
                      includeInferredRepository: scope.includeInferredRepository,
                      includeInferredFile: scope.includeInferredFile,
                  }
                : undefined,
        }
    }

    public reset(): void {
        this.interactions = []
        this.internalID = new Date().toISOString()
    }
}

/**
 * Truncates the given prompt messages to fit within the available tokens budget.
 * The truncation is done by removing the oldest pairs of messages first.
 * No individual message will be truncated. We just remove pairs of messages if they exceed the available tokens budget.
 */
function truncatePrompt(messages: Message[], maxTokens: number): Message[] {
    const newPromptMessages = []
    let availablePromptTokensBudget = maxTokens
    for (let i = messages.length - 1; i >= 1; i -= 2) {
        const humanMessage = messages[i - 1]
        const botMessage = messages[i]
        const combinedTokensUsage = estimateTokensUsage(humanMessage) + estimateTokensUsage(botMessage)

        // We stop adding pairs of messages once we exceed the available tokens budget.
        if (combinedTokensUsage <= availablePromptTokensBudget) {
            newPromptMessages.push(botMessage, humanMessage)
            availablePromptTokensBudget -= combinedTokensUsage
        } else {
            break
        }
    }

    // Reverse the prompt messages, so they appear in chat order (older -> newer).
    return newPromptMessages.reverse()
}

/**
 * Gives a rough estimate for the number of tokens used by the message.
 */
function estimateTokensUsage(message: Message): number {
    return Math.round((message.text || '').length / CHARS_PER_TOKEN)
}
