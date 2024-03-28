import type { Message } from '../sourcegraph-api'

// tracked for telemetry purposes. Which context source provided this context
// file.
//
// For now we just track "embeddings" since that is the main driver for
// understanding if it is being useful.
export type ContextFileSource = 'embeddings'

export interface ContextFile {
    fileName: string
    repoName?: string
    revision?: string

    source?: ContextFileSource
}

export interface ContextMessage extends Message {
    file?: ContextFile
    preciseContext?: PreciseContext
}

export interface PreciseContext {
    symbol: {
        fuzzyName?: string
    }
    hoverText: string[]
    definitionSnippet: string
    filePath: string
    range?: {
        startLine: number
        startCharacter: number
        endLine: number
        endCharacter: number
    }
}

export interface HoverContext {
    symbolName: string
    sourceSymbolName?: string
    type: 'definition' | 'typeDefinition' | 'implementation'
    content: string[]

    uri: string
    range?: {
        startLine: number
        startCharacter: number
        endLine: number
        endCharacter: number
    }
}

export interface OldContextMessage extends Message {
    fileName?: string
}

export function getContextMessageWithResponse(
    text: string,
    file: ContextFile,
    response: string = 'Ok.'
): ContextMessage[] {
    return [
        { speaker: 'human', text, file },
        { speaker: 'assistant', text: response },
    ]
}
