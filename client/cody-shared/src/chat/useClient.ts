import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { CodebaseContext } from '../codebase-context'
import { isErrorLike } from '../common'
import type { ConfigurationWithAccessToken } from '../configuration'
import { type Editor, NoopEditor } from '../editor'
import { type PrefilledOptions, withPreselectedOptions } from '../editor/withPreselectedOptions'
import { SourcegraphIntentDetectorClient } from '../intent-detector/client'
import { SourcegraphBrowserCompletionsClient } from '../sourcegraph-api/completions/browserClient'
import { SourcegraphGraphQLAPIClient } from '../sourcegraph-api/graphql'
import { UnifiedContextFetcherClient } from '../unified-context/client'
import { isError } from '../utils'

import { BotResponseMultiplexer } from './bot-response-multiplexer'
import { ChatClient } from './chat'
import { getMultiRepoPreamble } from './preamble'
import { getRecipe } from './recipes/browser-recipes'
import type { RecipeID } from './recipes/recipe'
import { Transcript } from './transcript'
import type { ChatMessage } from './transcript/messages'
import { Typewriter } from './typewriter'
import { reformatBotMessage } from './viewHelpers'

export type CodyClientConfig = Pick<
    ConfigurationWithAccessToken,
    'serverEndpoint' | 'useContext' | 'accessToken' | 'customHeaders' | 'experimentalLocalSymbols'
> & { debugEnable: boolean; needsEmailVerification: boolean }

export interface CodyClientScope {
    includeInferredRepository: boolean
    includeInferredFile: boolean
    repositories: string[]
    editor: Editor
}

export interface CodyClientScopePartial {
    repositories?: string[]
    editor?: Editor
}

export type CodyClientEvent = 'submit' | 'initializedNewChat' | 'error'

export interface CodyClient {
    readonly transcript: Transcript | null
    readonly chatMessages: ChatMessage[]
    readonly messageInProgress: ChatMessage | null
    readonly isMessageInProgress: boolean
    readonly scope: CodyClientScope
    readonly config: CodyClientConfig
    setTranscript: (transcript: Transcript) => Promise<void>
    setScope: (scope: CodyClientScope) => void
    setConfig: (config: CodyClientConfig) => void
    submitMessage: (humanChatInput: string, scope?: CodyClientScope) => Promise<Transcript | null>
    editMessage: (
        humanChatInput: string,
        messageId?: string | undefined,
        scope?: CodyClientScope
    ) => Promise<Transcript | null>
    initializeNewChat: (newScope?: Partial<CodyClientScope>) => Transcript | null
    executeRecipe: (
        recipeId: RecipeID,
        options?: {
            prefilledOptions?: PrefilledOptions
            humanChatInput?: string
            scope?: {
                editor?: Editor
            }
        }
    ) => Promise<Transcript | null>
    setEditorScope: (editor: Editor) => void
    toggleIncludeInferredRepository: () => void
    toggleIncludeInferredFile: () => void
    abortMessageInProgress: () => void
    fetchRepositoryNames: (count: number) => Promise<string[]>
}

interface CodyClientProps {
    config: CodyClientConfig
    scope?: CodyClientScope
    initialTranscript?: Transcript | null
    onEvent?: (event: CodyClientEvent) => void
}

export const useClient = ({
    config: initialConfig,
    initialTranscript = null,
    scope: initialScope = {
        includeInferredRepository: true,
        includeInferredFile: true,
        repositories: [],
        editor: new NoopEditor(),
    },
    onEvent,
}: CodyClientProps): CodyClient => {
    const [transcript, setTranscriptState] = useState<Transcript | null>(initialTranscript)
    const [chatMessages, setChatMessagesState] = useState<ChatMessage[]>([])
    const [isMessageInProgress, setIsMessageInProgressState] = useState<boolean>(false)
    const [abortMessageInProgressInternal, setAbortMessageInProgress] = useState<() => void>(() => () => undefined)

    const transcriptIdRef = useRef<Transcript['id']>()
    useEffect(() => {
        transcriptIdRef.current = transcript?.id
    }, [transcript])

    const messageInProgress: ChatMessage | null = useMemo(() => {
        if (isMessageInProgress) {
            const lastMessage = chatMessages.at(-1)

            if (lastMessage?.speaker === 'assistant') {
                return lastMessage
            }
        }

        return null
    }, [chatMessages, isMessageInProgress])

    const abortMessageInProgress = useCallback(() => {
        abortMessageInProgressInternal()

        transcript
            ?.toChatPromise()
            .then(messages => {
                setChatMessagesState(messages)
                setIsMessageInProgressState(false)
            })
            // eslint-disable-next-line no-console
            .catch(error => console.error(`aborting in progress message failed: ${error}`))
    }, [abortMessageInProgressInternal, transcript, setChatMessagesState, setIsMessageInProgressState])

    const setTranscript = useCallback(async (transcript: Transcript): Promise<void> => {
        const messages = await transcript.toChatPromise()

        setIsMessageInProgressState(false)
        setTranscriptState(transcript)
        setChatMessagesState(messages)
    }, [])

    const [config, setConfig] = useState<CodyClientConfig>(initialConfig)

    const { graphqlClient, chatClient, intentDetector } = useMemo(() => {
        const completionsClient = new SourcegraphBrowserCompletionsClient(config)
        const chatClient = new ChatClient(completionsClient)
        const graphqlClient = new SourcegraphGraphQLAPIClient(config)
        const intentDetector = new SourcegraphIntentDetectorClient(graphqlClient, completionsClient)

        return { graphqlClient, chatClient, intentDetector }
    }, [config])

    const [scope, setScopeState] = useState<CodyClientScope>(initialScope)
    const setScope = useCallback((scope: CodyClientScope) => setScopeState(scope), [setScopeState])

    const setEditorScope = useCallback(
        (editor: Editor) => {
            const newRepoName = editor.getActiveTextEditor()?.repoName

            return setScopeState(scope => {
                const oldRepoName = scope.editor.getActiveTextEditor()?.repoName

                const resetInferredScope = newRepoName !== oldRepoName

                return {
                    ...scope,
                    editor,
                    includeInferredRepository: resetInferredScope ? true : scope.includeInferredRepository,
                    includeInferredFile: resetInferredScope ? true : scope.includeInferredFile,
                }
            })
        },
        [setScopeState]
    )

    const toggleIncludeInferredRepository = useCallback(
        () =>
            setScopeState(scope => ({
                ...scope,
                includeInferredRepository: !scope.includeInferredRepository,
                includeInferredFile: !scope.includeInferredRepository,
            })),
        [setScopeState]
    )

    const toggleIncludeInferredFile = useCallback(
        () => setScopeState(scope => ({ ...scope, includeInferredFile: !scope.includeInferredFile })),
        [setScopeState]
    )

    const activeEditor = useMemo(() => scope.editor.getActiveTextEditor(), [scope.editor])

    const codebases: string[] = useMemo(() => {
        const repos = [...scope.repositories]
        if (scope.includeInferredRepository && activeEditor?.repoName) {
            repos.push(activeEditor.repoName)
        }

        return repos
    }, [scope, activeEditor])

    const codebaseIds: Promise<string[]> = useMemo(async () => {
        if (!codebases.length) {
            return []
        }

        const results = await graphqlClient.getRepoIds(codebases)
        if (isError(results)) {
            // eslint-disable-next-line no-console
            console.error(
                `Cody could not access the repositories on your Sourcegraph instance. Details: ${results.message}`
            )
            return []
        }

        return results.map(({ id }) => id)
    }, [codebases, graphqlClient])

    const fetchRepositoryNames = useCallback(
        async (count: number): Promise<string[]> =>
            graphqlClient
                .getRepoNames(count)
                .then(repositories => (isErrorLike(repositories) ? [] : repositories))
                .catch(error => {
                    // eslint-disable-next-line no-console
                    console.error(
                        `Cody could not fetch the list of repositories on your Sourcegraph instance. Details: ${error}`
                    )

                    return []
                }),
        [graphqlClient]
    )

    const initializeNewChat = useCallback(
        (initialScope?: Partial<CodyClientScope>): Transcript | null => {
            if (config.needsEmailVerification) {
                return transcript
            }
            const newTranscript = new Transcript()
            setIsMessageInProgressState(false)
            setTranscriptState(newTranscript)
            setChatMessagesState(newTranscript.toChat())
            setScopeState(scope => ({
                includeInferredRepository: initialScope?.includeInferredFile ?? true,
                includeInferredFile: initialScope?.includeInferredFile ?? true,
                repositories: initialScope?.repositories ?? [],
                editor: initialScope?.editor ?? scope.editor,
            }))

            onEvent?.('initializedNewChat')

            return newTranscript
        },
        [onEvent, config.needsEmailVerification, transcript]
    )

    const executeRecipe = useCallback(
        async (
            recipeId: RecipeID,
            options?: {
                prefilledOptions?: PrefilledOptions
                humanChatInput?: string
                scope?: CodyClientScopePartial
            }
        ): Promise<Transcript | null> => {
            const recipe = getRecipe(recipeId)
            if (!recipe || transcript === null || isMessageInProgress || config.needsEmailVerification) {
                return Promise.resolve(null)
            }

            const repoNames = [...codebases]
            const repoIds = [...(await codebaseIds)]
            const editor = options?.scope?.editor || (scope.includeInferredFile ? scope.editor : new NoopEditor())
            const activeEditor = editor.getActiveTextEditor()
            if (activeEditor?.repoName && !repoNames.includes(activeEditor.repoName)) {
                // NOTE(naman): We allow users to disable automatic inferrence of current file & repo
                // using `includeInferredFile` and `includeInferredRepository` options. But for editor recipes
                // like "Explain code at high level", we need to pass the current repo & file context.
                // Here we are passing the current repo & file context based on `options.scope.editor`
                // if present.
                const additionalRepoId = await graphqlClient.getRepoId(activeEditor.repoName)
                if (isError(additionalRepoId)) {
                    // eslint-disable-next-line no-console
                    console.error(
                        `Cody could not access the ${activeEditor.repoName} repository on your Sourcegraph instance. Details: ${additionalRepoId.message}`
                    )
                } else {
                    repoIds.push(additionalRepoId)
                    repoNames.push(activeEditor.repoName)
                }
            }

            const unifiedContextFetcherClient = new UnifiedContextFetcherClient(graphqlClient, repoIds)
            const codebaseContext = new CodebaseContext(
                config,
                undefined,
                null,
                null,
                null,
                null,
                undefined,
                unifiedContextFetcherClient
            )

            const { humanChatInput = '', prefilledOptions } = options ?? {}
            // TODO(naman): save scope with each interaction
            const interaction = await recipe.getInteraction(humanChatInput, {
                editor: prefilledOptions ? withPreselectedOptions(editor, prefilledOptions) : editor,
                intentDetector,
                codebaseContext,
                responseMultiplexer: new BotResponseMultiplexer(),
                firstInteraction: transcript.isEmpty,
            })
            if (!interaction) {
                return Promise.resolve(null)
            }

            transcript.addInteraction(interaction)
            setChatMessagesState(transcript.toChat())
            setIsMessageInProgressState(true)
            onEvent?.('submit')

            const { prompt, contextFiles, preciseContexts } = await transcript.getPromptForLastInteraction(
                getMultiRepoPreamble(repoNames)
            )
            transcript.setUsedContextFilesForLastInteraction(contextFiles, preciseContexts)

            const responsePrefix = interaction.getAssistantMessage().prefix ?? ''
            let rawText = ''

            const updatedTranscript = await new Promise<Transcript | null>(resolve => {
                const typewriter = new Typewriter({
                    update(_rawText) {
                        if (transcript.id !== transcriptIdRef.current) {
                            abort()
                            resolve(transcript)
                            return
                        }
                        rawText = _rawText

                        const text = reformatBotMessage(rawText, responsePrefix)
                        transcript.addAssistantResponse(text)
                        setChatMessagesState(transcript.toChat())
                    },
                    close() {
                        transcript
                            .toChatPromise()
                            .then(messages => {
                                setChatMessagesState(messages)
                                setIsMessageInProgressState(false)
                            })
                            .catch(() => null)

                        resolve(transcript)
                    },
                })

                const abort = chatClient.chat(prompt, {
                    onChange(content) {
                        if (transcript.id !== transcriptIdRef.current) {
                            typewriter.close()
                            typewriter.stop()
                            abort()
                            resolve(transcript)
                            return
                        }

                        try {
                            typewriter.update(content)
                        } catch (error: any) {
                            // eslint-disable-next-line no-console
                            console.error(`Error while updating typewriter: ${error}`)

                            typewriter.close()
                            typewriter.stop()
                            abort()
                            resolve(transcript)
                        }
                    },
                    onComplete() {
                        if (transcript.id !== transcriptIdRef.current) {
                            typewriter.close()
                            typewriter.stop()
                            abort()
                            resolve(transcript)
                            return
                        }

                        typewriter.close()
                    },
                    onError(error) {
                        // Display error message as assistant response
                        transcript.addErrorAsAssistantResponse(error)
                        // eslint-disable-next-line no-console
                        console.error(`Completion request failed: ${error}`)
                        onEvent?.('error')

                        typewriter.close()
                        typewriter.stop()
                        abort()
                        resolve(transcript)
                    },
                })

                setAbortMessageInProgress(() => () => {
                    typewriter.close()
                    typewriter.stop()
                    abort()
                    resolve(transcript)
                })
            })

            setAbortMessageInProgress(() => () => undefined)

            return updatedTranscript
        },
        [
            config,
            scope,
            codebases,
            codebaseIds,
            graphqlClient,
            transcript,
            intentDetector,
            chatClient,
            isMessageInProgress,
            onEvent,
            setAbortMessageInProgress,
        ]
    )

    const submitMessage = useCallback(
        async (humanChatInput: string, scope?: CodyClientScope): Promise<Transcript | null> =>
            executeRecipe('chat-question', { humanChatInput, scope }),
        [executeRecipe]
    )

    // TODO(naman): load message scope from the interaction
    const editMessage = useCallback(
        async (
            humanChatInput: string,
            messageId?: string | undefined,
            scope?: CodyClientScope
        ): Promise<Transcript | null> => {
            if (!transcript) {
                return transcript
            }

            const timestamp = messageId || transcript.getLastInteraction()?.timestamp || new Date().toISOString()

            transcript.removeInteractionsSince(timestamp)
            setChatMessagesState(transcript.toChat())

            return submitMessage(humanChatInput, scope)
        },
        [transcript, submitMessage]
    )

    const returningChatMessages = useMemo(
        () => (messageInProgress ? chatMessages.slice(0, -1) : chatMessages),
        [chatMessages, messageInProgress]
    )

    return useMemo(
        () => ({
            transcript,
            chatMessages: returningChatMessages,
            isMessageInProgress,
            messageInProgress,
            setTranscript,
            scope,
            setScope,
            setEditorScope,
            config,
            setConfig,
            executeRecipe,
            submitMessage,
            initializeNewChat,
            editMessage,
            toggleIncludeInferredRepository,
            toggleIncludeInferredFile,
            abortMessageInProgress,
            fetchRepositoryNames,
        }),
        [
            transcript,
            returningChatMessages,
            isMessageInProgress,
            messageInProgress,
            setTranscript,
            scope,
            setScope,
            setEditorScope,
            config,
            setConfig,
            executeRecipe,
            submitMessage,
            initializeNewChat,
            editMessage,
            toggleIncludeInferredRepository,
            toggleIncludeInferredFile,
            abortMessageInProgress,
            fetchRepositoryNames,
        ]
    )
}
