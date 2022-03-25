import React, { useMemo } from 'react'

import { noop } from 'lodash'
import { Observable } from 'rxjs'

import { StreamingSearchResultsListProps } from '@sourcegraph/search-ui'
import { ExtensionsControllerProps } from '@sourcegraph/shared/src/extensions/controller'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { NotebookBlock } from '@sourcegraph/shared/src/schema'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'

import { Block, BlockInit } from '..'
import { NotebookFields } from '../../graphql-operations'
import { SearchStreamingProps } from '../../search'
import { CopyNotebookProps } from '../notebook'
import { NotebookComponentMemoized } from '../notebook/NotebookComponent'

export interface NotebookContentProps
    extends SearchStreamingProps,
        ThemeProps,
        TelemetryProps,
        Omit<StreamingSearchResultsListProps, 'allExpanded' | 'extensionsController' | 'platformContext'>,
        PlatformContextProps<'sourcegraphURL' | 'requestGraphQL' | 'urlToFile' | 'settings' | 'forceUpdateTooltip'>,
        ExtensionsControllerProps<'extHostAPI' | 'executeCommand'> {
    globbing: boolean
    viewerCanManage: boolean
    blocks: NotebookBlock[]
    exportedFileName: string
    isEmbedded?: boolean
    onUpdateBlocks: (blocks: Block[]) => void
    onCopyNotebook: (props: Omit<CopyNotebookProps, 'title'>) => Observable<NotebookFields>
}

const NotebookContent: React.FunctionComponent<NotebookContentProps> = ({
    viewerCanManage,
    blocks,
    exportedFileName,
    onCopyNotebook,
    onUpdateBlocks,
    globbing,
    streamSearch,
    isLightTheme,
    telemetryService,
    searchContextsEnabled,
    isSourcegraphDotCom,
    fetchHighlightedFileLineRanges,
    authenticatedUser,
    showSearchContext,
    settingsCascade,
    platformContext,
    extensionsController,
}) => {
    const initializerBlocks: BlockInit[] = useMemo(
        () =>
            blocks.map(block => {
                switch (block.__typename) {
                    case 'MarkdownBlock':
                        return { id: block.id, type: 'md', input: { text: block.markdownInput } }
                    case 'QueryBlock':
                        return { id: block.id, type: 'query', input: { query: block.queryInput } }
                    case 'FileBlock':
                        return {
                            id: block.id,
                            type: 'file',
                            input: { ...block.fileInput, revision: block.fileInput.revision ?? '' },
                        }
                    case 'SymbolBlock':
                        return {
                            id: block.id,
                            type: 'symbol',
                            input: { ...block.symbolInput, revision: block.symbolInput.revision ?? '' },
                        }
                    case 'ComputeBlock':
                        return {
                            id: block.id,
                            type: 'compute',
                            input: block.computeInput,
                        }
                }
            }),
        [blocks]
    )

    return (
        <NotebookComponentMemoized
            globbing={globbing}
            streamSearch={streamSearch}
            isLightTheme={isLightTheme}
            telemetryService={telemetryService}
            searchContextsEnabled={searchContextsEnabled}
            isSourcegraphDotCom={isSourcegraphDotCom}
            fetchHighlightedFileLineRanges={fetchHighlightedFileLineRanges}
            authenticatedUser={authenticatedUser}
            showSearchContext={showSearchContext}
            settingsCascade={settingsCascade}
            platformContext={platformContext}
            extensionsController={extensionsController}
            isReadOnly={!viewerCanManage}
            blocks={initializerBlocks}
            onSerializeBlocks={viewerCanManage ? onUpdateBlocks : noop}
            exportedFileName={exportedFileName}
            onCopyNotebook={onCopyNotebook}
        />
    )
}

export const NotebookContentMemoized = React.memo(NotebookContent)
