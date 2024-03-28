import { escapeRegExp } from 'lodash'
// We're using marked import here to access the `marked` package type definitions.

// eslint-disable-next-line no-restricted-imports
import { type marked, Renderer } from 'marked'
import { type Observable, forkJoin, of } from 'rxjs'
import { startWith, catchError, map, switchMap } from 'rxjs/operators'
import * as uuid from 'uuid'

import { renderMarkdown, asError, isErrorLike } from '@sourcegraph/common'
import {
    aggregateStreamingSearch,
    emptyAggregateResults,
    LATEST_VERSION,
    type SymbolMatch,
} from '@sourcegraph/shared/src/search/stream'
import type { UIRangeSpec } from '@sourcegraph/shared/src/util/url'

import type { Block, BlockInit, BlockDependencies, BlockInput, BlockDirection, SymbolBlockInput } from '..'
import { type NotebookFields, SearchPatternType } from '../../graphql-operations'
import { parseBrowserRepoURL } from '../../util/url'
import { createNotebook } from '../backend'
import { fetchSuggestions } from '../blocks/suggestions/suggestions'
import { blockToGQLInput, serializeBlockToMarkdown } from '../serialize'

import markdownBlockStyles from '../blocks/markdown/NotebookMarkdownBlock.module.scss'

const DONE = 'DONE' as const

export interface CopyNotebookProps {
    title: string
    blocks: BlockInit[]
    namespace: string
}

export function copyNotebook({ title, blocks, namespace }: CopyNotebookProps): Observable<NotebookFields> {
    return createNotebook({
        notebook: {
            title,
            blocks: blocks.map(blockToGQLInput),
            namespace,
            public: false,
        },
    })
}

function findSymbolAtRevision(
    input: Omit<SymbolBlockInput, 'revision'>,
    revision: string
): Observable<{ range: UIRangeSpec['range']; revision: string } | Error> {
    const { repositoryName, filePath, symbolName, symbolContainerName, symbolKind } = input
    return fetchSuggestions(
        `repo:${escapeRegExp(repositoryName)} file:${escapeRegExp(
            filePath
        )} rev:${revision} ${symbolName} type:symbol count:50`,
        (suggestion): suggestion is SymbolMatch => suggestion.type === 'symbol',
        symbol => symbol
    ).pipe(
        map(results => {
            const matchingFile = results.find(file => file.repository === repositoryName && file.path === filePath)
            const matchingSymbol = matchingFile?.symbols.find(
                symbol =>
                    symbol.name === symbolName &&
                    symbol.containerName === symbolContainerName &&
                    symbol.kind === symbolKind
            )
            if (!matchingFile || !matchingSymbol) {
                return new Error('Symbol not found')
            }

            const { range } = parseBrowserRepoURL(matchingSymbol.url)
            if (!range) {
                return new Error('Symbol not found')
            }
            return { range, revision: matchingFile.commit ?? '' }
        })
    )
}

export class NotebookHeadingMarkdownRenderer extends Renderer {
    public heading(
        this: marked.Renderer<never>,
        text: string,
        level: 1 | 2 | 3 | 4 | 5 | 6,
        raw: string,
        slugger: marked.Slugger
    ): string {
        const headerPrefix = this.options.headerPrefix ?? ''
        const slug = slugger.slug(raw)
        const headingId = `${slug}-${headerPrefix}`
        return `<h${level} id="${headingId}">
            <a class="${markdownBlockStyles.headingLink}" href="#${headingId}">#</a>
            <span>${text}</span>
        </h${level}>\n`
    }
}

export class Notebook {
    private blocks: Map<string, Block>
    private blockOrder: string[]

    constructor(initializerBlocks: BlockInit[], private dependencies: BlockDependencies) {
        const blocks = initializerBlocks.map(block => ({ ...block, output: null }))

        this.blocks = new Map(blocks.map(block => [block.id, block]))
        this.blockOrder = blocks.map(block => block.id)

        // Pre-run certain blocks, for a better user experience.
        for (const block of blocks) {
            if (block.type === 'md' || block.type === 'file' || block.type === 'symbol') {
                this.runBlockById(block.id)
            }
        }
    }

    public getBlockIndex(id: string): number {
        return this.blockOrder.indexOf(id)
    }

    public getBlocks(): Block[] {
        return this.blockOrder.map(blockId => {
            const block = this.blocks.get(blockId)
            if (!block) {
                throw new Error(`Block with id:${blockId} does not exist.`)
            }
            return block
        })
    }

    public setBlockInputById(id: string, { type, input }: BlockInput): void {
        const block = this.blocks.get(id)
        if (!block) {
            return
        }
        if (block.type !== type) {
            throw new Error(`Input block type ${type} does not match existing block type ${block.type}.`)
        }
        // We checked that the existing block and the input block have the same type, so the cast below is safe.
        this.blocks.set(block.id, { ...block, input } as Block)
    }

    public runBlockById(id: string): void {
        const block = this.blocks.get(id)
        if (!block) {
            return
        }
        switch (block.type) {
            case 'md': {
                this.blocks.set(block.id, {
                    ...block,
                    output: renderMarkdown(block.input.text, {
                        renderer: new NotebookHeadingMarkdownRenderer(),
                        headerPrefix: block.id,
                    }),
                })
                break
            }
            case 'query': {
                // Removes comments
                const query = block.input.query.replaceAll(/\/\/.*/g, '')
                this.blocks.set(block.id, {
                    ...block,
                    output: aggregateStreamingSearch(of(query), {
                        version: LATEST_VERSION,
                        patternType: SearchPatternType.standard,
                        caseSensitive: false,
                        trace: undefined,
                        chunkMatches: true,
                    }).pipe(startWith(emptyAggregateResults)),
                })
                break
            }
            case 'file': {
                this.blocks.set(block.id, {
                    ...block,
                    output: this.dependencies
                        .fetchHighlightedFileLineRanges({
                            repoName: block.input.repositoryName,
                            commitID: block.input.revision || 'HEAD',
                            filePath: block.input.filePath,
                            ranges: block.input.lineRange
                                ? [block.input.lineRange]
                                : [{ startLine: 0, endLine: 2147483647 }], // entire file,
                            disableTimeout: false,
                        })
                        .pipe(
                            map(ranges => ranges[0]),
                            catchError(() => [asError('File not found')])
                        ),
                })
                break
            }
            case 'symbol': {
                // Start by searching for the symbol at the latest HEAD (main) revision.
                const output = findSymbolAtRevision(block.input, 'HEAD').pipe(
                    switchMap(symbolSearchResult => {
                        if (!isErrorLike(symbolSearchResult)) {
                            return of({ ...symbolSearchResult, symbolFoundAtLatestRevision: true })
                        }
                        // If not found, look at the revision stored in the block input (should always be found).
                        return findSymbolAtRevision(block.input, block.input.revision).pipe(
                            map(symbolSearchResult =>
                                !isErrorLike(symbolSearchResult)
                                    ? { ...symbolSearchResult, symbolFoundAtLatestRevision: false }
                                    : symbolSearchResult
                            )
                        )
                    }),
                    switchMap(symbolSearchResult => {
                        if (isErrorLike(symbolSearchResult)) {
                            return of(symbolSearchResult)
                        }
                        const { repositoryName, filePath, lineContext } = block.input
                        const { range, revision, symbolFoundAtLatestRevision } = symbolSearchResult
                        const highlightLineRange = {
                            startLine: Math.max(range.start.line - 1 - lineContext, 0),
                            endLine: range.end.line + lineContext,
                        }
                        const highlightSymbolRange = {
                            startLine: range.start.line - 1,
                            startCharacter: range.start.character - 1,
                            endLine: range.end.line - 1,
                            endCharacter: range.end.character - 1,
                        }
                        return this.dependencies
                            .fetchHighlightedFileLineRanges({
                                repoName: repositoryName,
                                commitID: revision,
                                filePath,
                                ranges: [highlightLineRange],
                                disableTimeout: false,
                            })
                            .pipe(
                                map(ranges => ({
                                    highlightedLines: ranges[0],
                                    highlightLineRange,
                                    highlightSymbolRange,
                                    symbolRange: range,
                                    effectiveRevision: revision,
                                    symbolFoundAtLatestRevision,
                                }))
                            )
                    }),
                    catchError(error => [asError(error)])
                )
                this.blocks.set(block.id, { ...block, output })
                break
            }
        }
    }

    public runAllBlocks(): Observable<typeof DONE[]> {
        const observables: Observable<typeof DONE>[] = []
        // Iterate over block ids and run each block. We do not iterate over values
        // because `runBlockById` method assigns a new value for the id so we have
        // to fetch the block value separately.
        for (const blockId of this.blocks.keys()) {
            this.runBlockById(blockId)

            const block = this.getBlockById(blockId)
            if (!block?.output) {
                continue
            }
            // Identical if/else if branches to make the TS compiler happy
            if (block.type === 'query') {
                observables.push(block.output.pipe(map(() => DONE)))
            } else if (block.type === 'file') {
                observables.push(block.output.pipe(map(() => DONE)))
            } else if (block.type === 'symbol') {
                observables.push(block.output.pipe(map(() => DONE)))
            }
        }
        // We store output observables and join them into a single observable,
        // to let the caller know when all async outputs have finished.
        return observables.length > 0 ? forkJoin(observables) : of([DONE])
    }

    public deleteBlockById(id: string): void {
        const index = this.blockOrder.indexOf(id)
        if (index === -1) {
            return
        }
        this.blocks.delete(id)
        this.blockOrder.splice(index, 1)
    }

    public insertBlockAtIndex(index: number, blockToInsert: BlockInput): Block {
        const id = uuid.v4()
        const block = { ...blockToInsert, id, output: null }
        // Insert block at the provided index
        this.blockOrder.splice(index, 0, id)
        this.blocks.set(id, block)
        return block
    }

    public getBlockById(id: string): Block | null {
        return this.blocks.get(id) ?? null
    }

    public moveBlockById(id: string, direction: BlockDirection): void {
        const index = this.blockOrder.indexOf(id)
        if ((direction === 'up' && index < 1) || (direction === 'down' && index === this.blockOrder.length - 1)) {
            return
        }
        const swapIndex = direction === 'up' ? index - 1 : index + 1
        const temporaryId = this.blockOrder[swapIndex]
        this.blockOrder[swapIndex] = id
        this.blockOrder[index] = temporaryId
    }

    public duplicateBlockById(id: string): Block | null {
        const block = this.blocks.get(id)
        if (!block) {
            return null
        }
        const index = this.blockOrder.indexOf(id)
        return this.insertBlockAtIndex(index + 1, block)
    }

    public getFirstBlockId(): string | null {
        return this.blockOrder.length > 0 ? this.blockOrder[0] : null
    }

    public getLastBlockId(): string | null {
        return this.blockOrder.length > 0 ? this.blockOrder.at(-1)! : null
    }

    public getPreviousBlockId(id: string): string | null {
        const index = this.blockOrder.indexOf(id)
        return index >= 1 ? this.blockOrder[index - 1] : null
    }

    public getNextBlockId(id: string): string | null {
        const index = this.blockOrder.indexOf(id)
        return index >= 0 && index < this.blockOrder.length - 1 ? this.blockOrder[index + 1] : null
    }

    public exportToMarkdown(sourcegraphURL: string): Observable<string> {
        const serializedBlocks = this.getBlocks().map(block => serializeBlockToMarkdown(block, sourcegraphURL))
        return forkJoin(serializedBlocks).pipe(
            map(blocks => blocks.filter(block => block.length > 0).join('\n\n') + '\n')
        )
    }
}
