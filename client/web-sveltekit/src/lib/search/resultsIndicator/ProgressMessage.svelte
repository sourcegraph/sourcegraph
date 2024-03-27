<script lang="ts">
    import InfoBadge from '$lib/search/resultsIndicator/InfoBadge.svelte'
    import type { Progress } from '$lib/shared'

    export let state: 'error' | 'loading' | 'complete'
    export let progress: Progress
    export let elapsedDuration: number
    export let severity: string

    $: isError = state === 'error' || severity === 'error'
    $: loading = state === 'loading'
</script>

{#if loading}
    <div class="progress-message">
        Fetching results... {(elapsedDuration / 1000).toFixed(1)}s
    </div>
{:else}
    <InfoBadge {progress} {isError} />
{/if}

<style lang="scss">
    .progress-message {
        font-size: var(--font-size-base);
    }
</style>
