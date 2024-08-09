<svelte:options immutable />

<script lang="ts" context="module">
    const BY_LINE_RANKING = 'by-line-number'
    const DEFAULT_CONTEXT_LINES = 1
    const DEFAULT_EXPANDED_MATCHES = 5
</script>

<script lang="ts">
    import CodeExcerpt from '$lib/CodeExcerpt.svelte'
    import { pluralize, SourcegraphURL } from '$lib/common'
    import Icon from '$lib/Icon.svelte'
    import { observeIntersection } from '$lib/intersection-observer'
    import RepoStars from '$lib/repo/RepoStars.svelte'
    import { fetchFileRangeMatches } from '$lib/search/api/highlighting'
    import { rankContentMatch } from '$lib/search/results'
    import { getFileMatchUrl, type ContentMatch, rankByLine, rankPassthrough } from '$lib/shared'
    import { settings } from '$lib/stores'

    import FileSearchResultHeader from './FileSearchResultHeader.svelte'
    import PreviewButton from './PreviewButton.svelte'
    import SearchResult from './SearchResult.svelte'
    import { getSearchResultsContext } from './searchResultsContext'

    export let result: ContentMatch

    $: contextLines = $settings?.['search.contextLines'] ?? DEFAULT_CONTEXT_LINES
    $: ranking =
        $settings?.experimentalFeatures?.clientSearchResultRanking === BY_LINE_RANKING ? rankByLine : rankPassthrough
    $: ({ expandedMatchGroups, collapsedMatchGroups, hiddenMatchesCount } = rankContentMatch(
        result,
        ranking,
        DEFAULT_EXPANDED_MATCHES,
        contextLines
    ))
    $: collapsible = hiddenMatchesCount > 0
    $: fileURL = getFileMatchUrl(result)

    const searchResultContext = getSearchResultsContext()
    const scrollContainer = searchResultContext.scrollContainer
    let expanded: boolean = searchResultContext?.isExpanded(result)
    $: searchResultContext.setExpanded(result, expanded)
    $: expandButtonText = expanded
        ? 'Show less'
        : `Show ${hiddenMatchesCount} more ${pluralize('match', hiddenMatchesCount, 'matches')}`
    $: matchesToShow = expanded ? expandedMatchGroups : collapsedMatchGroups

    let root: HTMLElement
    let userInteracted = false
    $: if (!expanded && root && userInteracted) {
        setTimeout(() => {
            const reducedMotion = !window.matchMedia('(prefers-reduced-motion: no-preference)').matches
            root.scrollIntoView({ block: 'nearest', behavior: reducedMotion ? 'auto' : 'smooth' })
        }, 0)
    }

    function getMatchURL(line: number, endLine: number): string {
        return SourcegraphURL.from(fileURL).setLineRange({ line, endLine }).toString()
    }

    let visible = false
    let highlightedHTMLRows: Promise<string[][]> | undefined
    $: if (visible) {
        // If the file contains some large lines, avoid stressing syntax-highlighter and the browser.
        if (!result.chunkMatches?.some(chunk => chunk.contentTruncated)) {
            // We rely on fetchFileRangeMatches to cache the result for us so that repeated
            // calls will not result in repeated network requests.
            highlightedHTMLRows = fetchFileRangeMatches({
                result,
                ranges: expandedMatchGroups.map(group => ({
                    startLine: group.startLine,
                    endLine: group.endLine,
                })),
            })
        }
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

    <div
        bind:this={root}
        use:observeIntersection={$scrollContainer}
        on:intersecting={event => (visible = event.detail)}
        class="matches"
    >
        {#each matchesToShow as group, index}
            <div class="code">
                <a href={getMatchURL(group.startLine + 1, group.endLine)} data-focusable-search-result>
                    <!--
                        We need to "post-slice" `highlightedHTMLRows` because we fetch highlighting for
                        the whole chunk.
                    -->
                    {#await highlightedHTMLRows}
                        <CodeExcerpt
                            startLine={group.startLine}
                            matches={group.matches}
                            plaintextLines={group.plaintextLines}
                            --background-color="transparent"
                        />
                    {:then result}
                        <CodeExcerpt
                            startLine={group.startLine}
                            matches={group.matches}
                            plaintextLines={group.plaintextLines}
                            highlightedHTMLRows={result?.[index]?.slice(0, group.plaintextLines.length)}
                            --background-color="transparent"
                        />
                    {/await}
                </a>
            </div>
        {/each}
        {#if collapsible}
            <button
                type="button"
                on:click={() => {
                    expanded = !expanded
                    userInteracted = true
                }}
                class:expanded
                data-focusable-search-result
            >
                <Icon icon={expanded ? ILucideChevronUp : ILucideChevronDown} inline aria-hidden="true" />
                <span>{expandButtonText}</span>
            </button>
        {/if}
    </div>
</SearchResult>

<style lang="scss">
    button {
        width: 100%;
        text-align: left;
        border: none;
        padding: 0.25rem 0.5rem;
        background-color: var(--code-bg);
        color: var(--text-muted);
        cursor: pointer;

        &.expanded {
            position: sticky;
            bottom: 0;
        }

        &:hover {
            background-color: var(--secondary-4);
            color: var(--text-title);
        }
    }

    .code {
        border-bottom: 1px solid var(--border-color);
        background-color: var(--code-bg);

        &:last-child {
            border-bottom: none;
        }

        &:hover {
            background-color: var(--secondary-4);
        }

        a {
            text-decoration: none;
            color: inherit;
            display: block;
            padding: 0.125rem 0.375rem;
        }
    }

    [data-focusable-search-result]:focus {
        box-shadow: var(--focus-shadow-inset);
    }
</style>
