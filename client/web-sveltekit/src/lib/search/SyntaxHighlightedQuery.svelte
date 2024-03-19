<script lang="ts">
    import { decorateQuery } from '$lib/branded'
    import EmphasizedLabel from './EmphasizedLabel.svelte'

    export let query: string
    export let matches: Set<number> | null = null

    $: decorations = decorateQuery(query)
</script>

<span class="text-monospace search-query-link">
    {#if decorations}
        {#each decorations as { key, className, value, token } (key)}
            <span class={className}>
                {#if matches}
                    <EmphasizedLabel label={value} {matches} offset={token.range.start} />{:else}
                    {value}{/if}</span
            >
        {/each}
    {:else}
        {query}
    {/if}
</span>
