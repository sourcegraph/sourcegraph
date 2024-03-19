import { useState, useEffect, useCallback, useMemo } from 'react'

import { noop } from 'lodash'

import {
    Transcript,
    type TranscriptJSON,
    type TranscriptJSONScope,
} from '@sourcegraph/cody-shared/dist/chat/transcript'
import {
    useClient,
    type CodyClient,
    type CodyClientScope,
    type CodyClientConfig,
    type CodyClientEvent,
} from '@sourcegraph/cody-shared/dist/chat/useClient'
import { NoopEditor } from '@sourcegraph/cody-shared/dist/editor'
import type { Scalars } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { useLocalStorage } from '@sourcegraph/wildcard'

import { eventLogger } from '../tracking/eventLogger'
import { EventName } from '../util/constants'

import { isEmailVerificationNeededForCody } from './isCodyEnabled'

export type { CodyClientScope } from '@sourcegraph/cody-shared/dist/chat/useClient'

export interface CodyChatStore
    extends Pick<
        CodyClient,
        | 'transcript'
        | 'chatMessages'
        | 'isMessageInProgress'
        | 'messageInProgress'
        | 'submitMessage'
        | 'editMessage'
        | 'executeRecipe'
        | 'scope'
        | 'setScope'
        | 'setEditorScope'
        | 'toggleIncludeInferredRepository'
        | 'toggleIncludeInferredFile'
        | 'abortMessageInProgress'
    > {
    readonly transcriptHistory: TranscriptJSON[]
    readonly loaded: boolean
    readonly storageQuotaExceeded: boolean
    clearHistory: () => void
    deleteHistoryItem: (id: string) => void
    loadTranscriptFromHistory: (id: string) => Promise<void>
    logTranscriptEvent: (
        v1EventLabel: string,
        feature: CodyTranscriptEventFeatures,
        action: CodyTranscriptEventActions,
        eventProperties?: { [key: string]: any }
    ) => void
    initializeNewChat: () => void
}

export const codyChatStoreMock: CodyChatStore = {
    transcript: null,
    chatMessages: [],
    isMessageInProgress: false,
    messageInProgress: null,
    storageQuotaExceeded: false,
    submitMessage: () => Promise.resolve(null),
    editMessage: () => Promise.resolve(null),
    initializeNewChat: () => null,
    executeRecipe: () => Promise.resolve(null),
    scope: {
        repositories: [],
        editor: new NoopEditor(),
        includeInferredRepository: true,
        includeInferredFile: true,
    },
    setScope: () => {},
    setEditorScope: () => {},
    transcriptHistory: [],
    loaded: true,
    clearHistory: () => {},
    deleteHistoryItem: () => {},
    loadTranscriptFromHistory: () => Promise.resolve(),
    logTranscriptEvent: () => {},
    toggleIncludeInferredRepository: () => {},
    toggleIncludeInferredFile: () => {},
    abortMessageInProgress: () => {},
}

interface CodyChatProps extends TelemetryV2Props {
    userID?: Scalars['ID']
    scope?: CodyClientScope
    config?: CodyClientConfig
    onEvent?: (event: CodyClientEvent) => void
    onTranscriptHistoryLoad?: (
        loadTranscriptFromHistory: (id: string) => Promise<void>,
        transcriptHistory: TranscriptJSON[],
        initializeNewChat: CodyClient['initializeNewChat']
    ) => void
    autoLoadTranscriptFromHistory?: boolean
}

const CODY_TRANSCRIPT_HISTORY_KEY = 'cody.chat.history'
const SAVE_MAX_TRANSCRIPT_HISTORY = 20

export type CodyTranscriptEventFeatures =
    | 'cody.chat'
    | 'cody.chat.item'
    | 'cody.chat.inferredRepo'
    | 'cody.chat.inferredFile'
    | 'cody.chat.getEditorExtensionCTA'
    | 'cody.chat.scope.repo'
    | 'repo.askCody'

export type CodyTranscriptEventActions =
    | 'view'
    | 'submit'
    | 'edit'
    | 'initialize'
    | 'remove'
    | 'reset'
    | 'delete'
    | 'click'
    | 'disable'
    | 'enable'
    | 'add'

export const useCodyChat = ({
    userID = 'anonymous',
    scope: initialScope,
    config: initialConfig,
    onEvent,
    onTranscriptHistoryLoad,
    autoLoadTranscriptFromHistory = true,
    telemetryRecorder,
}: CodyChatProps): CodyChatStore => {
    const [loadedTranscriptFromHistory, setLoadedTranscriptFromHistory] = useState(false)
    // Read old transcript history from local storage, if any exists. We will use this to
    // preserve the history as we migrate to a new key that is differentiated by user.
    const oldJSON = window.localStorage.getItem(CODY_TRANSCRIPT_HISTORY_KEY)
    // eslint-disable-next-line no-restricted-syntax
    const [transcriptHistoryInternal, setTranscriptHistoryStateInternal] = useLocalStorage<TranscriptJSON[]>(
        // Users have distinct transcript histories, so we use the user ID as a key.
        `${CODY_TRANSCRIPT_HISTORY_KEY}:${userID}`,
        []
    )
    const [storageQuotaExceeded, setStorageQuotaExceeded] = useState(false)
    const transcriptHistory = useMemo(() => transcriptHistoryInternal || [], [transcriptHistoryInternal])
    const setTranscriptHistoryState = useCallback<typeof setTranscriptHistoryStateInternal>(
        value => {
            try {
                setTranscriptHistoryStateInternal(value)
                setStorageQuotaExceeded(false)
            } catch (error) {
                if (error.name === 'QuotaExceededError') {
                    setStorageQuotaExceeded(true)
                }
            }
        },
        [setTranscriptHistoryStateInternal, setStorageQuotaExceeded]
    )

    const {
        transcript,
        isMessageInProgress,
        messageInProgress,
        chatMessages,
        scope,
        setScope: setScopeInternal,
        setEditorScope,
        setTranscript,
        abortMessageInProgress,
        toggleIncludeInferredRepository: toggleIncludeInferredRepositoryInternal,
        toggleIncludeInferredFile: toggleIncludeInferredFileInternal,
        initializeNewChat: initializeNewChatInternal,
        submitMessage: submitMessageInternal,
        editMessage: editMessageInternal,
        executeRecipe: executeRecipeInternal,
        ...client
    } = useClient({
        config: initialConfig || {
            serverEndpoint: window.location.origin,
            useContext: 'unified',
            accessToken: null,
            customHeaders: window.context.xhrHeaders,
            debugEnable: false,
            needsEmailVerification: isEmailVerificationNeededForCody(),
            experimentalLocalSymbols: false,
        },
        scope: initialScope,
        onEvent,
    })

    /** Event logger for transcript specific events to capture the transcriptId */
    const logTranscriptEvent = useCallback(
        (
            v1EventLabel: string,
            feature: CodyTranscriptEventFeatures,
            action: CodyTranscriptEventActions,
            eventProperties?: { [key: string]: any }
        ) => {
            if (!transcript) {
                return
            }
            eventLogger.log(v1EventLabel, { transcriptId: transcript.id, ...eventProperties })

            let numericID = new Date(transcript.id).getTime()
            if (isNaN(numericID)) {
                numericID = 0
            }
            telemetryRecorder.recordEvent(feature, action, { metadata: { transcriptId: numericID / 1000 } })
        },
        [transcript, telemetryRecorder]
    )

    const loadTranscriptFromHistory = useCallback(
        async (id: string) => {
            if (transcript?.id === id) {
                return
            }

            const transcriptToLoad = transcriptHistory.find(item => item.id === id)
            if (transcriptToLoad) {
                await setTranscript(Transcript.fromJSON(transcriptToLoad))

                if (transcriptToLoad.scope) {
                    setScopeInternal({ ...scope, ...transcriptToLoad.scope })
                }
            }
        },
        [transcriptHistory, transcript?.id, setTranscript, setScopeInternal, scope]
    )

    const updateTranscriptInHistory = useCallback(
        async (transcript: Transcript, transcriptScope?: TranscriptJSONScope) => {
            const transcriptJSON = await transcript.toJSON(transcriptScope || scope)

            setTranscriptHistoryState((history: TranscriptJSON[]) => {
                const index = history.findIndex(item => item.id === transcript.id)
                if (index >= 0) {
                    history[index] = transcriptJSON
                }

                return [...history]
            })
        },
        [setTranscriptHistoryState, scope]
    )

    const pushTranscriptToHistory = useCallback(
        async (transcript: Transcript, transcriptScope?: TranscriptJSONScope) => {
            const transcriptJSON = await transcript.toJSON(transcriptScope || scope)

            setTranscriptHistoryState((history: TranscriptJSON[] = []) =>
                sortSliceTranscriptHistory([transcriptJSON, ...history])
            )
        },
        [setTranscriptHistoryState, scope]
    )

    const clearHistory = useCallback(() => {
        if (client.config.needsEmailVerification) {
            return
        }

        eventLogger.log(EventName.CODY_CHAT_HISTORY_CLEARED)

        const newTranscript = initializeNewChatInternal()
        if (newTranscript) {
            setTranscriptHistoryState([newTranscript.toJSONEmpty()])
        } else {
            setTranscriptHistoryState([])
        }
    }, [client.config.needsEmailVerification, initializeNewChatInternal, setTranscriptHistoryState])

    const deleteHistoryItem = useCallback(
        (id: string): void => {
            if (client.config.needsEmailVerification) {
                return
            }

            logTranscriptEvent(EventName.CODY_CHAT_HISTORY_ITEM_DELETED, 'cody.chat.item', 'delete')

            setTranscriptHistoryState((history: TranscriptJSON[]) => {
                const updatedHistory = [...history.filter(transcript => transcript.id !== id)]

                if (transcript?.id === id) {
                    if (updatedHistory.length === 0) {
                        const newTranscript = initializeNewChatInternal()

                        if (newTranscript) {
                            updatedHistory.push(newTranscript.toJSONEmpty())
                        }
                    } else {
                        const transcriptToLoad = updatedHistory[0]

                        setTranscript(Transcript.fromJSON(transcriptToLoad)).catch(() => null)

                        if (transcriptToLoad.scope) {
                            setScopeInternal({ ...scope, ...transcriptToLoad.scope })
                        }
                    }
                }

                return sortSliceTranscriptHistory(updatedHistory)
            })
        },
        [
            setTranscript,
            setScopeInternal,
            client.config.needsEmailVerification,
            initializeNewChatInternal,
            transcript?.id,
            setTranscriptHistoryState,
            scope,
            logTranscriptEvent,
        ]
    )

    const submitMessage = useCallback<typeof submitMessageInternal>(
        async (humanInputText, scope): Promise<Transcript | null> => {
            const transcript = await submitMessageInternal(humanInputText, scope)

            if (transcript) {
                await updateTranscriptInHistory(transcript)
            }

            logTranscriptEvent(EventName.CODY_CHAT_SUBMIT, 'cody.chat', 'submit')
            return transcript
        },
        [submitMessageInternal, updateTranscriptInHistory, logTranscriptEvent]
    )

    const editMessage = useCallback<typeof editMessageInternal>(
        async (humanInputText, messageId?, scope?): Promise<Transcript | null> => {
            const transcript = await editMessageInternal(humanInputText, messageId, scope)

            if (transcript) {
                await updateTranscriptInHistory(transcript)
            }

            logTranscriptEvent(EventName.CODY_CHAT_EDIT, 'cody.chat', 'edit')
            return transcript
        },
        [editMessageInternal, updateTranscriptInHistory, logTranscriptEvent]
    )

    const initializeNewChat = useCallback((): Transcript | null => {
        // If we already have a transcript, but it hasn't been interacted with yet, don't
        // create a new one.
        if (transcript && !transcript.getLastInteraction()) {
            return null
        }

        const newTranscript = initializeNewChatInternal(scope)

        if (!newTranscript) {
            return null
        }

        pushTranscriptToHistory(newTranscript).catch(noop)

        logTranscriptEvent(EventName.CODY_CHAT_INITIALIZED, 'cody.chat', 'initialize')
        return newTranscript
    }, [initializeNewChatInternal, pushTranscriptToHistory, scope, transcript, logTranscriptEvent])

    const executeRecipe = useCallback<typeof executeRecipeInternal>(
        async (recipeId, options): Promise<Transcript | null> => {
            eventLogger.log(`web:codyChat:recipe:${recipeId}:executed`, { recipeId })

            const transcript = await executeRecipeInternal(recipeId, options)

            if (transcript) {
                await updateTranscriptInHistory(transcript)
            }

            return transcript
        },
        [executeRecipeInternal, updateTranscriptInHistory]
    )

    const loaded = useMemo(() => loadedTranscriptFromHistory, [loadedTranscriptFromHistory])

    // Autoload the latest transcript from history once it is loaded. Initially the transcript is null.
    useEffect(() => {
        if (!loadedTranscriptFromHistory && transcript === null) {
            // User transcript history entries were previously stored in a single array in
            // local storage under the generic key `cody.chat.history`, which is not
            // differentiated by user. The first time we run this effect, we take any old
            // entries and write them to the new user-differentiated key, and then delete
            // the old key. When the effect runs again in the future, it will thus only
            // run the code that comes after this block.
            if (oldJSON) {
                setTranscriptHistoryState(JSON.parse(oldJSON))
                window.localStorage.removeItem(CODY_TRANSCRIPT_HISTORY_KEY)
                return
            }

            const history = sortSliceTranscriptHistory([...transcriptHistory])

            if (autoLoadTranscriptFromHistory) {
                if (history.length > 0) {
                    const transcriptToLoad = history[0]

                    setTranscript(Transcript.fromJSON(transcriptToLoad)).catch(() => null)

                    if (transcriptToLoad.scope) {
                        setScopeInternal({ ...scope, ...transcriptToLoad.scope })
                    }
                } else {
                    const newTranscript = new Transcript()
                    history.push({ interactions: [], id: newTranscript.id, lastInteractionTimestamp: newTranscript.id })
                    setTranscript(newTranscript)
                        .then(() => setTranscriptHistoryState(history))
                        .catch(() => null)
                }
            }
            // usefull to load transcript from any other source like url.
            onTranscriptHistoryLoad?.(loadTranscriptFromHistory, history, initializeNewChat)

            setLoadedTranscriptFromHistory(true)
        }
    }, [
        transcriptHistory,
        oldJSON,
        loadedTranscriptFromHistory,
        transcript,
        autoLoadTranscriptFromHistory,
        onTranscriptHistoryLoad,
        setTranscript,
        setTranscriptHistoryState,
        loadTranscriptFromHistory,
        initializeNewChat,
        scope,
        setScopeInternal,
    ])

    const setScope = useCallback<CodyClient['setScope']>(
        scope => {
            setScopeInternal(scope)

            if (transcript) {
                updateTranscriptInHistory(transcript, scope).catch(() => null)
            }
        },
        [setScopeInternal, transcript, updateTranscriptInHistory]
    )

    const toggleIncludeInferredRepository = useCallback<CodyClient['toggleIncludeInferredRepository']>(() => {
        const action = scope.includeInferredRepository ? 'disable' : 'enable'
        logTranscriptEvent(
            scope.includeInferredRepository
                ? EventName.CODY_CHAT_SCOPE_INFERRED_REPO_DISABLED
                : EventName.CODY_CHAT_SCOPE_INFERRED_REPO_ENABLED,
            'cody.chat.inferredRepo',
            action
        )

        toggleIncludeInferredRepositoryInternal()

        if (transcript) {
            updateTranscriptInHistory(transcript, {
                ...scope,
                includeInferredRepository: !scope.includeInferredRepository,
            }).catch(() => null)
        }
    }, [transcript, updateTranscriptInHistory, scope, toggleIncludeInferredRepositoryInternal, logTranscriptEvent])

    const toggleIncludeInferredFile = useCallback<CodyClient['toggleIncludeInferredFile']>(() => {
        const action = scope.includeInferredFile ? 'disable' : 'enable'
        logTranscriptEvent(
            scope.includeInferredFile
                ? EventName.CODY_CHAT_SCOPE_INFERRED_FILE_DISABLED
                : EventName.CODY_CHAT_SCOPE_INFERRED_FILE_ENABLED,
            'cody.chat.inferredFile',
            action
        )

        toggleIncludeInferredFileInternal()

        if (transcript) {
            updateTranscriptInHistory(transcript, {
                ...scope,
                includeInferredFile: !scope.includeInferredFile,
            }).catch(() => null)
        }
    }, [transcript, updateTranscriptInHistory, scope, toggleIncludeInferredFileInternal, logTranscriptEvent])

    return {
        loaded,
        transcript,
        transcriptHistory,
        chatMessages,
        messageInProgress,
        isMessageInProgress,
        submitMessage,
        editMessage,
        initializeNewChat,
        clearHistory,
        deleteHistoryItem,
        loadTranscriptFromHistory,
        logTranscriptEvent,
        executeRecipe,
        scope,
        setScope,
        setEditorScope,
        toggleIncludeInferredRepository,
        toggleIncludeInferredFile,
        abortMessageInProgress,
        storageQuotaExceeded,
    }
}

export const safeTimestampToDate = (timestamp: string = ''): Date => {
    if (isNaN(new Date(timestamp) as any)) {
        return new Date()
    }

    return new Date(timestamp)
}

const sortSliceTranscriptHistory = (transcriptHistory: TranscriptJSON[]): TranscriptJSON[] =>
    transcriptHistory
        .sort(
            (a, b) =>
                (safeTimestampToDate(b.lastInteractionTimestamp) as any) -
                (safeTimestampToDate(a.lastInteractionTimestamp) as any)
        )
        .map(transcript => (transcript.id ? transcript : { ...transcript, id: Transcript.fromJSON(transcript).id }))
        .slice(0, SAVE_MAX_TRANSCRIPT_HISTORY)
