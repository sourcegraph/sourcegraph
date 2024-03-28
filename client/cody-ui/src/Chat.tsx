import React, { useCallback, useMemo, useState } from 'react'

import classNames from 'classnames'

import {
    type ChatButton,
    type ChatContextStatus,
    type ChatMessage,
    type CodyPrompt,
    isDefined,
} from '@sourcegraph/cody-shared'

import type { FileLinkProps } from './chat/ContextFiles'
import { ChatInputContext } from './chat/inputContext/ChatInputContext'
import { Transcript } from './chat/Transcript'
import type { TranscriptItemClassNames } from './chat/TranscriptItem'

import styles from './Chat.module.scss'

interface ChatProps extends ChatClassNames {
    transcript: ChatMessage[]
    messageInProgress: ChatMessage | null
    messageBeingEdited: boolean
    setMessageBeingEdited: (input: boolean) => void
    contextStatus?: ChatContextStatus | null
    formInput: string
    setFormInput: (input: string) => void
    inputHistory: string[]
    setInputHistory: (history: string[]) => void
    onSubmit: (text: string, submitType: 'user' | 'suggestion' | 'example') => void
    contextStatusComponent?: React.FunctionComponent<any>
    contextStatusComponentProps?: any
    gettingStartedComponent?: React.FunctionComponent<any>
    gettingStartedComponentProps?: any
    textAreaComponent: React.FunctionComponent<ChatUITextAreaProps>
    submitButtonComponent: React.FunctionComponent<ChatUISubmitButtonProps>
    suggestionButtonComponent?: React.FunctionComponent<ChatUISuggestionButtonProps>
    fileLinkComponent: React.FunctionComponent<FileLinkProps>
    helpMarkdown?: string
    afterMarkdown?: string
    gettingStartedButtons?: ChatButton[]
    className?: string
    EditButtonContainer?: React.FunctionComponent<EditButtonProps>
    editButtonOnSubmit?: (text: string) => void
    FeedbackButtonsContainer?: React.FunctionComponent<FeedbackButtonsProps>
    feedbackButtonsOnSubmit?: (text: string) => void
    copyButtonOnSubmit?: CopyButtonProps['copyButtonOnSubmit']
    suggestions?: string[]
    setSuggestions?: (suggestions: undefined | []) => void
    needsEmailVerification?: boolean
    needsEmailVerificationNotice?: React.FunctionComponent
    codyNotEnabledNotice?: React.FunctionComponent
    abortMessageInProgressComponent?: React.FunctionComponent<{ onAbortMessageInProgress: () => void }>
    onAbortMessageInProgress?: () => void
    isCodyEnabled: boolean
    ChatButtonComponent?: React.FunctionComponent<ChatButtonProps>
    pluginsDevMode?: boolean
    chatCommands?: [string, CodyPrompt][] | null
    ChatCommandsComponent?: React.FunctionComponent<ChatCommandsProps>
    isTranscriptError?: boolean
}

interface ChatClassNames extends TranscriptItemClassNames {
    inputRowClassName?: string
    chatInputContextClassName?: string
    chatInputClassName?: string
}

export interface ChatButtonProps {
    label: string
    action: string
    onClick: (action: string) => void
}

export interface ChatUITextAreaProps {
    className: string
    rows: number
    autoFocus: boolean
    value: string
    required: boolean
    disabled?: boolean
    onInput: React.FormEventHandler<HTMLElement>
    onKeyDown?: (event: React.KeyboardEvent<HTMLElement>, caretPosition: number | null) => void
}

export interface ChatUISubmitButtonProps {
    className: string
    disabled: boolean
    onClick: (event: React.MouseEvent<HTMLButtonElement>) => void
}

export interface ChatUISuggestionButtonProps {
    suggestion: string
    onClick: (event: React.MouseEvent<HTMLButtonElement>) => void
}

export interface EditButtonProps {
    className: string
    disabled?: boolean
    messageBeingEdited: boolean
    setMessageBeingEdited: (input: boolean) => void
}

export interface FeedbackButtonsProps {
    className: string
    disabled?: boolean
    feedbackButtonsOnSubmit: (text: string) => void
}

// TODO: Rename to CodeBlockActionsProps
export interface CopyButtonProps {
    copyButtonOnSubmit: (text: string, insert?: boolean) => void
}

export interface ChatCommandsProps {
    setFormInput: (input: string) => void
    setSelectedChatCommand: (index: number) => void
    chatCommands?: [string, CodyPrompt][] | null
    selectedChatCommand?: number
    onSubmit: (input: string, inputType: 'user' | 'suggestion') => void
}

/**
 * The Cody chat interface, with a transcript of all messages and a message form.
 */
export const Chat: React.FunctionComponent<ChatProps> = ({
    messageInProgress,
    messageBeingEdited,
    setMessageBeingEdited,
    transcript,
    contextStatus,
    formInput,
    setFormInput,
    inputHistory,
    setInputHistory,
    onSubmit,
    textAreaComponent: TextArea,
    submitButtonComponent: SubmitButton,
    suggestionButtonComponent: SuggestionButton,
    fileLinkComponent,
    helpMarkdown,
    afterMarkdown,
    gettingStartedButtons,
    className,
    codeBlocksCopyButtonClassName,
    codeBlocksInsertButtonClassName,
    transcriptItemClassName,
    humanTranscriptItemClassName,
    transcriptItemParticipantClassName,
    transcriptActionClassName,
    inputRowClassName,
    chatInputContextClassName,
    chatInputClassName,
    EditButtonContainer,
    editButtonOnSubmit,
    FeedbackButtonsContainer,
    feedbackButtonsOnSubmit,
    copyButtonOnSubmit,
    suggestions,
    setSuggestions,
    needsEmailVerification = false,
    codyNotEnabledNotice: CodyNotEnabledNotice,
    needsEmailVerificationNotice: NeedsEmailVerificationNotice,
    contextStatusComponent: ContextStatusComponent,
    contextStatusComponentProps = {},
    gettingStartedComponent: GettingStartedComponent,
    gettingStartedComponentProps = {},
    abortMessageInProgressComponent: AbortMessageInProgressButton,
    onAbortMessageInProgress = () => {},
    isCodyEnabled,
    ChatButtonComponent,
    pluginsDevMode,
    chatCommands,
    ChatCommandsComponent,
    isTranscriptError,
}) => {
    const [inputRows, setInputRows] = useState(1)
    const [displayCommands, setDisplayCommands] = useState<[string, CodyPrompt][] | null>(chatCommands || null)
    const [selectedChatCommand, setSelectedChatCommand] = useState(-1)
    const [historyIndex, setHistoryIndex] = useState(inputHistory.length)

    // Handles selecting a chat command when the user types a slash in the chat input.
    const chatCommentSelectionHandler = useCallback(
        (inputValue: string): void => {
            if (!chatCommands || !ChatCommandsComponent) {
                return
            }
            if (inputValue === '/') {
                setDisplayCommands(chatCommands)
                setSelectedChatCommand(chatCommands.length)
                return
            }
            if (inputValue.startsWith('/')) {
                const filteredCommands = chatCommands.filter(([_, prompt]) =>
                    prompt.slashCommand?.startsWith(inputValue)
                )
                setDisplayCommands(filteredCommands)
                setSelectedChatCommand(0)
                return
            }
            setDisplayCommands(null)
            setSelectedChatCommand(-1)
        },
        [ChatCommandsComponent, chatCommands]
    )

    const inputHandler = useCallback(
        (inputValue: string): void => {
            chatCommentSelectionHandler(inputValue)
            const rowsCount = (inputValue.match(/\n/g)?.length || 0) + 1
            setInputRows(rowsCount > 25 ? 25 : rowsCount)
            setFormInput(inputValue)
            if (inputValue !== inputHistory[historyIndex]) {
                setHistoryIndex(inputHistory.length)
            }
        },
        [chatCommentSelectionHandler, historyIndex, inputHistory, setFormInput]
    )

    const submitInput = useCallback(
        (input: string, submitType: 'user' | 'suggestion' | 'example'): void => {
            if (messageInProgress) {
                return
            }
            onSubmit(input, submitType)
            setSuggestions?.(undefined)
            setHistoryIndex(inputHistory.length + 1)
            setInputHistory([...inputHistory, input])
            setDisplayCommands(null)
            setSelectedChatCommand(-1)
        },
        [inputHistory, messageInProgress, onSubmit, setInputHistory, setSuggestions]
    )
    const onChatInput = useCallback(
        ({ target }: React.SyntheticEvent) => {
            const { value } = target as HTMLInputElement
            inputHandler(value)
        },
        [inputHandler]
    )

    const onChatSubmit = useCallback((): void => {
        // Submit chat only when input is not empty and not in progress
        if (formInput.trim() && !messageInProgress) {
            setInputRows(1)
            submitInput(formInput, 'user')
            setFormInput('')
        }
    }, [formInput, messageInProgress, setFormInput, submitInput])

    const onChatKeyDown = useCallback(
        (event: React.KeyboardEvent<HTMLElement>, caretPosition: number | null): void => {
            // Submit input on Enter press (without shift) and
            // trim the formInput to make sure input value is not empty.
            if (
                event.key === 'Enter' &&
                !event.shiftKey &&
                !event.nativeEvent.isComposing &&
                formInput &&
                formInput.trim() &&
                !displayCommands?.length
            ) {
                event.preventDefault()
                event.stopPropagation()
                setMessageBeingEdited(false)
                onChatSubmit()
            }

            // Ignore alt + c key combination for editor to avoid conflict with cody shortcut
            if (event.altKey && event.key === 'c') {
                event.preventDefault()
                event.stopPropagation()
            }

            // Handles cycling through chat command suggestions using the up and down arrow keys
            if (displayCommands && formInput.startsWith('/')) {
                if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
                    const commandsLength = displayCommands?.length
                    const newIndex = event.key === 'ArrowUp' ? selectedChatCommand - 1 : selectedChatCommand + 1
                    const newCommandIndex = newIndex < 0 ? commandsLength : newIndex > commandsLength ? 0 : newIndex
                    setSelectedChatCommand(newCommandIndex)
                    const newInput = displayCommands?.[newCommandIndex]?.[1]?.slashCommand
                    setFormInput(newInput || formInput)
                }
                // close the chat command suggestions on escape key
                if (event.key === 'Escape') {
                    setDisplayCommands(null)
                    setSelectedChatCommand(-1)
                    setFormInput('')
                }

                // tab/enter to complete
                if (
                    (event.key === 'Tab' || event.key === 'Enter') &&
                    selectedChatCommand > -1 &&
                    displayCommands.length
                ) {
                    event.preventDefault()
                    event.stopPropagation()
                    const newInput = displayCommands?.[selectedChatCommand]?.[1]?.slashCommand
                    setFormInput(newInput || formInput)
                    setDisplayCommands(null)
                    setSelectedChatCommand(-1)
                }
            }

            // Loop through input history on up arrow press
            if (!inputHistory.length) {
                return
            }

            // Clear & reset session on CMD+K
            if (event.metaKey && event.key === 'k') {
                onSubmit('/r', 'user')
            }

            if (formInput === inputHistory[historyIndex] || !formInput) {
                if (event.key === 'ArrowUp' && caretPosition === 0) {
                    const newIndex = historyIndex - 1 < 0 ? inputHistory.length - 1 : historyIndex - 1
                    setHistoryIndex(newIndex)
                    setFormInput(inputHistory[newIndex])
                } else if (event.key === 'ArrowDown' && caretPosition === formInput.length) {
                    if (historyIndex + 1 < inputHistory.length) {
                        const newIndex = historyIndex + 1
                        setHistoryIndex(newIndex)
                        setFormInput(inputHistory[newIndex])
                    }
                }
            }
        },
        [
            formInput,
            selectedChatCommand,
            displayCommands,
            inputHistory,
            historyIndex,
            setMessageBeingEdited,
            onChatSubmit,
            setFormInput,
            onSubmit,
        ]
    )

    const transcriptWithWelcome = useMemo<ChatMessage[]>(
        () => [
            {
                speaker: 'assistant',
                displayText: welcomeText({ helpMarkdown, afterMarkdown }),
                buttons: gettingStartedButtons,
            },
            ...transcript,
        ],
        [helpMarkdown, afterMarkdown, gettingStartedButtons, transcript]
    )

    const isGettingStartedComponentVisible = transcript.length === 0 && GettingStartedComponent !== undefined

    return (
        <div className={classNames(className, styles.innerContainer)}>
            {!isCodyEnabled && CodyNotEnabledNotice ? (
                <div className="flex-1">
                    <CodyNotEnabledNotice />
                </div>
            ) : needsEmailVerification && NeedsEmailVerificationNotice ? (
                <div className="flex-1">
                    <NeedsEmailVerificationNotice />
                </div>
            ) : (
                <Transcript
                    transcript={transcriptWithWelcome}
                    messageInProgress={messageInProgress}
                    messageBeingEdited={messageBeingEdited}
                    setMessageBeingEdited={setMessageBeingEdited}
                    fileLinkComponent={fileLinkComponent}
                    codeBlocksCopyButtonClassName={codeBlocksCopyButtonClassName}
                    codeBlocksInsertButtonClassName={codeBlocksInsertButtonClassName}
                    transcriptItemClassName={transcriptItemClassName}
                    humanTranscriptItemClassName={humanTranscriptItemClassName}
                    transcriptItemParticipantClassName={transcriptItemParticipantClassName}
                    transcriptActionClassName={transcriptActionClassName}
                    className={!isGettingStartedComponentVisible ? 'flex-1' : undefined}
                    textAreaComponent={TextArea}
                    EditButtonContainer={EditButtonContainer}
                    editButtonOnSubmit={editButtonOnSubmit}
                    FeedbackButtonsContainer={FeedbackButtonsContainer}
                    feedbackButtonsOnSubmit={feedbackButtonsOnSubmit}
                    copyButtonOnSubmit={copyButtonOnSubmit}
                    submitButtonComponent={SubmitButton}
                    chatInputClassName={chatInputClassName}
                    ChatButtonComponent={ChatButtonComponent}
                    pluginsDevMode={pluginsDevMode}
                    isTranscriptError={isTranscriptError}
                />
            )}

            {isGettingStartedComponentVisible && (
                <GettingStartedComponent {...gettingStartedComponentProps} submitInput={submitInput} />
            )}

            {/* eslint-disable-next-line react/forbid-elements */}
            <form className={classNames(styles.inputRow, inputRowClassName)}>
                {!displayCommands && suggestions !== undefined && suggestions.length !== 0 && SuggestionButton ? (
                    <div className={styles.suggestions}>
                        {suggestions.map((suggestion: string) =>
                            suggestion.trim().length > 0 ? (
                                <SuggestionButton
                                    key={suggestion}
                                    suggestion={suggestion}
                                    onClick={() => submitInput(suggestion, 'suggestion')}
                                />
                            ) : null
                        )}
                    </div>
                ) : null}
                {displayCommands && ChatCommandsComponent && formInput && (
                    <ChatCommandsComponent
                        chatCommands={displayCommands}
                        selectedChatCommand={selectedChatCommand}
                        setFormInput={setFormInput}
                        setSelectedChatCommand={setSelectedChatCommand}
                        onSubmit={onSubmit}
                    />
                )}
                {messageInProgress && AbortMessageInProgressButton && (
                    <div className={classNames(styles.abortButtonContainer)}>
                        <AbortMessageInProgressButton onAbortMessageInProgress={onAbortMessageInProgress} />
                    </div>
                )}
                <div className={styles.textAreaContainer}>
                    <TextArea
                        className={classNames(styles.chatInput, chatInputClassName)}
                        rows={inputRows}
                        value={isCodyEnabled ? formInput : 'Cody is disabled on this instance'}
                        autoFocus={true}
                        required={true}
                        disabled={needsEmailVerification || !isCodyEnabled}
                        onInput={onChatInput}
                        onKeyDown={onChatKeyDown}
                    />
                    <SubmitButton
                        className={styles.submitButton}
                        onClick={onChatSubmit}
                        disabled={
                            !!messageInProgress || needsEmailVerification || !isCodyEnabled || formInput.length === 0
                        }
                    />
                </div>
                {ContextStatusComponent ? (
                    <ContextStatusComponent {...contextStatusComponentProps} />
                ) : (
                    contextStatus && (
                        <ChatInputContext contextStatus={contextStatus} className={chatInputContextClassName} />
                    )
                )}
            </form>
        </div>
    )
}

interface WelcomeTextOptions {
    /** Provide users with a way to quickly access Cody docs/help.*/
    helpMarkdown?: string
    /** Provide additional content to supplement the original message. Example: tips, privacy policy. */
    afterMarkdown?: string
}

function welcomeText({
    helpMarkdown = 'See [Cody documentation](https://docs.sourcegraph.com/cody) for help and tips.',
    afterMarkdown,
}: WelcomeTextOptions): string {
    return ["Hello! I'm Cody. I can write code and answer questions for you. " + helpMarkdown, afterMarkdown]
        .filter(isDefined)
        .join('\n\n')
}
