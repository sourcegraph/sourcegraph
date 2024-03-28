import React, { type FC, useRef, useState, useEffect, useMemo } from 'react'

import { mdiFileDocumentOutline, mdiFolderOutline } from '@mdi/js'
import classNames from 'classnames'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { NoopEditor } from '@sourcegraph/cody-shared'
import { basename } from '@sourcegraph/common'
import { gql } from '@sourcegraph/http-client'
import type { TreeFields } from '@sourcegraph/shared/src/graphql-operations'
import { useSettings } from '@sourcegraph/shared/src/settings/settings'
import {
    Card,
    CardHeader,
    H2,
    Icon,
    LanguageIcon,
    Link,
    LinkOrSpan,
    LoadingSpinner,
    useElementObscuredArea,
} from '@sourcegraph/wildcard'

import { FileContentEditor } from '../../cody/components/FileContentEditor'
import { useCodySidebar } from '../../cody/sidebar/Provider'
import type { BlobFileFields, TreeHistoryFields } from '../../graphql-operations'
import { fetchBlob } from '../blob/backend'
import { RenderedFile } from '../blob/RenderedFile'
import { CommitMessageWithLinks } from '../commit/CommitMessageWithLinks'

import styles from './TreePagePanels.module.scss'

export const treeHistoryFragment = gql`
    fragment TreeHistoryFields on TreeEntry {
        path
        history(first: 1) {
            nodes {
                commit {
                    id
                    canonicalURL
                    subject
                    author {
                        date
                    }
                    committer {
                        date
                    }
                    externalURLs {
                        url
                        serviceKind
                    }
                }
            }
        }
    }
`

interface ReadmePreviewCardProps {
    entry: TreeFields['entries'][number]
    repoName: string
    revision: string
    className?: string
}
export const ReadmePreviewCard: React.FunctionComponent<ReadmePreviewCardProps> = ({
    entry,
    repoName,
    revision,
    className,
}) => {
    const [readmeInfo, setReadmeInfo] = useState<null | BlobFileFields>(null)
    const { setEditorScope } = useCodySidebar()

    useEffect(() => {
        const subscription = fetchBlob({
            repoName,
            revision,
            filePath: entry.path,
            disableTimeout: true,
        }).subscribe(blob => {
            if (blob) {
                setReadmeInfo(blob)
            } else {
                setReadmeInfo(null)
            }
        })
        return () => subscription.unsubscribe()
    }, [repoName, revision, entry.path])

    useEffect(() => {
        if (readmeInfo) {
            setEditorScope(
                new FileContentEditor({ filePath: entry.path, repoName, revision, content: readmeInfo.content })
            )
        }

        return () => {
            if (readmeInfo) {
                setEditorScope(new NoopEditor())
            }
        }
    }, [repoName, revision, entry.path, readmeInfo, setEditorScope])

    return (
        <section className={classNames('mb-4', className)}>
            {readmeInfo ? (
                <RenderedReadmeFile blob={readmeInfo} entryUrl={entry.url} />
            ) : (
                <div className={classNames('text-muted', styles.readmeLoading)}>
                    <LoadingSpinner />
                </div>
            )}
        </section>
    )
}

interface RenderedReadmeFileProps {
    blob: BlobFileFields
    entryUrl: string
}
const RenderedReadmeFile: React.FC<RenderedReadmeFileProps> = ({ blob, entryUrl }) => {
    const renderedFileRef = useRef<HTMLDivElement>(null)
    const { bottom } = useElementObscuredArea(renderedFileRef)
    return (
        <>
            {blob.richHTML ? (
                <RenderedFile ref={renderedFileRef} dangerousInnerHTML={blob.richHTML} className={styles.readme} />
            ) : (
                <div ref={renderedFileRef} className={styles.readme}>
                    <H2 className={styles.readmePreHeader}>{basename(entryUrl)}</H2>
                    <pre className={styles.readmePre}>{blob.content}</pre>
                </div>
            )}
            {bottom > 0 && (
                <>
                    <div className={styles.readmeFader} />
                    <Link to={entryUrl} className={styles.readmeMoreLink}>
                        View full README
                    </Link>
                </>
            )}
        </>
    )
}

export interface DiffStat {
    path: string
    added: number
    deleted: number
}

export interface FilePanelProps {
    entries: TreeFields['entries']
    historyEntries?: TreeHistoryFields[]
    className?: string
}

export const FilesCard: FC<FilePanelProps> = ({ entries, historyEntries, className }) => {
    const settings = useSettings()
    const preferAbsoluteTimestamps = Boolean(settings?.['history.preferAbsoluteTimestamps'])
    const hasHistoryEntries = historyEntries && historyEntries.length > 0
    const fileHistoryByPath = useMemo(() => {
        const fileHistoryByPath: Record<string, TreeHistoryFields['history']['nodes'][number]['commit']> = {}
        for (const entry of historyEntries || []) {
            if (entry.history.nodes.length > 0) {
                fileHistoryByPath[entry.path] = entry.history.nodes[0].commit
            }
        }
        return fileHistoryByPath
    }, [historyEntries])

    return (
        <Card as="table" className={classNames(className, styles.files)}>
            <thead>
                <CardHeader as="tr">
                    <th className={styles.fileNameColumn}>File</th>
                    {hasHistoryEntries && (
                        <>
                            <th>Last commit message</th>
                            <th
                                className={classNames(
                                    styles.commitDateColumn,
                                    preferAbsoluteTimestamps && styles.absolute
                                )}
                            >
                                Last commit date
                            </th>
                        </>
                    )}
                </CardHeader>
            </thead>
            <tbody>
                {entries.map(entry => (
                    <tr key={entry.name}>
                        <td className={styles.fileName}>
                            <LinkOrSpan
                                to={entry.url}
                                className={classNames(
                                    'test-page-file-decorable',
                                    entry.isDirectory && 'font-weight-bold',
                                    `test-tree-entry-${entry.isDirectory ? 'directory' : 'file'}`
                                )}
                                title={entry.path}
                                data-testid="tree-entry"
                            >
                                {entry.__typename === 'GitBlob' ? (
                                    <LanguageIcon
                                        language={entry.languages.at(0) ?? ''}
                                        fileNameOrExtensions={entry.name}
                                        className="mr-1"
                                    />
                                ) : (
                                    <Icon
                                        svgPath={entry.isDirectory ? mdiFolderOutline : mdiFileDocumentOutline}
                                        className="mr-1"
                                        aria-hidden={true}
                                    />
                                )}
                                {entry.name}
                                {entry.isDirectory && '/'}
                            </LinkOrSpan>
                        </td>
                        {fileHistoryByPath[entry.path] && (
                            <>
                                <td className={styles.commitMessage}>
                                    <span
                                        title={fileHistoryByPath[entry.path].subject}
                                        data-testid="git-commit-message-with-links"
                                    >
                                        <CommitMessageWithLinks
                                            to={fileHistoryByPath[entry.path].canonicalURL}
                                            message={fileHistoryByPath[entry.path].subject}
                                            className="text-muted"
                                            externalURLs={fileHistoryByPath[entry.path].externalURLs}
                                        />
                                    </span>
                                </td>
                                <td className={classNames(styles.commitDate, 'text-muted')}>
                                    <Timestamp
                                        noAbout={true}
                                        preferAbsolute={preferAbsoluteTimestamps}
                                        date={getCommitDate(fileHistoryByPath[entry.path])}
                                    />
                                </td>
                            </>
                        )}
                    </tr>
                ))}
            </tbody>
        </Card>
    )
}

function getCommitDate(commit: { author: { date: string }; committer?: { date: string } | null }): string {
    return commit.committer ? commit.committer.date : commit.author.date
}
