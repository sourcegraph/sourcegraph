import { ApolloError } from '@apollo/client'
import classNames from 'classnames'
import React, { FunctionComponent } from 'react'

import { ErrorAlert } from '@sourcegraph/web/src/components/alerts'
import { LoadingSpinner } from '@sourcegraph/wildcard'

import { GitObjectType } from '../../../graphql-operations'

import styles from './GitObjectPreview.module.scss'
import { GitObjectPreviewResult, usePreviewGitObjectFilter } from './useSearchGit'

export interface GitObjectPreviewWrapperProps {
    repoId: string
    type: GitObjectType
    pattern: string
}

const GitObjectHeader = <h3>Preview of Git object filter</h3>

export const GitObjectPreview: FunctionComponent<GitObjectPreviewWrapperProps> = ({ repoId, type, pattern }) => {
    if (!type || type === GitObjectType.GIT_BLOB || type === GitObjectType.GIT_UNKNOWN) {
        return (
            <>
                {GitObjectHeader}
                <small>Select a Git object type to preview matching commits.</small>
            </>
        )
    }

    return {
        [GitObjectType.GIT_COMMIT]: <GitCommitPreview repoId={repoId} pattern={pattern} typeText=" commit." />,
        [GitObjectType.GIT_TAG]: <GitTagPreview repoId={repoId} pattern={pattern} typeText=" tags." />,
        [GitObjectType.GIT_TREE]: <GitBranchesPreview repoId={repoId} pattern={pattern} typeText=" branches." />,
    }[type]
}

export interface GitPreviewProps {
    repoId: string
    pattern: string
    typeText: string
}

const createGitCommitPreview = (type: GitObjectType): FunctionComponent<GitPreviewProps> => ({
    repoId,
    pattern,
    typeText,
}) => {
    const { previewResult, isLoadingPreview, previewError } = usePreviewGitObjectFilter(repoId, type, pattern)

    return (
        <GitPreview
            typeText={typeText}
            preview={previewResult}
            previewLoading={isLoadingPreview}
            previewError={previewError}
        />
    )
}

const GitTagPreview: FunctionComponent<GitPreviewProps> = createGitCommitPreview(GitObjectType.GIT_TAG)
const GitBranchesPreview: FunctionComponent<GitPreviewProps> = createGitCommitPreview(GitObjectType.GIT_TREE)
const GitCommitPreview: FunctionComponent<GitPreviewProps> = createGitCommitPreview(GitObjectType.GIT_COMMIT)

interface GitObjectPreviewProps {
    typeText: string
    preview: GitObjectPreviewResult
    previewError: ApolloError | undefined
    previewLoading: boolean
}

const GitPreview: FunctionComponent<GitObjectPreviewProps> = ({ typeText, preview, previewError, previewLoading }) => (
    <div className={styles.wrapper}>
        {GitObjectHeader}
        <small>
            {preview.preview.length === 0 ? (
                <>Configuration policy does not match any known commits.</>
            ) : (
                <>
                    Configuration policy will be applied to the following
                    {typeText}
                </>
            )}
        </small>

        {previewError && <ErrorAlert prefix="Error fetching matching repository objects" error={previewError} />}

        {previewLoading ? (
            <LoadingSpinner className={styles.loading} />
        ) : (
            <>
                {preview.preview.length !== 0 ? (
                    <div className="mt-2 pt-2">
                        <div className={classNames('bg-dark text-light p-2', styles.container)}>
                            {preview.preview.map(tag => (
                                <p key={`${tag.repoName}@${tag.name}`} className="text-monospace p-0 m-0">
                                    <span className="search-filter-keyword">repo:</span>
                                    <span>{tag.repoName}</span>
                                    <span className="search-filter-keyword">@</span>
                                    <span>{tag.name}</span>
                                    <span className="badge badge-info ml-4">{tag.rev.slice(0, 7)}</span>
                                </p>
                            ))}
                        </div>
                    </div>
                ) : (
                    <div className="mt-2 pt-2">
                        <div className={styles.empty}>
                            <p className="text-monospace">N/A</p>
                        </div>
                    </div>
                )}
            </>
        )}
    </div>
)
