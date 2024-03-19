<svelte:options immutable />

<script lang="ts">
    import { observeIntersection } from '$lib/intersection-observer'
    import { fetchFileRangeMatches } from '$lib/search/api/highlighting'
    import CodeExcerpt from '$lib/search/CodeExcerpt.svelte'
    import CodeHostIcon from '$lib/search/CodeHostIcon.svelte'
    import SymbolKind from '$lib/search/SymbolKind.svelte'
    import type { SymbolMatch } from '$lib/shared'

    import FileSearchResultHeader from './FileSearchResultHeader.svelte'
    import PreviewButton from './PreviewButton.svelte'
    import RepoStars from './RepoStars.svelte'
    import SearchResult from './SearchResult.svelte'

    export let result: SymbolMatch

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
    <CodeHostIcon slot="icon" repository={result.repository} />
    <FileSearchResultHeader slot="title" {result} />
    <svelte:fragment slot="info">
        {#if result.repoStars}
            <RepoStars repoStars={result.repoStars} />
        {/if}
        <PreviewButton {result} />
    </svelte:fragment>
    <svelte:fragment slot="body">
        <div use:observeIntersection on:intersecting={event => (visible = event.detail)}>
            {#each result.symbols as symbol, index}
                <a href={symbol.url}>
                    <div class="result">
                        <div class="symbol-kind">
                            <SymbolKind symbolKind={symbol.kind} />
                        </div>
                        {#await highlightedHTMLRows then result}
                            <CodeExcerpt
                                startLine={symbol.line - 1}
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
        border-bottom: 1px solid var(--border-color);

        background-color: var(--code-bg);
        &:hover {
            background-color: var(--subtle-bg-2);
        }
    }

    .symbol-kind {
        margin-right: 0.5rem;
    }

    a:hover {
        text-decoration: none;
    }
</style>
