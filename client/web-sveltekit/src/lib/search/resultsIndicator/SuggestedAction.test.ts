// @vitest-environment jsdom

import { render } from '@testing-library/svelte'
import type { ComponentProps } from 'svelte'
import { describe, test, expect } from 'vitest'

import SuggestedAction from '$lib/search/resultsIndicator/SuggestedAction.svelte'

describe('SuggestedAction.svelte', () => {
    function renderSuggestedAction(options?: Partial<ComponentProps<SuggestedAction>>): void {
        render(SuggestedAction, { ...options })
    }

    test('renders action with title and suggestion', async () => {
        renderSuggestedAction({
            progress: {
                done: true,
                matchCount: 600,
                durationMs: 5260,
                skipped: [
                    {
                        reason: 'error',
                        title: 'this is an error',
                        message: 'vv much an error',
                        severity: 'error',
                        suggested: {
                            title: "here's a title",
                            message: "here's a message",
                        },
                    },
                ],
            },
            severity: 'error',
            state: 'complete',
        })

        const title = document.getElementsByClassName('info-badge')
        expect(title).toHaveLength(1)

        const interpunct = document.getElementsByClassName('separator')
        expect(interpunct).toHaveLength(1)

        const action = document.getElementsByClassName('action-badge')
        expect(action).toHaveLength(1)
    })

    test('renders title only when there is no suggested items', async () => {
        renderSuggestedAction({
            progress: {
                done: true,
                matchCount: 600,
                durationMs: 5260,
                skipped: [
                    {
                        reason: 'error',
                        title: 'this is an error',
                        message: 'vv much an error',
                        severity: 'error',
                    },
                ],
            },
            severity: 'error',
            state: 'error',
        })

        const infoBadge = document.getElementsByClassName('info-badge error-text')
        expect(infoBadge).toHaveLength(1)

        const interpunct = document.getElementsByClassName('separator')
        expect(interpunct).toHaveLength(0)

        const action = document.getElementsByClassName('action-badge')
        expect(action).toHaveLength(0)
    })

    test('shows most severe skipped item in action container', async () => {
        renderSuggestedAction({
            progress: {
                matchCount: 600,
                durationMs: 5260,
                skipped: [
                    {
                        reason: 'display',
                        title: 'Display limit hit',
                        message: 'you hit the display limit',
                        severity: 'info',
                    },
                    {
                        reason: 'error is the reason',
                        title: 'Error badge',
                        message: 'error is the message',
                        severity: 'error',
                    },
                ],
                done: true,
            },
            severity: 'error',
            state: 'error',
        })

        const infoBadge = document.getElementsByClassName('info-badge error-text')
        expect(infoBadge).toHaveLength(1)
        expect(infoBadge[0].textContent).toBe('Error badge')
    })
})
