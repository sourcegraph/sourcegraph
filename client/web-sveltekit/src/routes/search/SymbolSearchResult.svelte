<svelte:options immutable />

<script lang="ts">
    import CodeExcerpt from '$lib/CodeExcerpt.svelte'
    import { observeIntersection } from '$lib/intersection-observer'
    import RepoStars from '$lib/repo/RepoStars.svelte'
    import { fetchFileRangeMatches } from '$lib/search/api/highlighting'
    import SymbolKindIcon from '$lib/search/SymbolKindIcon.svelte'
    import type { SymbolMatch } from '$lib/shared'

    import FileSearchResultHeader from './FileSearchResultHeader.svelte'
    import PreviewButton from './PreviewButton.svelte'
    import SearchResult from './SearchResult.svelte'
    import { getSearchResultsContext } from './searchResultsContext'

    export let result: SymbolMatch

    const scrollContainer = getSearchResultsContext().scrollContainer

    $: ranges = result.symbols.map(symbol => ({
        startLine: symbol.line - 1,
        endLine: symbol.line,
    }))

    let visible = false
    let highlightedHTMLRows: Promise<string[][]> | undefined
    $: if (visible) {
        // We rely on fetchFileRangeMatches to cache the result for us so that repeated
        // calls will not result in repeated network requests.
        highlightedHTMLRows = fetchFileRangeMatches({ result, ranges: ranges })
    }
</script>

<SearchResult>
    <FileSearchResultHeader slot="title" {result} />
    <svelte:fragment slot="info">
        {#if result.repoStars}
            <RepoStars repoStars={result.repoStars} />
        {/if}
        <PreviewButton {result} />
    </svelte:fragment>
    <svelte:fragment slot="body">
        <div use:observeIntersection={$scrollContainer} on:intersecting={event => (visible = event.detail)}>
            {#each result.symbols as symbol, index}
                <a href={symbol.url} data-focusable-search-result>
                    <div class="result">
                        <SymbolKindIcon symbolKind={symbol.kind} />
                        {#await highlightedHTMLRows then result}
                            <CodeExcerpt
                                startLine={symbol.line}
                                plaintextLines={['']}
                                highlightedHTMLRows={result?.[index]}
                                --background-color="transparent"
                            />
                        {/await}
                    </div>
                </a>
            {/each}
        </div>
    </svelte:fragment>
</SearchResult>

<style lang="scss">
    .result {
        display: flex;
        align-items: center;
        width: 100%;
        padding: 0.5rem;
        gap: 0.5rem;
        border-bottom: 1px solid var(--border-color);

        &:hover {
            background-color: var(--subtle-bg-2);
        }
    }

    a {
        display: block;
        box-sizing: border-box;
        &:hover {
            text-decoration: none;
        }
    }

    [data-focusable-search-result]:focus {
        box-shadow: var(--focus-shadow-inset);
    }
</style>
