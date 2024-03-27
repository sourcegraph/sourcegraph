// @vitest-environment jsdom

import { render } from '@testing-library/svelte'
import type { ComponentProps } from 'svelte'
import { describe, test, expect } from 'vitest'

import ProgressMessage from '$lib/search/resultsIndicator/ProgressMessage.svelte'

describe('ProgressMessage.svelte', () => {
    function renderProgressMessage(options?: Partial<ComponentProps<ProgressMessage>>): void {
        render(ProgressMessage, { ...options })
    }

    test('render general message if in loading state', async () => {
        renderProgressMessage({
            progress: {
                matchCount: 0,
                durationMs: 0,
                skipped: [],
            },
            elapsedDuration: 5000,
            state: 'loading',
            severity: 'info',
        })

        const progressMessage = document.getElementsByClassName('progress-message')
        expect(progressMessage[0].textContent).toBe(`Fetching results... ${(5000 / 1000).toFixed(1)}s`)
        expect(progressMessage).toHaveLength(1)
    })
})
