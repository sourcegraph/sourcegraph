import { useMemo, useCallback } from 'react'

import { mdiDelete } from '@mdi/js'
import classNames from 'classnames'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import type { Transcript, TranscriptJSON } from '@sourcegraph/cody-shared'
import { Text, Icon, Tooltip } from '@sourcegraph/wildcard'

import { type CodyChatStore, safeTimestampToDate } from '../../useCodyChat'

import styles from './HistoryList.module.scss'

interface HistoryListProps {
    transcriptHistory: TranscriptJSON[]
    currentTranscript: Transcript | null
    loadTranscriptFromHistory: CodyChatStore['loadTranscriptFromHistory']
    deleteHistoryItem: CodyChatStore['deleteHistoryItem']
    truncateMessageLength?: number
    itemClassName?: string
}

export const HistoryList: React.FunctionComponent<HistoryListProps> = ({
    currentTranscript,
    transcriptHistory,
    truncateMessageLength,
    loadTranscriptFromHistory,
    deleteHistoryItem,
    itemClassName,
}) =>
    transcriptHistory.length === 0 ? (
        <Text className="p-2 pb-0 text-muted text-center">No chats yet</Text>
    ) : (
        <div className="p-0 d-flex flex-column">
            {transcriptHistory.map(transcript => (
                <HistoryListItem
                    key={transcript.id}
                    currentTranscript={currentTranscript}
                    transcript={transcript}
                    className={itemClassName}
                    truncateMessageLength={truncateMessageLength}
                    loadTranscriptFromHistory={loadTranscriptFromHistory}
                    deleteHistoryItem={deleteHistoryItem}
                />
            ))}
        </div>
    )

const HistoryListItem: React.FunctionComponent<{
    currentTranscript: Transcript | null
    transcript: TranscriptJSON
    loadTranscriptFromHistory: CodyChatStore['loadTranscriptFromHistory']
    deleteHistoryItem: CodyChatStore['deleteHistoryItem']
    truncateMessageLength?: number
    className?: string
}> = ({
    currentTranscript,
    transcript: { id, interactions, lastInteractionTimestamp },
    truncateMessageLength = 80,
    loadTranscriptFromHistory,
    deleteHistoryItem,
    className,
}) => {
    const lastMessage = useMemo(() => {
        let message = null

        for (let index = interactions.length - 1; index >= 0; index--) {
            const { assistantMessage, humanMessage } = interactions[index]

            if (assistantMessage?.text) {
                message = assistantMessage
            } else if (humanMessage?.text) {
                message = humanMessage
            }

            if (message) {
                break
            }
        }

        return message
    }, [interactions])

    const deleteItem = useCallback(
        (event: React.SyntheticEvent) => {
            event.stopPropagation()
            deleteHistoryItem(id)
        },
        [deleteHistoryItem, id]
    )

    const text = lastMessage?.text || 'Write your first message.'

    return (
        <button
            key={id}
            type="button"
            className={classNames(
                'text-left',
                styles.historyItem,
                {
                    [styles.selected]: currentTranscript?.id === id,
                },
                className
            )}
            onClick={() => loadTranscriptFromHistory(id)}
            onKeyDown={() => loadTranscriptFromHistory(id)}
        >
            <div className="d-flex align-items-center mb-1 justify-content-between w-100">
                <Text className="mb-1 text-muted" size="small">
                    <Timestamp date={safeTimestampToDate(lastInteractionTimestamp)} />
                </Text>
                {!!interactions.length && (
                    <Tooltip content="Delete">
                        <Icon
                            aria-label="Delete"
                            svgPath={mdiDelete}
                            onClick={deleteItem}
                            className={styles.deleteButton}
                        />
                    </Tooltip>
                )}
            </div>
            <Text className="mb-0 truncate text-body">
                {text.slice(0, truncateMessageLength)}
                {text.length > truncateMessageLength ? '...' : ''}
            </Text>
        </button>
    )
}
