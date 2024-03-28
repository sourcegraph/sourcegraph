import React, { useEffect, useRef } from 'react'

import classNames from 'classnames'

import type { ChatMessage } from '@sourcegraph/cody-shared'

import type {
    ChatButtonProps,
    ChatUISubmitButtonProps,
    ChatUITextAreaProps,
    CopyButtonProps,
    EditButtonProps,
    FeedbackButtonsProps,
} from '../Chat'

import type { FileLinkProps } from './ContextFiles'
import { TranscriptItem, type TranscriptItemClassNames } from './TranscriptItem'

import styles from './Transcript.module.scss'

export const Transcript: React.FunctionComponent<
    {
        transcript: ChatMessage[]
        messageInProgress: ChatMessage | null
        messageBeingEdited: boolean
        setMessageBeingEdited: (input: boolean) => void
        fileLinkComponent: React.FunctionComponent<FileLinkProps>
        className?: string
        textAreaComponent?: React.FunctionComponent<ChatUITextAreaProps>
        EditButtonContainer?: React.FunctionComponent<EditButtonProps>
        editButtonOnSubmit?: (text: string) => void
        FeedbackButtonsContainer?: React.FunctionComponent<FeedbackButtonsProps>
        feedbackButtonsOnSubmit?: (text: string) => void
        copyButtonOnSubmit?: CopyButtonProps['copyButtonOnSubmit']
        submitButtonComponent?: React.FunctionComponent<ChatUISubmitButtonProps>
        ChatButtonComponent?: React.FunctionComponent<ChatButtonProps>
        pluginsDevMode?: boolean
        isTranscriptError?: boolean
    } & TranscriptItemClassNames
> = React.memo(function TranscriptContent({
    transcript,
    messageInProgress,
    messageBeingEdited,
    setMessageBeingEdited,
    fileLinkComponent,
    className,
    codeBlocksCopyButtonClassName,
    codeBlocksInsertButtonClassName,
    transcriptItemClassName,
    humanTranscriptItemClassName,
    transcriptItemParticipantClassName,
    transcriptActionClassName,
    textAreaComponent,
    EditButtonContainer,
    editButtonOnSubmit,
    FeedbackButtonsContainer,
    feedbackButtonsOnSubmit,
    copyButtonOnSubmit,
    submitButtonComponent,
    chatInputClassName,
    ChatButtonComponent,
    pluginsDevMode,
    isTranscriptError,
}) {
    const transcriptContainerRef = useRef<HTMLDivElement>(null)
    useEffect(() => {
        if (transcriptContainerRef.current) {
            // Only scroll if the user didn't scroll up manually more than the scrolling threshold.
            // That is so that you can freely copy content or read up on older content while new
            // content is being produced.
            //
            // We allow some small threshold for "what is considered not scrolled up" so that
            // minimal scroll doesn't affect it (ie. if I'm not all the way scrolled down by like a
            // pixel or two, I probably still want it to scroll).
            const SCROLL_THRESHOLD = 50
            const delta = Math.abs(
                transcriptContainerRef.current.scrollHeight -
                    transcriptContainerRef.current.offsetHeight -
                    transcriptContainerRef.current.scrollTop
            )
            if (delta < SCROLL_THRESHOLD) {
                transcriptContainerRef.current.scrollTo({
                    top: transcriptContainerRef.current.scrollHeight,
                })
            }
        }
    }, [transcript, transcriptContainerRef])

    // Scroll down whenever a new message is received.
    const lastMessageSpeaker = transcript.at(-1)?.speaker
    useEffect(() => {
        transcriptContainerRef.current?.scrollTo({
            top: transcriptContainerRef.current.scrollHeight,
        })
    }, [lastMessageSpeaker])

    return (
        <div ref={transcriptContainerRef} className={classNames(className, styles.container)}>
            {transcript.map(
                (message, index) =>
                    message?.displayText && (
                        <TranscriptItem
                            // eslint-disable-next-line react/no-array-index-key
                            key={index}
                            message={message}
                            inProgress={false}
                            beingEdited={index > 0 && transcript.length - index === 2 && messageBeingEdited}
                            setBeingEdited={setMessageBeingEdited}
                            fileLinkComponent={fileLinkComponent}
                            codeBlocksCopyButtonClassName={codeBlocksCopyButtonClassName}
                            codeBlocksInsertButtonClassName={codeBlocksInsertButtonClassName}
                            transcriptItemClassName={transcriptItemClassName}
                            humanTranscriptItemClassName={humanTranscriptItemClassName}
                            transcriptItemParticipantClassName={transcriptItemParticipantClassName}
                            transcriptActionClassName={transcriptActionClassName}
                            textAreaComponent={textAreaComponent}
                            EditButtonContainer={EditButtonContainer}
                            editButtonOnSubmit={editButtonOnSubmit}
                            showEditButton={index > 0 && transcript.length - index === 2}
                            FeedbackButtonsContainer={FeedbackButtonsContainer}
                            feedbackButtonsOnSubmit={feedbackButtonsOnSubmit}
                            copyButtonOnSubmit={copyButtonOnSubmit}
                            showFeedbackButtons={index !== 0 && !isTranscriptError}
                            submitButtonComponent={submitButtonComponent}
                            chatInputClassName={chatInputClassName}
                            ChatButtonComponent={ChatButtonComponent}
                            pluginsDevMode={pluginsDevMode}
                        />
                    )
            )}
            {messageInProgress && messageInProgress.speaker === 'assistant' && (
                <TranscriptItem
                    message={messageInProgress}
                    inProgress={true}
                    beingEdited={false}
                    setBeingEdited={setMessageBeingEdited}
                    fileLinkComponent={fileLinkComponent}
                    codeBlocksCopyButtonClassName={codeBlocksCopyButtonClassName}
                    codeBlocksInsertButtonClassName={codeBlocksInsertButtonClassName}
                    transcriptItemClassName={transcriptItemClassName}
                    transcriptItemParticipantClassName={transcriptItemParticipantClassName}
                    transcriptActionClassName={transcriptActionClassName}
                    showEditButton={false}
                    showFeedbackButtons={false}
                    copyButtonOnSubmit={copyButtonOnSubmit}
                    submitButtonComponent={submitButtonComponent}
                    chatInputClassName={chatInputClassName}
                    ChatButtonComponent={ChatButtonComponent}
                />
            )}
        </div>
    )
})
