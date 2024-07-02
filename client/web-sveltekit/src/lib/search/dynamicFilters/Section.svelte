<script lang="ts">
    import type { ComponentProps } from 'svelte'

    import { Button } from '$lib/wildcard'

    import SectionItem from './SectionItem.svelte'

    export let items: ComponentProps<SectionItem>[]
    export let title: string
    export let filterPlaceholder: string = ''
    export let showAll: boolean = false

    let filterText = ''
    $: processedFilterText = filterText.trim().toLowerCase()
    $: filteredItems = processedFilterText
        ? items.filter(item => item.label.toLowerCase().includes(processedFilterText))
        : items
    let filterInputRef: HTMLInputElement

    let showMore = false
    $: showCount = showAll ? items.length : showMore ? 10 : 5
    $: limitedItems = filteredItems.slice(0, showCount)
</script>

{#if items.length > 0}
    <article>
        <header><h4>{title}</h4></header>
        {#if items.length > showCount}
            <input bind:this={filterInputRef} bind:value={filterText} placeholder={filterPlaceholder} />
        {/if}
        <ul>
            {#each limitedItems as item}
                <li>
                    <slot name="item" {item}>
                        <SectionItem {...item} on:select />
                    </slot>
                </li>
            {/each}
        </ul>
        {#if filteredItems.length === 0}
            <small class="filter-message">
                <div class="header"><strong>No matches in search results.</strong></div>
                Try expanding your search using the
                <Button variant="link" display="inline" on:click={() => filterInputRef.focus()}>search bar</Button>
                above.
            </small>
        {:else if showMore}
            {#if filteredItems.length > limitedItems.length}
                <small class="filter-message">
                    {filteredItems.length - limitedItems.length} not shown. Use
                    <Button variant="link" display="inline" on:click={() => filterInputRef.focus()}>search</Button>
                    to see more.
                </small>
            {/if}
            <footer class="show-more">
                <Button variant="link" on:click={() => (showMore = false)}>Show less</Button>
            </footer>
        {:else if filteredItems.length > limitedItems.length}
            <footer class="show-more">
                <Button variant="link" on:click={() => (showMore = true)}>Show more</Button>
            </footer>
        {/if}
    </article>
{/if}

<style lang="scss">
    article {
        padding: 0 1rem;
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    input {
        display: block;
        width: 100%;
        height: var(--input-height);
        padding: var(--input-padding-y) var(--input-padding-x);
        font-size: var(--input-font-size);
        font-weight: var(--input-font-weight);
        line-height: var(--input-line-height);
        color: var(--input-color);
        background-color: var(--input-bg);
        background-clip: padding-box;
        border: var(--input-border-width) solid var(--input-border-color);
        border-radius: var(--border-radius);
    }

    ul {
        display: flex;
        flex-flow: column nowrap;
        gap: 0.125rem;
        margin: 0;
        padding: 0;
        list-style: none;
    }

    .filter-message {
        background-color: var(--secondary-2);
        color: var(--text-muted);
        padding: 0.75rem 1rem;
        border-radius: 0.5rem;

        .header {
            margin-bottom: 0.25rem;
        }
        :global(button) {
            padding: 0;
            text-align: inherit;
            line-height: inherit;
            font-size: inherit;
            font-style: inherit;
            vertical-align: inherit;
        }
    }

    .show-more {
        text-align: center;
    }
</style>
