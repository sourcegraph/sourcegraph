<script lang="ts">
    import { queryStateStore } from '$lib/search/state'
    import { settings } from '$lib/stores'

    import type { PageData, Snapshot } from './$types'
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

{#if data.searchStream}
    <SearchResults
        bind:this={searchResults}
        stream={data.searchStream}
        queryFromURL={data.queryFromURL}
        {queryState}
        selectedFilters={data.queryFilters}
    />
{:else}
    <SearchHome {queryState} />
{/if}
