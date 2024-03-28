import type { InteractionMessage } from '../chat/transcript/messages'

/**
 * Prompt mixins elaborate every prompt presented to the LLM.
 * Add a prompt mixin to prompt for cross-cutting concerns relevant to multiple recipes.
 */
export class PromptMixin {
    private static mixins: PromptMixin[] = []
    private static customMixin: PromptMixin[] = []

    /**
     * Adds a prompt mixin to the global set.
     */
    public static add(mixin: PromptMixin): void {
        this.mixins.push(mixin)
    }

    /**
     * Adds a custom prompt mixin but not to the global set to make sure it will not be added twice
     * and any new change could replace the old one.
     */
    public static addCustom(mixin: PromptMixin): void {
        this.customMixin = [mixin]
    }

    /**
     * Prepends all mixins to `humanMessage`. Modifies and returns `humanMessage`.
     */
    public static mixInto(humanMessage: InteractionMessage): InteractionMessage {
        const mixins = [...this.mixins, ...this.customMixin].map(mixin => mixin.prompt).join('\n\n')
        if (mixins) {
            // Stuff the prompt mixins at the start of the human text.
            // Note we do not reflect them in displayText.
            return { ...humanMessage, text: `${mixins}\n\n${humanMessage.text}` }
        }
        return humanMessage
    }

    /**
     * Creates a mixin with the given, fixed prompt to insert.
     */
    constructor(private readonly prompt: string) {}
}

/**
 * Creates a prompt mixin to get Cody to reply in the given language, for example "en-AU" for "Australian English".
 * End with a new statement to redirect Cody to the next prompt. This prevents Cody from responding to the language prompt.
 */
export function languagePromptMixin(languageCode: string): PromptMixin {
    const languagePrompt = languageCode ? `, in the language with RFC5646/ISO language code "${languageCode}"` : ''
    return new PromptMixin(`(Reply as Cody, a coding assistant developed by Sourcegraph${languagePrompt}) `)
}

export function newPromptMixin(text: string): PromptMixin {
    return new PromptMixin(text)
}
