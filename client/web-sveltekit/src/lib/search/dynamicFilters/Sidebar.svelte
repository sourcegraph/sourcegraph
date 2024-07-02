<script context="module" lang="ts">
    function queryHasTypeFilter(query: string): boolean {
        const tokens = scanSearchQuery(query)
        if (tokens.type !== 'success') {
            return false
        }
        const filters = tokens.term.filter((token): token is QueryFilter => token.type === 'filter')
        return filters.some(filter => filter.field.value === 'type')
    }

    const sectionKinds = [
        'file',
        'repo',
        'lang',
        'utility',
        'author',
        'commit date',
        'symbol type',
        'type',
        'snippet',
    ] as const

    type SectionKind = typeof sectionKinds[number]

    // A statically-defined filter
    type StaticFilter = {
        label: string
        value: string
    }

    // A selected filter
    type SelectedFilter = StaticFilter

    // A filter sourced from the stream API
    type StreamFilter = StaticFilter & {
        count: number
        exhaustive: boolean
    }

    // Everything needed to render a SectionItem except the href, which
    // can be calculated from the current URL and the other props.
    type MergedFilter = Omit<ComponentProps<SectionItem>, 'href'>

    const typeFilterIcons: Record<string, IconComponent> = {
        Code: ILucideBraces,
        Repositories: ILucideGitFork,
        Paths: ILucideFile,
        Symbols: ILucideSquareFunction,
        Commits: ILucideGitCommitVertical,
        Diffs: ILucideDiff,
    }

    const staticTypeFilters: StaticFilter[] = [
        { label: 'Code', value: 'type:file' },
        { label: 'Repositories', value: 'type:repo' },
        { label: 'Paths', value: 'type:path' },
        { label: 'Symbols', value: 'type:symbol' },
        { label: 'Commits', value: 'type:commit' },
        { label: 'Diffs', value: 'type:diff' },
    ]

    // mergeFilterSources merges the filters of a shared kind from different sources.
    function mergeFilterSources(
        staticFilters: readonly StaticFilter[],
        selectedFilters: readonly SelectedFilter[],
        streamFilters: readonly StreamFilter[]
    ): MergedFilter[] {
        // Start with static filters, which are well-ordered
        const merged: MergedFilter[] = staticFilters.map(filter => ({
            ...filter,
            selected: false,
            count: undefined,
        }))

        // Then merge in the selected filters
        for (const selectedFilter of selectedFilters) {
            const found = merged.find(filter => filter.label === selectedFilter.label)
            if (found !== undefined) {
                // If we found a matching static filter, update it to be selected
                found.selected = true
            } else {
                // Othersie, add it to the end of the list
                merged.push({
                    ...selectedFilter,
                    selected: true,
                    count: undefined,
                })
            }
        }

        // Finally, merge in the filters from the search stream
        for (const streamFilter of streamFilters) {
            const found = merged.find(filter => filter.label === streamFilter.label)
            if (found !== undefined) {
                // If we found a matching filter, update its count
                found.count = { count: streamFilter.count, exhaustive: streamFilter.exhaustive }
            } else {
                // Otherwise, add it to the end of the list
                merged.push({
                    ...streamFilter,
                    count: { count: streamFilter.count, exhaustive: streamFilter.exhaustive },
                    selected: false,
                })
            }
        }

        return merged
    }
</script>

<script lang="ts">
    import { onMount, type ComponentProps } from 'svelte'

    import type { Filter as QueryFilter } from '@sourcegraph/shared/src/search/query/token'

    import { goto } from '$app/navigation'
    import { page } from '$app/stores'
    import { getGraphQLClient } from '$lib/graphql'
    import Icon, { type IconComponent } from '$lib/Icon.svelte'
    import KeyboardShortcut from '$lib/KeyboardShortcut.svelte'
    import LanguageIcon from '$lib/LanguageIcon.svelte'
    import Popover from '$lib/Popover.svelte'
    import RepoPopover, { fetchRepoPopoverData } from '$lib/repo/RepoPopover/RepoPopover.svelte'
    import CodeHostIcon from '$lib/search/CodeHostIcon.svelte'
    import SymbolKindIcon from '$lib/search/SymbolKindIcon.svelte'
    import { TELEMETRY_FILTER_TYPES, displayRepoName, scanSearchQuery, type Filter } from '$lib/shared'
    import { settings } from '$lib/stores'
    import { TELEMETRY_RECORDER } from '$lib/telemetry'
    import { delay } from '$lib/utils'
    import { Alert } from '$lib/wildcard'
    import Button from '$lib/wildcard/Button.svelte'

    import HelpFooter from './HelpFooter.svelte'
    import { type URLQueryFilter, moveFiltersToQuery, resetFilters, updateFilterInURL } from './index'
    import LoadingSkeleton from './LoadingSkeleton.svelte'
    import Section from './Section.svelte'
    import SectionItem from './SectionItem.svelte'

    export let searchQuery: string
    export let streamFilters: Filter[]
    export let selectedFilters: URLQueryFilter[]
    export let state: 'complete' | 'error' | 'loading'

    // We have three potential sources for filters:
    // - Static filters (types, snippets)
    // - Selected filters (stored in the URL)
    // - Stream filters (generated from search results)
    //
    // First, we group each source of filters by kind which is only relevant
    // for grouping and not for rendering individual items.

    let groupedStaticFilters: Partial<Record<SectionKind, StaticFilter[]>>
    $: groupedStaticFilters = {
        type: staticTypeFilters,
        snippet:
            $settings?.['search.scopes']?.map(
                (scope): StaticFilter => ({
                    label: scope.name,
                    value: scope.value,
                })
            ) ?? [],
    }

    let groupedSelectedFilters: Partial<Record<SectionKind, SelectedFilter[]>>
    $: groupedSelectedFilters = Object.groupBy(selectedFilters, ({ kind }) => kind)

    let groupedStreamFilters: Partial<Record<SectionKind, StreamFilter[]>>
    $: groupedStreamFilters = Object.groupBy(streamFilters, ({ kind }) => kind)

    // Then we merge the groups together. Different sources provide different
    // information (see mergeFilterSources for details). After we've merged, add
    // an href to the results. At that point, we have everything we need to render
    // a SectionItem.

    let sectionItems: Record<SectionKind, ComponentProps<SectionItem>[]>
    $: sectionItems = Object.fromEntries(
        sectionKinds.map(sectionKind => [
            sectionKind satisfies SectionKind,
            mergeFilterSources(
                groupedStaticFilters[sectionKind] ?? [],
                groupedSelectedFilters[sectionKind] ?? [],
                groupedStreamFilters[sectionKind] ?? []
            ).map(mergedFilter => ({
                ...mergedFilter,
                href: updateFilterInURL($page.url, { ...mergedFilter, kind: sectionKind }, mergedFilter.selected),
            })) satisfies ComponentProps<SectionItem>[],
        ])
    ) as Record<SectionKind, ComponentProps<SectionItem>[]> // Safe assertion because of internal `satisfies`

    $: resetURL = resetFilters($page.url).toString()
    $: enableReset = selectedFilters.length > 0

    function handleResetKeydown(event: KeyboardEvent) {
        if (enableReset && event.altKey && event.key === 'Backspace') {
            goto(resetURL)
        }
    }

    function handleFilterSelect(kind: keyof typeof TELEMETRY_FILTER_TYPES): void {
        TELEMETRY_RECORDER.recordEvent('search.filters', 'select', {
            metadata: { filterKind: TELEMETRY_FILTER_TYPES[kind] },
        })
    }

    // TODO: use registerHotkey
    onMount(() => {
        window.addEventListener('keydown', handleResetKeydown)
        return () => window.removeEventListener('keydown', handleResetKeydown)
    })
</script>

<aside class="sidebar">
    <div class="scroll-container">
        <div class="header">
            <h3>Filter results</h3>
            {#if enableReset}
                <div class="reset">
                    <a href={resetURL}>Reset all</a>&nbsp;
                    <KeyboardShortcut shortcut={{ key: 'alt+⌫' }} />
                </div>
            {/if}
        </div>

        {#if !queryHasTypeFilter(searchQuery)}
            <Section items={sectionItems.type} title="By type" showAll>
                <SectionItem slot="item" let:item {...item} on:select={() => handleFilterSelect('type')}>
                    <Icon slot="icon" icon={typeFilterIcons[item.label]} inline />
                </SectionItem>
            </Section>
        {/if}

        <Section items={sectionItems.repo} title="By repository" filterPlaceholder="Filter repositories">
            <svelte:fragment slot="item" let:item>
                <Popover showOnHover let:registerTrigger placement="right-start">
                    <div use:registerTrigger>
                        <SectionItem {...item} on:select={() => handleFilterSelect('repo')}>
                            <CodeHostIcon slot="icon" disableTooltip repository={item.label} />
                            <span slot="label">{displayRepoName(item.label)}</span>
                        </SectionItem>
                    </div>
                    <svelte:fragment slot="content">
                        {#await delay(fetchRepoPopoverData(getGraphQLClient(), item.label), 200) then data}
                            <RepoPopover {data} withHeader />
                        {:catch error}
                            <Alert size="slim" variant="danger">{error}</Alert>
                        {/await}
                    </svelte:fragment>
                </Popover>
            </svelte:fragment>
        </Section>
        <Section items={sectionItems.lang} title="By language" filterPlaceholder="Filter languages">
            <SectionItem slot="item" let:item {...item} on:select={() => handleFilterSelect('lang')}>
                <LanguageIcon slot="icon" language={item.label} inline />
            </SectionItem>
        </Section>
        <Section items={sectionItems['symbol type']} title="By symbol type" filterPlaceholder="Filter symbol types">
            <SectionItem slot="item" let:item {...item} on:select={() => handleFilterSelect('symbol type')}>
                <SymbolKindIcon slot="icon" symbolKind={item.label.toUpperCase()} />
            </SectionItem>
        </Section>
        <Section
            items={sectionItems.author}
            title="By author"
            filterPlaceholder="Filter authors"
            on:select={() => handleFilterSelect('author')}
        />
        <Section items={sectionItems['commit date']} title="By commit date">
            <SectionItem slot="item" let:item {...item} on:select={() => handleFilterSelect('commit date')}>
                <svelte:fragment slot="label">
                    {item.label}
                    <small><pre>{item.value}</pre></small>
                </svelte:fragment>
            </SectionItem>
        </Section>
        <Section items={sectionItems.file} title="By file" showAll on:select={() => handleFilterSelect('file')} />
        <Section items={sectionItems.utility} title="Utility" showAll on:select={() => handleFilterSelect('utility')} />

        <Section items={sectionItems.snippet} title="Snippets">
            <SectionItem slot="item" let:item {...item} on:select={() => handleFilterSelect('snippet')}>
                <svelte:fragment slot="label">
                    {item.label}
                    <small><pre>{item.value}</pre></small>
                </svelte:fragment>
            </SectionItem>
        </Section>

        {#if state === 'loading'}
            <LoadingSkeleton />
        {/if}

        <div class="help-footer">
            <HelpFooter />
        </div>
    </div>
    {#if selectedFilters.length > 0}
        <div class="move-button">
            <Button variant="secondary" display="block" outline on:click={() => goto(moveFiltersToQuery($page.url))}>
                Move filters to query&nbsp;
                <Icon icon={ILucideCornerRightDown} aria-hidden inline />
            </Button>
        </div>
    {/if}
</aside>

<style lang="scss">
    .sidebar {
        display: flex;
        flex-direction: column;
        height: 100%;
    }

    .scroll-container {
        display: flex;
        flex-direction: column;
        height: 100%;
        gap: 1.5rem;
        overflow-y: auto;
        padding-top: 1.25rem;
        background-color: var(--color-bg-1);

        .header {
            display: flex;
            padding: 0 1rem;
            h3 {
                margin: 0;
            }
            .reset {
                font-size: var(--font-size-tiny);
                margin-left: auto;
            }
        }

        .help-footer {
            margin-top: auto;
        }
    }

    .move-button {
        margin-top: auto;
        padding: 1rem;
        border-top: 1px solid var(--border-color);
        :global(svg) {
            transform: rotateX(180deg);
            fill: none !important;
            --icon-color: var(--body-color);
        }
    }

    pre {
        // Overwrites global default
        margin-bottom: 0;
    }
</style>
