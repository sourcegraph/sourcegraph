import type { Configuration } from '../configuration'
import type { EmbeddingsSearch } from '../embeddings'
import type { GraphContextFetcher } from '../graph-context'
import type {
    ContextResult,
    FilenameContextFetcher,
    IndexedKeywordContextFetcher,
    KeywordContextFetcher,
} from '../local-context'
import {
    isMarkdownFile,
    populateCodeContextTemplate,
    populateMarkdownContextTemplate,
    populatePreciseCodeContextTemplate,
} from '../prompt/templates'
import type { Message } from '../sourcegraph-api'
import type { EmbeddingsSearchResult } from '../sourcegraph-api/graphql/client'
import type { UnifiedContextFetcher } from '../unified-context'
import { isError } from '../utils'

import {
    type ContextFile,
    type ContextFileSource,
    type ContextMessage,
    getContextMessageWithResponse,
} from './messages'

export interface ContextSearchOptions {
    numCodeResults: number
    numTextResults: number
}

export class CodebaseContext {
    private embeddingResultsError = ''

    constructor(
        private config: Pick<Configuration, 'useContext' | 'serverEndpoint' | 'experimentalLocalSymbols'>,
        private codebase: string | undefined,
        private embeddings: EmbeddingsSearch | null,
        private keywords: KeywordContextFetcher | null,
        private filenames: FilenameContextFetcher | null,
        private graph: GraphContextFetcher | null,
        public symf?: IndexedKeywordContextFetcher,
        private unifiedContextFetcher?: UnifiedContextFetcher | null,
        private rerank?: (query: string, results: ContextResult[]) => Promise<ContextResult[]>
    ) {}

    public getCodebase(): string | undefined {
        return this.codebase
    }

    public onConfigurationChange(newConfig: typeof this.config): void {
        this.config = newConfig
    }

    private mergeContextResults(keywordResults: ContextResult[], filenameResults: ContextResult[]): ContextResult[] {
        // Just take the single most relevant filename suggestion for now. Otherwise, because our reranking relies solely
        // on filename, the filename results would dominate the keyword results.
        const merged = filenameResults.slice(-1).concat(keywordResults)

        const uniques = new Map<string, ContextResult>()
        for (const result of merged) {
            uniques.set(result.fileName, result)
        }

        return Array.from(uniques.values())
    }

    /**
     * Returns context messages from both generic contexts and graph-based contexts.
     * The final list is a combination of these two sets of messages.
     */
    public async getCombinedContextMessages(query: string, options: ContextSearchOptions): Promise<ContextMessage[]> {
        const contextMessages = this.getContextMessages(query, options)
        const graphContextMessages = this.getGraphContextMessages()

        // TODO(efritz) - open problem to figure out how to best rank these into a unified list
        return [...(await contextMessages), ...(await graphContextMessages)]
    }

    /**
     * Returns list of context messages for a given query, sorted in *reverse* order of importance (that is,
     * the most important context message appears *last*)
     */
    public async getContextMessages(query: string, options: ContextSearchOptions): Promise<ContextMessage[]> {
        switch (this.config.useContext) {
            case 'unified': {
                return this.getUnifiedContextMessages(query, options)
            }
            case 'keyword': {
                return this.getLocalContextMessages(query, options)
            }
            case 'none': {
                return []
            }
            default: {
                return this.embeddings
                    ? this.getEmbeddingsContextMessages(query, options)
                    : this.getLocalContextMessages(query, options)
            }
        }
    }

    public checkEmbeddingsConnection(): boolean {
        return !!this.embeddings
    }

    public getEmbeddingSearchErrors(): string {
        return this.embeddingResultsError.trim()
    }

    public async getSearchResults(
        query: string,
        options: ContextSearchOptions
    ): Promise<{ results: ContextResult[] | EmbeddingsSearchResult[]; endpoint: string }> {
        if (this.embeddings && this.config.useContext !== 'keyword') {
            return {
                results: await this.getEmbeddingSearchResults(query, options),
                endpoint: this.config.serverEndpoint,
            }
        }
        return {
            results:
                (await this.keywords?.getSearchContext(query, options.numCodeResults + options.numTextResults)) || [],
            endpoint: this.config.serverEndpoint,
        }
    }

    // We split the context into multiple messages instead of joining them into a single giant message.
    // We can gradually eliminate them from the prompt, instead of losing them all at once with a single large messeage
    // when we run out of tokens.
    private async getEmbeddingsContextMessages(
        query: string,
        options: ContextSearchOptions
    ): Promise<ContextMessage[]> {
        const combinedResults = await this.getEmbeddingSearchResults(query, options)

        return groupResultsByFile(combinedResults)
            .reverse() // Reverse results so that they appear in ascending order of importance (least -> most).
            .flatMap(groupedResults => this.makeContextMessageWithResponse(groupedResults))
            .map(message => contextMessageWithSource(message, 'embeddings'))
    }

    private async getEmbeddingSearchResults(
        query: string,
        options: ContextSearchOptions
    ): Promise<EmbeddingsSearchResult[]> {
        if (!this.embeddings) {
            return []
        }

        const embeddingsSearchResults = await this.embeddings.search(
            query,
            options.numCodeResults,
            options.numTextResults
        )

        if (isError(embeddingsSearchResults)) {
            // eslint-disable-next-line no-console
            console.error('Error retrieving embeddings:', embeddingsSearchResults)
            this.embeddingResultsError = `Error retrieving embeddings: ${embeddingsSearchResults}`
            return []
        }
        this.embeddingResultsError = ''
        return embeddingsSearchResults.codeResults.concat(embeddingsSearchResults.textResults)
    }

    private makeContextMessageWithResponse(groupedResults: { file: ContextFile; results: string[] }): ContextMessage[] {
        const contextTemplateFn = isMarkdownFile(groupedResults.file.fileName)
            ? populateMarkdownContextTemplate
            : populateCodeContextTemplate

        return groupedResults.results.flatMap<Message>(text =>
            getContextMessageWithResponse(
                contextTemplateFn(text, groupedResults.file.fileName, groupedResults.file.repoName),
                groupedResults.file
            )
        )
    }

    private async getUnifiedContextMessages(query: string, options: ContextSearchOptions): Promise<ContextMessage[]> {
        if (!this.unifiedContextFetcher) {
            return []
        }

        const results = await this.unifiedContextFetcher.getContext(
            query,
            options.numCodeResults,
            options.numTextResults
        )

        if (isError(results)) {
            // eslint-disable-next-line no-console
            console.error('Error retrieving context:', results)
            return []
        }

        return results.flatMap(result => {
            if (result?.type === 'FileChunkContext') {
                const { content, filePath, repoName, revision } = result
                const messageText = isMarkdownFile(filePath)
                    ? populateMarkdownContextTemplate(content, filePath, repoName)
                    : populateCodeContextTemplate(content, filePath, repoName)

                return getContextMessageWithResponse(messageText, { fileName: filePath, repoName, revision })
            }

            return []
        })
    }

    private async getLocalContextMessages(query: string, options: ContextSearchOptions): Promise<ContextMessage[]> {
        try {
            const keywordResultsPromise = this.getKeywordSearchResults(query, options)
            const filenameResultsPromise = this.getFilenameSearchResults(query, options)

            const [keywordResults, filenameResults] = await Promise.all([keywordResultsPromise, filenameResultsPromise])

            const combinedResults = this.mergeContextResults(keywordResults, filenameResults)
            const rerankedResults = await (this.rerank ? this.rerank(query, combinedResults) : combinedResults)
            const messages = resultsToMessages(rerankedResults)

            this.embeddingResultsError = ''

            return messages
        } catch (error) {
            // eslint-disable-next-line no-console
            console.error('Error retrieving local context:', error)
            this.embeddingResultsError = `Error retrieving local context: ${error}`
            return []
        }
    }

    private async getKeywordSearchResults(query: string, options: ContextSearchOptions): Promise<ContextResult[]> {
        if (!this.keywords) {
            return []
        }
        const results = await this.keywords.getContext(query, options.numCodeResults + options.numTextResults)
        return results
    }

    private async getFilenameSearchResults(query: string, options: ContextSearchOptions): Promise<ContextResult[]> {
        if (!this.filenames) {
            return []
        }
        const results = await this.filenames.getContext(query, options.numCodeResults + options.numTextResults)
        return results
    }

    public async getGraphContextMessages(): Promise<ContextMessage[]> {
        if (!this.config.experimentalLocalSymbols || !this.graph) {
            return []
        }
        // eslint-disable-next-line no-console
        console.debug('Fetching graph context')

        const contextMessages: ContextMessage[] = []
        for (const preciseContext of await this.graph.getContext()) {
            const text = populatePreciseCodeContextTemplate(
                preciseContext.symbol.fuzzyName || 'unknown',
                preciseContext.filePath,
                preciseContext.definitionSnippet
            )

            contextMessages.push({ speaker: 'human', preciseContext, text }, { speaker: 'assistant', text: 'okay' })
        }

        return contextMessages
    }
}

function groupResultsByFile(results: EmbeddingsSearchResult[]): { file: ContextFile; results: string[] }[] {
    const originalFileOrder: ContextFile[] = []
    for (const result of results) {
        if (!originalFileOrder.find((ogFile: ContextFile) => ogFile.fileName === result.fileName)) {
            originalFileOrder.push({ fileName: result.fileName, repoName: result.repoName, revision: result.revision })
        }
    }

    const resultsGroupedByFile = new Map<string, EmbeddingsSearchResult[]>()
    for (const result of results) {
        const results = resultsGroupedByFile.get(result.fileName)
        if (results === undefined) {
            resultsGroupedByFile.set(result.fileName, [result])
        } else {
            resultsGroupedByFile.set(result.fileName, results.concat([result]))
        }
    }

    return originalFileOrder.map(file => ({
        file,
        results: mergeConsecutiveResults(resultsGroupedByFile.get(file.fileName)!),
    }))
}

function mergeConsecutiveResults(results: EmbeddingsSearchResult[]): string[] {
    const sortedResults = results.sort((a, b) => a.startLine - b.startLine)
    const mergedResults = [results[0].content]

    for (let i = 1; i < sortedResults.length; i++) {
        const result = sortedResults[i]
        const previousResult = sortedResults[i - 1]

        if (result.startLine === previousResult.endLine) {
            mergedResults[mergedResults.length - 1] = mergedResults.at(-1)! + result.content
        } else {
            mergedResults.push(result.content)
        }
    }

    return mergedResults
}

function resultsToMessages(results: ContextResult[]): ContextMessage[] {
    return results.flatMap(({ content, fileName, repoName, revision }) => {
        const messageText = populateCodeContextTemplate(content, fileName, repoName)
        return getContextMessageWithResponse(messageText, { fileName, repoName, revision })
    })
}

function contextMessageWithSource(message: ContextMessage, source: ContextFileSource): ContextMessage {
    if (message.file) {
        message.file.source = source
    }
    return message
}
