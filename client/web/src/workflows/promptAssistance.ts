import { fetchEventSource } from '@microsoft/fetch-event-source'

import type { WorkflowFormValue } from './WorkflowForm'

export async function generateSuggestedPrompt(
    workflow: Pick<WorkflowFormValue, 'name' | 'description'>
): Promise<string> {
    const suggestion = await getCodyCompletionOneShot([
        {
            speaker: 'human',
            text: 'You are Cody, an AI-powered coding assistant.',
        },
        { speaker: 'assistant', text: 'Understood.' },
        {
            speaker: 'human',
            text: `Help me write a prompt for an AI code assistant, as though you are an expert prompt engineer. The name of my task is '${workflow.name}' and the description is '${workflow.description}'. ONLY respond with the prompt, no other explanations.`,
        },
        { speaker: 'assistant', text: 'Here is the prompt:\n\n' },
    ])
    return suggestion
}

interface CompletionRequest {
    messages: { speaker: 'human' | 'assistant'; text: string }[]
    temperature: number
    maxTokensToSample: number
    topK: number
    topP: number
}

const DEFAULT_CHAT_COMPLETION_PARAMETERS: Omit<CompletionRequest, 'messages'> = {
    temperature: 0.2,
    maxTokensToSample: 1000,
    topK: -1,
    topP: -1,
}

function getCodyCompletionOneShot(messages: CompletionRequest['messages']): Promise<string> {
    return new Promise<string>((resolve, reject) => {
        let lastCompletion: string | undefined
        fetchEventSource('/.api/completions/stream', {
            method: 'POST',
            headers: { 'X-Requested-With': 'Sourcegraph', 'Content-Type': 'application/json; charset=utf-8' },
            body: JSON.stringify({
                ...DEFAULT_CHAT_COMPLETION_PARAMETERS,
                messages,
            } satisfies CompletionRequest),
            onmessage(message) {
                if (message.event === 'completion') {
                    const data = JSON.parse(message.data) as { completion: string }
                    lastCompletion = data.completion
                }
            },
            onclose() {
                if (lastCompletion) {
                    resolve(lastCompletion)
                } else {
                    reject(new Error('no completion received'))
                }
            },
            onerror(error) {
                reject(error)
            },
        }).catch(error => reject(error))
    })
}
