<svelte:options immutable />

<script lang="ts">
    import { onMount } from 'svelte'

    import RepoStars from '$lib/repo/RepoStars.svelte'
    import type { PathMatch } from '$lib/shared'

    import FileSearchResultHeader from './FileSearchResultHeader.svelte'
    import PreviewButton from './PreviewButton.svelte'
    import SearchResult from './SearchResult.svelte'

    export let result: PathMatch

    let headerContainer: HTMLElement
    onMount(() => {
        const lastPathElement = headerContainer.querySelector<HTMLElement>('.last[data-path-item] > a')
        if (lastPathElement) {
            lastPathElement.dataset.focusableSearchResult = 'true'
        }
    })
</script>

<SearchResult>
    <div bind:this={headerContainer} class="header-container" slot="title">
        <FileSearchResultHeader {result} />
    </div>
    <svelte:fragment slot="info">
        {#if result.repoStars}
            <RepoStars repoStars={result.repoStars} />
        {/if}
        <PreviewButton {result} />
    </svelte:fragment>
</SearchResult>

<style lang="scss">
    .header-container {
        display: contents;
        :global([data-focusable-search-result]:focus) {
            box-shadow: var(--focus-shadow);
        }
    }
</style>
