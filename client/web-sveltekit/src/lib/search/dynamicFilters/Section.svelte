<script lang="ts">
    import { mdiClose } from '@mdi/js'

    import { page } from '$app/stores'
    import Icon from '$lib/Icon.svelte'
    import { Button } from '$lib/wildcard'

    import CountBadge from './CountBadge.svelte'
    import { updateFilterInURL, type SectionItem } from './index'

    export let items: SectionItem[]
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
                    <a
                        href={updateFilterInURL($page.url, item, item.selected).toString()}
                        class:selected={item.selected}
                    >
                        <span class="label">
                            <slot name="label" label={item.label} value={item.value}>
                                {item.label}
                            </slot>
                        </span>
                        <CountBadge count={item.count} exhaustive={item.exhaustive} />
                        {#if item.selected}
                            <span class="close">
                                <Icon svgPath={mdiClose} inline />
                            </span>
                        {/if}
                    </a>
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
        gap: 0.5rem;
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

    a {
        display: flex;
        width: 100%;
        align-items: center;
        border: none;
        text-align: left;
        text-decoration: none;
        border-radius: var(--border-radius);
        color: inherit;
        white-space: nowrap;
        gap: 0.25rem;

        padding: 0.25rem 0.25rem 0.25rem 0.5rem;
        margin: 0;
        font-weight: 400;

        .label {
            flex: 1;
            text-overflow: ellipsis;
            overflow: hidden;
        }

        &:hover {
            background-color: var(--secondary-2);
        }

        &.selected {
            background-color: var(--primary);
            color: var(--primary-4);
            --color: var(--primary-4);
        }

        .close {
            flex-shrink: 0;
        }
    }
</style>
