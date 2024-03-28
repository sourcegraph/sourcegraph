export type ConfigurationUseContext = 'embeddings' | 'keyword' | 'none' | 'blended' | 'unified'

// Should we share VS Code specific config via cody-shared?
export interface Configuration {
    serverEndpoint: string
    proxy?: string | null
    codebase?: string
    debugEnable: boolean
    debugFilter: RegExp | null
    debugVerbose: boolean
    telemetryLevel: 'all' | 'off'
    useContext: ConfigurationUseContext
    customHeaders: Record<string, string>
    chatPreInstruction: string
    autocomplete: boolean
    experimentalChatPredictions: boolean
    inlineChat: boolean
    experimentalCommandLenses: boolean
    experimentalEditorTitleCommandIcon: boolean
    experimentalGuardrails: boolean
    experimentalNonStop: boolean
    experimentalLocalSymbols: boolean
    experimentalSymfPath: string
    experimentalSymfAnthropicKey: string
    autocompleteAdvancedProvider: 'anthropic' | 'unstable-codegen' | 'unstable-fireworks' | 'unstable-openai' | null
    autocompleteAdvancedServerEndpoint: string | null
    autocompleteAdvancedModel: string | null
    autocompleteAdvancedAccessToken: string | null
    autocompleteAdvancedEmbeddings: boolean
    autocompleteExperimentalCompleteSuggestWidgetSelection?: boolean
    autocompleteExperimentalSyntacticPostProcessing?: boolean
    autocompleteExperimentalGraphContext?: boolean
    isRunningInsideAgent?: boolean
}

export interface ConfigurationWithAccessToken extends Configuration {
    /** The access token, which is stored in the secret storage (not configuration). */
    accessToken: string | null
}
