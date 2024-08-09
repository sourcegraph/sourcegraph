<script lang="ts">
    import type { SearchPatternType } from '@sourcegraph/web/src/graphql-operations'

    import { decorateQuery } from '$lib/branded'

    import EmphasizedLabel from './EmphasizedLabel.svelte'

    export let query: string
    export let patternType: SearchPatternType | undefined = undefined
    export let matches: Set<number> | null = null
    /**
     * If true the query will be wrapped between tokens as necessary
     */
    export let wrap: boolean = false

    $: decorations = decorateQuery(query, patternType)
</script>

<code class="search-query-link" class:wrap>
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
</code>

<style lang="scss">
    code {
        font-size: inherit;

        &.wrap {
            white-space: initial;
            line-height: initial;
        }
    }
</style>
