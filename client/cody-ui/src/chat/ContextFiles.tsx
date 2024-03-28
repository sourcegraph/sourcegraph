import React from 'react'

import { mdiFileDocumentOutline, mdiMagnify } from '@mdi/js'

import { type ContextFile, pluralize } from '@sourcegraph/cody-shared'

import { TranscriptAction } from './actions/TranscriptAction'

export interface FileLinkProps {
    path: string
    repoName?: string
    revision?: string
}

export const ContextFiles: React.FunctionComponent<{
    contextFiles: ContextFile[]
    fileLinkComponent: React.FunctionComponent<FileLinkProps>
    className?: string
}> = React.memo(function ContextFilesContent({ contextFiles, fileLinkComponent: FileLink, className }) {
    const uniqueFiles = new Set<string>()
    const filteredFiles = contextFiles.filter(file => {
        if (uniqueFiles.has(file.fileName)) {
            return false
        }
        uniqueFiles.add(file.fileName)
        return true
    })

    return (
        <TranscriptAction
            title={{ verb: 'Read', object: `${filteredFiles.length} ${pluralize('file', filteredFiles.length)}` }}
            steps={[
                { verb: 'Searched', object: 'entire codebase for relevant files', icon: mdiMagnify },
                ...filteredFiles.map(file => ({
                    verb: '',
                    object: <FileLink path={file.fileName} repoName={file.repoName} revision={file.revision} />,
                    icon: mdiFileDocumentOutline,
                })),
            ]}
            className={className}
        />
    )
})
