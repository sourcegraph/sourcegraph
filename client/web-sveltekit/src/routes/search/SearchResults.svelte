<svelte:options immutable />

<script context="module" lang="ts">
    export interface SearchResultsCapture {
        // The search results scroll offset
        scroll: number
        // The currently focused search result (if any)
        focused: number | undefined
    }
    interface ResultStateCache {
        count: number
        expanded: Set<SearchMatch>
        preview: ContentMatch | SymbolMatch | PathMatch | null
    }
    const cache = new Map<string, ResultStateCache>()

    const DEFAULT_INITIAL_ITEMS_TO_SHOW = 15
    const INCREMENTAL_ITEMS_TO_SHOW = 10
</script>

<script lang="ts">
    import type { Observable } from 'rxjs'
    import { afterUpdate, onMount, tick } from 'svelte'
    import { writable } from 'svelte/store'

    import { beforeNavigate, goto } from '$app/navigation'
    import { limitHit } from '$lib/branded'
    import Icon from '$lib/Icon.svelte'
    import { observeIntersection } from '$lib/intersection-observer'
    import GlobalHeaderPortal from '$lib/navigation/GlobalHeaderPortal.svelte'
    import type { URLQueryFilter } from '$lib/search/dynamicFilters'
    import DynamicFiltersSidebar from '$lib/search/dynamicFilters/Sidebar.svelte'
    import { createRecentSearchesStore } from '$lib/search/input/recentSearches'
    import SearchInput, { Style } from '$lib/search/input/SearchInput.svelte'
    import { getQueryURL, type QueryStateStore } from '$lib/search/state'
    import {
        TELEMETRY_SEARCH_SOURCE_TYPE,
        type AggregateStreamingSearchResults,
        type PathMatch,
        type SearchMatch,
        type SymbolMatch,
        type ContentMatch,
    } from '$lib/shared'
    import { isViewportMobile } from '$lib/stores'
    import { TELEMETRY_RECORDER } from '$lib/telemetry'
    import Button from '$lib/wildcard/Button.svelte'
    import Panel from '$lib/wildcard/resizable-panel/Panel.svelte'
    import PanelGroup from '$lib/wildcard/resizable-panel/PanelGroup.svelte'
    import PanelResizeHandle from '$lib/wildcard/resizable-panel/PanelResizeHandle.svelte'

    import PreviewPanel from './PreviewPanel.svelte'
    import SearchAlert from './SearchAlert.svelte'
    import type { SearchJob } from './searchJob'
    import { getSearchResultComponent } from './searchResultFactory'
    import { setSearchResultsContext } from './searchResultsContext'
    import { focusedResultIndex, nextResult, nthFocusableResult } from './searchResultsFocus'
    import StreamingProgress from './StreamingProgress.svelte'

    export let stream: Observable<AggregateStreamingSearchResults>
    export let queryFromURL: string
    export let selectedFilters: URLQueryFilter[]
    export let queryState: QueryStateStore
    export let searchJob: SearchJob | undefined = undefined

    export function capture(): SearchResultsCapture {
        return {
            scroll: $resultContainer?.scrollTop ?? 0,
            focused: $resultContainer ? focusedResultIndex($resultContainer) : undefined,
        }
    }

    export function restore(capture?: SearchResultsCapture): void {
        if ($resultContainer && capture) {
            $resultContainer.scrollTop = capture.scroll
            if (capture.focused) {
                nthFocusableResult($resultContainer, capture.focused)?.focus({ preventScroll: true })
            }
        }
    }

    export function focusNextResult(direction: 'up' | 'down'): boolean {
        if ($resultContainer) {
            const nextFocus = nextResult($resultContainer, direction)
            if (!nextFocus) {
                return false
            }
            nextFocus.focus({ preventScroll: true })
            nextFocus.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
            return true
        }
        return false
    }

    const resultContainer = writable<HTMLElement | null>(null)
    let searchResultsFiltersPanel: Panel
    const recentSearches = createRecentSearchesStore()

    $: state = $stream.state // 'loading', 'error', 'complete'
    $: results = $stream.results
    $: if (state !== 'loading') {
        recentSearches.addRecentSearch({
            query: queryFromURL,
            limitHit: limitHit($stream.progress),
            resultCount: $stream.progress.matchCount,
        })
    }

    let haveSetFocus = false // gets reset on query resubmission or filter changes
    afterUpdate(() => {
        if (!$isViewportMobile && !haveSetFocus && results.length > 0) {
            const firstFocusableResult = $resultContainer?.querySelector<HTMLElement>('[data-focusable-search-result]')
            if (firstFocusableResult) {
                firstFocusableResult.focus()
                haveSetFocus = true
            }
        }
    })
    $: selectedFilters, (haveSetFocus = false) // reset focus on filter change

    // Logic for maintaining list state (scroll position, rendered items, open
    // items) for backwards navigation.
    $: cacheEntry = cache.get(queryFromURL)
    $: count = cacheEntry?.count ?? DEFAULT_INITIAL_ITEMS_TO_SHOW
    $: resultsToShow = results.slice(0, count)
    $: expandedSet = cacheEntry?.expanded || new Set<SearchMatch>()

    $: previewResult = writable(cacheEntry?.preview ?? null)

    setSearchResultsContext({
        isExpanded(match: SearchMatch): boolean {
            return expandedSet.has(match)
        },
        setExpanded(match: SearchMatch, expanded: boolean): void {
            if (expanded) {
                expandedSet.add(match)
            } else {
                expandedSet.delete(match)
            }
        },
        setPreview(result: ContentMatch | SymbolMatch | PathMatch | null): void {
            previewResult.set(result)
        },
        queryState,
        scrollContainer: resultContainer,
    })

    beforeNavigate(() => {
        cache.set(queryFromURL, { count, expanded: expandedSet, preview: $previewResult })
    })

    onMount(() => {
        TELEMETRY_RECORDER.recordEvent('search.results', 'view')
    })

    function loadMore(event: { detail: boolean }) {
        if (event.detail) {
            count += INCREMENTAL_ITEMS_TO_SHOW
        }
    }

    // FIXME: Not a great solution since it relies on implementation details of
    // the progress component
    async function onResubmitQuery(event: SubmitEvent) {
        const target = event.currentTarget as HTMLElement | null
        const filters = Array.from(target?.querySelectorAll('[name="query"]') ?? [])
            .filter(input => (input as HTMLInputElement).checked)
            .map(input => (input as HTMLInputElement).value)
            .join(' ')
        queryState.setQuery(query => query + ' ' + filters)
        await tick()
        void goto(getQueryURL($queryState))
    }

    function handleResultCopy(): void {
        TELEMETRY_RECORDER.recordEvent('search.result.code', 'copy')
    }

    function handleSearchResultClick(index: number): void {
        TELEMETRY_RECORDER.recordEvent('search.result.area', 'click', {
            metadata: {
                index,
                resultsLength: results.length,
            },
        })
    }

    function handleSubmit() {
        haveSetFocus = false // reset focus when a new query is submitted
        TELEMETRY_RECORDER.recordEvent('search', 'submit', {
            metadata: { source: TELEMETRY_SEARCH_SOURCE_TYPE['nav'] },
        })
    }
</script>

{#if $isViewportMobile}
    <div class="search-header">
        <SearchInput {queryState} style={Style.Compact | Style.NoBorder} onSubmit={handleSubmit} />
    </div>
{:else}
    <GlobalHeaderPortal>
        <div class="search-header">
            <SearchInput {queryState} style={Style.Compact} onSubmit={handleSubmit} />
        </div>
    </GlobalHeaderPortal>
{/if}

<div class="search-results">
    <PanelGroup id="search-results-panels">
        <Panel
            bind:this={searchResultsFiltersPanel}
            id="search-results-filters"
            order={1}
            defaultSize={25}
            maxSize={35}
            minSize={15}
            overlayOnMobile
        >
            <DynamicFiltersSidebar
                {selectedFilters}
                streamFilters={$stream.filters}
                searchQuery={queryFromURL}
                {state}
            />
        </Panel>
        <PanelResizeHandle />
        <Panel id="search-results-content" order={2} minSize={35}>
            <div class="results">
                <aside class="actions">
                    {#if $isViewportMobile}
                        <Button
                            variant="secondary"
                            display="block"
                            aria-label="Open filters"
                            on:click={() => searchResultsFiltersPanel.expand()}
                            data-scope-button>Filters</Button
                        >
                    {/if}
                    <StreamingProgress {state} progress={$stream.progress} on:submit={onResubmitQuery} {searchJob} />
                </aside>
                <div class="result-list" bind:this={$resultContainer}>
                    {#if $stream.alert}
                        <div class="message-container">
                            <SearchAlert alert={$stream.alert} />
                        </div>
                    {/if}
                    <!--
                        TODO: Address accessibility issues
                        1. A11y: visible, non-interactive elements with an on:click event
                           must be accompanied by an on:keydown, on:keyup, or on:keypress event.
                        2. A11y: Non-interactive element <ol> should not be assigned mouse
                           or keyboard event listeners.
                    -->
                    <ol on:copy={handleResultCopy}>
                        {#each resultsToShow as result, i}
                            {@const component = getSearchResultComponent(result)}
                            {#if i === resultsToShow.length - 1}
                                <li
                                    use:observeIntersection={$resultContainer}
                                    on:intersecting={loadMore}
                                    on:click={() => handleSearchResultClick(i)}
                                >
                                    <svelte:component this={component} {result} />
                                </li>
                            {:else}
                                <li on:click={() => handleSearchResultClick(i)}>
                                    <svelte:component this={component} {result} />
                                </li>
                            {/if}
                        {/each}
                    </ol>
                    {#if resultsToShow.length === 0 && state !== 'loading'}
                        <div class="message-container">
                            <Icon icon={ILucideOctagonX} aria-hidden="true" />
                            <p>No results found</p>
                        </div>
                    {/if}
                </div>
            </div>
        </Panel>

        {#if $previewResult}
            <PanelResizeHandle />
            <Panel id="search-results-file-preview" order={3} minSize={30}>
                <PreviewPanel result={$previewResult} />
            </Panel>
        {/if}
    </PanelGroup>
</div>

<style lang="scss">
    .search-header {
        width: 100%;
        // This ensures that the search suggestions panel is displayed above the
        // search results panel.
        z-index: 1;

        @media (--mobile) {
            border-bottom: 1px solid var(--border-color);
        }
    }

    .search-results {
        display: flex;
        flex: 1;
        overflow: auto;

        @media (--not-mobile) {
            // Isolate everything in search results so they won't be displayed over
            // the search suggestions. Previously, hovering over separator would
            // overlap the suggestions panel.
            // Do not do this on small screen devices because the sidebar is positioned
            // over the content instead, but the search input would cover the sidebar.
            isolation: isolate;
        }

        :global(#search-results-filters) {
            @media (--mobile) {
                // Needed to position sidebar above the search results content.
                z-index: 1;
            }
        }
    }

    .results {
        flex: 1;
        height: 100%;
        overflow: auto;
        min-height: 0;
        display: flex;
        flex-direction: column;

        .actions {
            border-bottom: 1px solid var(--border-color);
            padding: 0.5rem;

            @media (--mobile) {
                display: grid;
                gap: 0.5rem;
                grid-template-columns: 1fr auto;
            }

            :global([data-scope-button]) {
                align-self: stretch;
            }
        }

        .result-list {
            overflow: auto;

            ol {
                padding: 0;
                margin: 0;
                list-style: none;
            }

            :global([data-focusable-search-result]) {
                // Set a scroll margin on the focused search results
                // so that it doesn't underlay the sticky headers and
                // so that there is a little bit of space between the
                // result and the scroll box.
                scroll-margin: 4rem;
            }
        }

        .message-container {
            display: flex;
            flex-direction: column;
            align-items: center;
            margin: auto;
            color: var(--text-muted);
            margin: 2rem;
        }
    }
</style>
