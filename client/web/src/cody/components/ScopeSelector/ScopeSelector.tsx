import React, { useEffect, useMemo, useCallback } from 'react'

import classNames from 'classnames'

import type { TranscriptJSON, CodyClientScope } from '@sourcegraph/cody-shared'
import { useLazyQuery } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { Text } from '@sourcegraph/wildcard'

import type { ReposStatusResult, ReposStatusVariables } from '../../../graphql-operations'
import { EventName } from '../../../util/constants'
import type { CodyTranscriptEventActions, CodyTranscriptEventFeatures } from '../../useCodyChat'

import { ReposStatusQuery } from './backend'
import { RepositoriesSelectorPopover, getFileName, type IRepo } from './RepositoriesSelectorPopover'

import styles from './ScopeSelector.module.scss'

export interface ScopeSelectorProps {
    scope: CodyClientScope
    setScope: (scope: CodyClientScope) => void
    toggleIncludeInferredRepository: () => void
    toggleIncludeInferredFile: () => void
    logTranscriptEvent: (
        v1EventLabel: string,
        feature: CodyTranscriptEventFeatures,
        action: CodyTranscriptEventActions,
        eventProperties?: { [key: string]: any }
    ) => void
    transcriptHistory: TranscriptJSON[]
    className?: string
    renderHint?: (repos: IRepo[]) => React.ReactNode
    // Whether to encourage the selector popover to overlap its trigger if necessary,
    // rather than collapsing or flipping position.
    encourageOverlap?: boolean
    authenticatedUser: AuthenticatedUser | null
    isFileIgnored: (path: string) => boolean
}

export const ScopeSelector: React.FC<ScopeSelectorProps> = React.memo(function ScopeSelectorComponent({
    scope,
    setScope,
    toggleIncludeInferredRepository,
    toggleIncludeInferredFile,
    logTranscriptEvent,
    transcriptHistory,
    className,
    renderHint,
    encourageOverlap,
    authenticatedUser,
    isFileIgnored,
}) {
    const [loadReposStatus, { data: newReposStatusData, previousData: previousReposStatusData }] = useLazyQuery<
        ReposStatusResult,
        ReposStatusVariables
    >(ReposStatusQuery, { fetchPolicy: 'cache-first' })

    const reposStatusData = newReposStatusData || previousReposStatusData

    const activeEditor = useMemo(() => scope.editor.getActiveTextEditor(), [scope.editor])

    const isCurrentFileIgnored = activeEditor?.filePath ? isFileIgnored(activeEditor.filePath) : false
    const inferredFilePath = (!isCurrentFileIgnored && activeEditor?.filePath) || null
    useEffect(() => {
        if (isCurrentFileIgnored && scope.includeInferredFile) {
            setScope({ ...scope, includeInferredFile: false, includeInferredRepository: true })
        }
    }, [isCurrentFileIgnored, scope, setScope])

    useEffect(() => {
        const repoNames = [...scope.repositories]

        if (activeEditor?.repoName && !repoNames.includes(activeEditor.repoName)) {
            repoNames.push(activeEditor.repoName)
        }

        if (repoNames.length === 0) {
            return
        }

        loadReposStatus({
            variables: { repoNames, first: repoNames.length },
        }).catch(() => null)
    }, [activeEditor, scope.repositories, loadReposStatus])

    const allRepositories = useMemo(() => reposStatusData?.repositories.nodes || [], [reposStatusData])

    const inferredRepository = useMemo(() => {
        if (activeEditor?.repoName) {
            return allRepositories.find(repo => repo.name === activeEditor.repoName) || null
        }

        return null
    }, [activeEditor, allRepositories])

    const additionalRepositories: IRepo[] = useMemo(
        () =>
            scope.repositories.reduce((repos, repoName) => {
                const repoStats = allRepositories.find(repo => repo.name === repoName)
                if (repoStats) {
                    repos.push(repoStats)
                }

                return repos
            }, [] as IRepo[]),
        [allRepositories, scope.repositories]
    )

    const addRepository = useCallback(
        (repoName: string) => {
            if (!scope.repositories.includes(repoName)) {
                logTranscriptEvent(EventName.CODY_CHAT_SCOPE_REPO_ADDED, 'cody.chat.scope.repo', 'add')
                setScope({ ...scope, repositories: [...scope.repositories, repoName] })
            }
        },
        [scope, setScope, logTranscriptEvent]
    )

    const removeRepository = useCallback(
        (repoName: string) => {
            logTranscriptEvent(EventName.CODY_CHAT_SCOPE_REPO_REMOVED, 'cody.chat.scope.repo', 'remove')
            setScope({ ...scope, repositories: scope.repositories.filter(repo => repo !== repoName) })
        },
        [scope, setScope, logTranscriptEvent]
    )

    const resetScope = useCallback((): void => {
        logTranscriptEvent(EventName.CODY_CHAT_SCOPE_RESET, 'cody.chat.scope.repo', 'reset')
        setScope({ ...scope, repositories: [], includeInferredRepository: true, includeInferredFile: true })
    }, [scope, setScope, logTranscriptEvent])

    return (
        <>
            <div className={classNames(styles.wrapper, className)}>
                <div className="d-flex text-truncate w-100">
                    <RepositoriesSelectorPopover
                        includeInferredRepository={scope.includeInferredRepository}
                        includeInferredFile={scope.includeInferredFile}
                        inferredRepository={inferredRepository}
                        inferredFilePath={inferredFilePath}
                        additionalRepositories={additionalRepositories}
                        addRepository={addRepository}
                        resetScope={resetScope}
                        removeRepository={removeRepository}
                        toggleIncludeInferredRepository={toggleIncludeInferredRepository}
                        toggleIncludeInferredFile={toggleIncludeInferredFile}
                        encourageOverlap={encourageOverlap}
                        transcriptHistory={transcriptHistory}
                        authenticatedUser={authenticatedUser}
                    />

                    {scope.includeInferredFile && inferredFilePath && (
                        <Text size="small" className="ml-2 mb-0 align-self-center">
                            {getFileName(inferredFilePath)}
                        </Text>
                    )}
                </div>
            </div>
            {renderHint?.(additionalRepositories)}
        </>
    )
})
