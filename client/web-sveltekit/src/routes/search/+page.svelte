<script lang="ts">
    // @sg EnableRollout
    import { queryStateStore } from '$lib/search/state'
    import { settings } from '$lib/stores'

    import type { PageData, Snapshot } from './$types'
    import QueryExamples from './QueryExamples.svelte'
    import SearchHome from './SearchHome.svelte'
    import SearchResults, { type SearchResultsCapture } from './SearchResults.svelte'

    export let data: PageData

    export const snapshot: Snapshot<{ searchResults?: SearchResultsCapture }> = {
        capture() {
            return {
                searchResults: searchResults?.capture(),
            }
        },
        restore(value) {
            if (value) {
                searchResults?.restore(value.searchResults)
            }
        },
    }

    const queryState = queryStateStore(data.queryOptions ?? {}, $settings)
    let searchResults: SearchResults | undefined
    $: queryState.set(data.queryOptions ?? {})
    $: queryState.setSettings($settings)
</script>

<svelte:head>
    <title>{data.queryFromURL ? `${data.queryFromURL} - ` : ''}Sourcegraph</title>
</svelte:head>

{#if data.searchStream}
    <SearchResults
        bind:this={searchResults}
        stream={data.searchStream}
        queryFromURL={data.queryFromURL}
        {queryState}
        selectedFilters={data.queryFilters}
        searchJob={data.searchJob}
    />
{:else}
    <SearchHome {queryState}>
        <QueryExamples
            showQueryPage={data.showExampleQueries}
            queryExample={data.queryExample}
        />
        <svelte:component this={data.footer} />
    </SearchHome>
{/if}
