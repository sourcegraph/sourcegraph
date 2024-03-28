import { ANSWER_TOKENS } from '../prompt/constants'
import type { Message } from '../sourcegraph-api'
import type { SourcegraphCompletionsClient } from '../sourcegraph-api/completions/client'
import type { SourcegraphGraphQLAPIClient } from '../sourcegraph-api/graphql'

import type { IntentClassificationOption, IntentDetector } from '.'

const editorRegexps = [/editor/, /(open|current|this)\s+file/, /current(ly)?\s+open/, /have\s+open/]

export class SourcegraphIntentDetectorClient implements IntentDetector {
    constructor(
        private client: SourcegraphGraphQLAPIClient,
        private completionsClient?: SourcegraphCompletionsClient
    ) {}

    public isCodebaseContextRequired(input: string): Promise<boolean | Error> {
        return this.client.isContextRequiredForQuery(input)
    }

    public isEditorContextRequired(input: string): boolean | Error {
        const inputLowerCase = input.toLowerCase()
        // If the input matches any of the `editorRegexps` we assume that we have to include
        // the editor context (e.g., currently open file) to the overall message context.
        for (const regexp of editorRegexps) {
            if (inputLowerCase.match(regexp)) {
                return true
            }
        }
        return false
    }

    private buildInitialTranscript(options: IntentClassificationOption[]): Message[] {
        const functions = options
            .map(({ id, description }) => `Function ID: ${id}\nFunction Description: ${description}`)
            .join('\n')

        return [
            {
                speaker: 'human',
                text: prompt.replace('{functions}', functions),
            },
            {
                speaker: 'assistant',
                text: 'Ok.',
            },
        ]
    }

    private buildExampleTranscript(options: IntentClassificationOption[]): Message[] {
        const messages = options.flatMap(({ id, examplePrompts }) =>
            examplePrompts.flatMap(
                example =>
                    [
                        {
                            speaker: 'human',
                            text: example,
                        },
                        {
                            speaker: 'assistant',
                            text: `<classification>${id}</classification>`,
                        },
                    ] as const
            )
        )

        return messages
    }

    public async classifyIntentFromOptions<Intent extends string>(
        input: string,
        options: IntentClassificationOption<Intent>[],
        fallback: Intent
    ): Promise<Intent> {
        const matchingRawCommand = options.find(option => input.startsWith(option.rawCommand))
        if (matchingRawCommand) {
            // Matching command (e.g. /edit), so skip the LLM and return the intent.
            return matchingRawCommand.id
        }

        const completionsClient = this.completionsClient
        if (!completionsClient) {
            return fallback
        }

        const preamble = this.buildInitialTranscript(options)
        const examples = this.buildExampleTranscript(options)

        const result = await new Promise<string>(resolve => {
            let responseText = ''
            return completionsClient.stream(
                {
                    fast: true,
                    temperature: 0,
                    maxTokensToSample: ANSWER_TOKENS,
                    topK: -1,
                    topP: -1,
                    messages: [
                        ...preamble,
                        ...examples,
                        {
                            speaker: 'human',
                            text: input,
                        },
                        {
                            speaker: 'assistant',
                        },
                    ],
                },
                {
                    onChange: (text: string) => {
                        responseText = text
                    },
                    onComplete: () => {
                        resolve(responseText)
                    },
                    onError: (message: string, statusCode?: number) => {
                        // eslint-disable-next-line no-console
                        console.error(`Error detecting intent: Status code ${statusCode}: ${message}`)
                        resolve(fallback)
                    },
                }
            )
        })

        const responseClassification = result.match(/<classification>(.*?)<\/classification>/)?.[1]
        if (!responseClassification) {
            return fallback
        }

        return options.find(option => option.id === responseClassification)?.id ?? fallback
    }
}

const prompt = `
You are an AI assistant in a text editor. You are at expert at understanding the request of a software developer and selecting an available function to perform that request.
Think step-by-step to understand the request.
Only provide your response if you know the answer or can make a well-informed guess, otherwise respond with "unknown".
Enclose your response in <classification></classification> XML tags. Do not provide anything else.

Available functions:
{functions}
`
