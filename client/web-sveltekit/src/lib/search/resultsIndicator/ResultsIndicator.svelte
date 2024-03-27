<script lang="ts">
    import { mdiChevronDown, mdiInformationOutline, mdiAlert, mdiAlertCircle } from '@mdi/js'
    import { onMount } from 'svelte'

    import Icon from '$lib/Icon.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import ProgressMessage from '$lib/search/resultsIndicator/ProgressMessage.svelte'
    import SuggestedAction from '$lib/search/resultsIndicator/SuggestedAction.svelte'
    import TimeoutMessage from '$lib/search/resultsIndicator/TimeoutMessage.svelte'
    import type { Progress, Skipped } from '$lib/shared'

    export let state: 'error' | 'complete' | 'loading'
    export let progress: Progress
    export let suggestedItems: Required<Skipped>[]
    export let severity: string

    const SEARCH_JOB_THRESHOLD = 8000
    const icons: Record<string, string> = {
        info: mdiInformationOutline,
        warning: mdiAlert,
        error: mdiAlertCircle,
    }

    onMount(() => {
        let startTime = Date.now()
        const interval = setInterval(() => {
            const now = Date.now()
            elapsedDuration = now - startTime
            // once search has completed, reset the startTime
            if (done) {
                startTime = Date.now()
            }
        }, 1300)
        return () => clearInterval(interval)
    })

    $: elapsedDuration = 0
    $: takingTooLong = elapsedDuration >= SEARCH_JOB_THRESHOLD
    $: loading = state === 'loading'
    $: severity = progress.skipped.some(skipped => skipped.severity === 'warn' || skipped.severity === 'error')
        ? 'error'
        : 'info'
    /*
     * NOTE: progress.done and state === 'complete' will sometimes be different values.
     * Only one of them needs to evaluate to true in order for the ResultIndicator to
     * evaluate a search as being finished. Hence, we check both here with an OR relationship
     */
    $: done = progress.done || state === 'complete'
</script>

<div class="indicator">
    <div class="icon">
        {#if loading}
            <LoadingSpinner inline />
        {:else}
            <Icon svgPath={icons[severity]} size={18} />
        {/if}
    </div>

    <div class="messages">
        <ProgressMessage {state} {progress} {elapsedDuration} {severity} />

        <div class="action-container">
            {#if !done && takingTooLong}
                <TimeoutMessage />
            {:else if done}
                <SuggestedAction {progress} {suggestedItems} {severity} {state} />
            {:else}
                <div class="suggested-action">
                    {#if elapsedDuration <= SEARCH_JOB_THRESHOLD}
                        <div class="running-search">
                            <small> Running Search </small>
                        </div>
                    {/if}
                </div>
            {/if}
        </div>
    </div>

    <div class="dropdown-icon">
        <Icon svgPath={mdiChevronDown} size={18} />
    </div>
</div>

<style lang="scss">
    .action-container {
        margin-top: 0.3rem;
    }

    .icon {
        margin-right: 0.5rem;
    }

    .dropdown-icon {
        margin-left: 1.2rem;
    }

    .indicator {
        align-items: center;
        display: flex;
        flex-flow: row nowrap;
        justify-content: space-between;
    }

    .messages {
        align-content: center;
        align-items: flex-start;
        display: flex;
        flex-flow: column nowrap;
    }

    .running-search {
        color: var(--text-muted);
    }

    .suggested-action {
        display: flex;
        flex-flow: row nowrap;
    }
</style>
