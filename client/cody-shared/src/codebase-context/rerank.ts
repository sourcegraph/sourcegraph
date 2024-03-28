import x2js from 'x2js'

import type { ChatClient } from '../chat/chat'
import type { ContextResult } from '../local-context'

export interface Reranker {
    rerank(userQuery: string, results: ContextResult[]): Promise<ContextResult[]>
}

export class MockReranker implements Reranker {
    constructor(private rerank_: (userQuery: string, results: ContextResult[]) => Promise<ContextResult[]>) {}

    public rerank(userQuery: string, results: ContextResult[]): Promise<ContextResult[]> {
        return this.rerank_(userQuery, results)
    }
}

/**
 * A reranker class that uses a LLM to boost high-relevance results.
 */
export class LLMReranker implements Reranker {
    constructor(private chatClient: ChatClient) {}

    public async rerank(userQuery: string, results: ContextResult[]): Promise<ContextResult[]> {
        // Reverse the results so the most important appears first
        results = [...results].reverse()

        let out = await new Promise<string>((resolve, reject) => {
            let responseText = ''
            this.chatClient.chat(
                [
                    {
                        speaker: 'human',
                        text: `I am a professional computer programmer and need help deciding which of these files to read first to answer my question. My question is <userQuestion>${userQuery}</userQuestion>. Select the files from the following list that I should read to answer my question, ranked by most relevant first. Format the result as XML, like this: <list><item><filename>filename 1</filename><explanation>this is why I chose this item</explanation></item><item><filename>filename 2</filename><explanation>why I chose this item</explanation></item></list>\n${results
                            .map(r => r.fileName)
                            .join('\n')}`,
                    },
                ],
                {
                    onChange: (text: string) => {
                        responseText = text
                    },
                    onComplete: () => {
                        resolve(responseText)
                    },
                    onError: (message: string, statusCode?: number) => {
                        reject(new Error(`Status code ${statusCode}: ${message}`))
                    },
                },
                {
                    temperature: 0,
                    fast: true,
                }
            )
        })
        if (out.indexOf('<list>') > 0) {
            out = out.slice(out.indexOf('<list>'))
        }
        if (out.indexOf('</list>') !== out.length - '</list>'.length) {
            out = out.slice(0, out.indexOf('</list>') + '</list>'.length)
        }
        const boostedFilenames = parseFileExplanations(out)

        const resultsMap = Object.fromEntries(results.map(r => [r.fileName, r]))
        const boostedNames = new Set<string>()
        const rerankedResults = []
        for (const boostedFilename of boostedFilenames) {
            const boostedResult = resultsMap[boostedFilename]
            if (!boostedResult) {
                continue
            }
            rerankedResults.push(boostedResult)
            boostedNames.add(boostedFilename)
        }
        for (const result of results) {
            if (!boostedNames.has(result.fileName)) {
                rerankedResults.push(result)
            }
        }

        rerankedResults.reverse()
        return rerankedResults
    }
}

export function parseFileExplanations(xml: string): string[] {
    try {
        const result = new x2js({}).xml2js<{ list: { item: [{ filename: string; explanation: string }] } }>(xml)
        const items = result.list.item
        const files: { filename: string; explanation: string }[] = items.map(item => ({
            filename: item.filename,
            explanation: item.explanation,
        }))
        return files.map(f => f.filename)
    } catch {
        return []
    }
}
