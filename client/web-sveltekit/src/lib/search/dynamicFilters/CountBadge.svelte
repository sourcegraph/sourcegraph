<script context="module" lang="ts">
    const roundNumbers = [10000, 5000, 1000, 500, 100, 50, 10, 5, 1]
    function roundCount(count: number): number {
        for (const roundNumber of roundNumbers) {
            if (count >= roundNumber) {
                return roundNumber
            }
        }
        return 0
    }
</script>

<script lang="ts">
    import { pluralize } from '$lib/common'
    import Tooltip from '$lib/Tooltip.svelte'
    import { Badge } from '$lib/wildcard'

    export let count: number
    export let exhaustive: boolean
</script>

{#if exhaustive}
    <Badge variant="secondary">{count}</Badge>
{:else}
    <Tooltip placement="right" tooltip="At least {count} {pluralize('result', count)} match this filter.">
        <Badge variant="secondary">{roundCount(count)}+</Badge>
    </Tooltip>
{/if}
