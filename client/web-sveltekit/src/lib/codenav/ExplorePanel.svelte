<script lang="ts" context="module">
    export interface ActiveOccurrence {
        documentInfo: DocumentInfo
        occurrence: Occurrence
    }

    export interface ExplorePanelInputs {
        activeOccurrence?: ActiveOccurrence
        usageKindFilter?: SymbolUsageKind
        treeFilter?: TreeFilter
    }

    export interface ExplorePanelContext {
        openReferences(occurrence: ActiveOccurrence): void
        openDefinitions(occurrence: ActiveOccurrence): void
        openImplementations(occurrence: ActiveOccurrence): void
    }

    const exploreContextKey = Symbol('explore context key')
    export function getExplorePanelContext(): ExplorePanelContext {
        return getContext(exploreContextKey) ?? { openReferences: () => {} }
    }
    export function setExplorePanelContext(ctx: ExplorePanelContext) {
        setContext(exploreContextKey, ctx)
    }

    interface RepoTreeEntry {
        type: 'repo'
        name: string
        entries: PathTreeEntry[]
    }

    interface PathTreeEntry {
        type: 'path'
        repo: string
        name: string
    }

    type TreeEntry = RepoTreeEntry | PathTreeEntry

    interface PathGroup {
        path: string
        usages: ExplorePanel_Usage[]
    }

    interface RepoGroup {
        repo: string
        pathGroups: PathGroup[]
    }

    function groupUsages(usages: ExplorePanel_Usage[]): RepoGroup[] {
        const seenRepos: Record<string, { index: number; seenPaths: Record<string, number> }> = {}
        const repoGroups: RepoGroup[] = []

        for (const usage of usages) {
            const repo = usage.usageRange.repository
            if (seenRepos[repo] === undefined) {
                seenRepos[repo] = { index: repoGroups.length, seenPaths: {} }
                repoGroups.push({ repo, pathGroups: [] })
            }

            const path = usage.usageRange.path
            const seenPaths = seenRepos[repo].seenPaths
            const pathGroups = repoGroups[seenRepos[repo].index].pathGroups

            if (seenPaths[path] === undefined) {
                seenPaths[path] = pathGroups.length
                pathGroups.push({ path, usages: [] })
            }

            pathGroups[seenPaths[path]].usages.push(usage)
        }

        return repoGroups
    }

    function treeProviderForEntries(entries: TreeEntry[]): TreeProvider<TreeEntry> {
        return {
            getNodeID(entry) {
                if (entry.type === 'repo') {
                    return `repo-${entry.name}`
                } else {
                    return `path-${entry.repo}-${entry.name}`
                }
            },
            getEntries(): TreeEntry[] {
                return entries
            },
            isExpandable(entry) {
                return entry.type === 'repo'
            },
            isSelectable() {
                return true
            },
            fetchChildren(entry) {
                if (entry.type === 'repo') {
                    return Promise.resolve(treeProviderForEntries(entry.entries))
                } else {
                    throw new Error('path nodes are not expandable')
                }
            },
        }
    }

    function generateOutlineTree(repoGroups: RepoGroup[]): TreeProvider<TreeEntry> {
        const repoEntries: RepoTreeEntry[] = repoGroups.map(repoGroup => ({
            type: 'repo',
            name: repoGroup.repo,
            entries: repoGroup.pathGroups.map(pathGroup => ({
                type: 'path',
                name: pathGroup.path,
                repo: repoGroup.repo,
            })),
        }))
        return treeProviderForEntries(repoEntries)
    }

    export function getUsagesStore(client: GraphQLClient, documentInfo: DocumentInfo, occurrence: Occurrence) {
        const baseVariables: ExplorePanel_UsagesVariables = {
            repoName: documentInfo.repoName,
            revspec: documentInfo.commitID,
            filePath: documentInfo.filePath,
            rangeStart: occurrence.range.start,
            rangeEnd: occurrence.range.end,
            symbolComparator:
                occurrence.symbol && occurrence.symbolProvenance
                    ? {
                          name: { equals: occurrence.symbol },
                          provenance: { equals: occurrence.symbolProvenance },
                      }
                    : null,
            first: 100,
            afterCursor: null,
        }

        return infinityQuery({
            client,
            query: ExplorePanel_Usages,
            variables: baseVariables,
            map: result => ({
                data: result?.data?.usagesForSymbol?.nodes,
                error: result?.error,
                nextVariables: result?.data?.usagesForSymbol?.pageInfo?.hasNextPage
                    ? {
                          ...baseVariables,
                          afterCursor: result?.data?.usagesForSymbol?.pageInfo?.endCursor,
                      }
                    : undefined,
            }),
            merge: (previous, next) => (previous ?? []).concat(next ?? []),
        })
    }

    function matchesUsageKind(usageKindFilter: SymbolUsageKind | undefined): (usage: ExplorePanel_Usage) => boolean {
        return usage => usageKindFilter === undefined || usage.usageKind === usageKindFilter
    }

    interface TreeFilter {
        repository: string
        path?: string
    }

    export function entryIDForFilter(filter: TreeFilter): string {
        if (filter.path) {
            return `path-${filter.repository}-${filter.path}`
        }
        return `repo-${filter.repository}`
    }
</script>

<script lang="ts">
    import { getContext, setContext } from 'svelte'
    import { type Writable } from 'svelte/store'

    import { infinityQuery, type GraphQLClient, type InfinityQueryStore } from '$lib/graphql'
    import { SymbolUsageKind } from '$lib/graphql-types'
    import Icon from '$lib/Icon.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import Scroller from '$lib/Scroller.svelte'
    import CodeHostIcon from '$lib/search/CodeHostIcon.svelte'
    import LoadingSkeleton from '$lib/search/dynamicFilters/LoadingSkeleton.svelte'
    import { displayRepoName, Occurrence } from '$lib/shared'
    import { type SingleSelectTreeState, type TreeProvider } from '$lib/TreeView'
    import TreeView, { setTreeContext } from '$lib/TreeView.svelte'
    import type { DocumentInfo } from '$lib/web'
    import { Alert, PanelGroup } from '$lib/wildcard'
    import Panel from '$lib/wildcard/resizable-panel/Panel.svelte'
    import PanelResizeHandle from '$lib/wildcard/resizable-panel/PanelResizeHandle.svelte'

    import type { ExplorePanel_Usage, ExplorePanel_UsagesVariables } from './ExplorePanel.gql'
    import { ExplorePanel_Usages } from './ExplorePanel.gql'
    import ExplorePanelFileUsages from './ExplorePanelFileUsages.svelte'

    export let inputs: Writable<ExplorePanelInputs>
    export let connection: InfinityQueryStore<ExplorePanel_Usage[], ExplorePanel_UsagesVariables> | undefined
    export let treeState: Writable<SingleSelectTreeState>

    $: setTreeContext(treeState)

    // TODO: it would be really nice if the tree API emitted select events with tree elements, not HTML elements
    function handleSelect(target: HTMLElement) {
        const selected = target.querySelector('[data-repo-name]') as HTMLElement
        const repository = selected.dataset.repoName ?? ''
        const path = selected.dataset.path
        inputs.update(old => {
            const deselect = old.treeFilter && old.treeFilter.repository === repository && old.treeFilter.path === path
            return {
                ...old,
                treeFilter: deselect ? undefined : { repository, path },
            }
        })
    }

    $: loading = $connection?.fetching
    $: usages = $connection?.data
    $: kindFilteredUsages = usages?.filter(matchesUsageKind($inputs.usageKindFilter))
    $: repoGroups = groupUsages(kindFilteredUsages ?? [])
    $: outlineTree = generateOutlineTree(repoGroups)
    $: displayGroups = repoGroups
        .flatMap(repoGroup => repoGroup.pathGroups.map(pathGroup => ({ repo: repoGroup.repo, ...pathGroup })))
        .filter(displayGroup => {
            if ($inputs.treeFilter === undefined) {
                return true
            } else if ($inputs.treeFilter.repository !== displayGroup.repo) {
                return false
            }
            return $inputs.treeFilter.path === undefined || $inputs.treeFilter.path === displayGroup.path
        })

    let referencesScroller: HTMLElement | undefined
</script>

{#if $inputs.activeOccurrence === undefined}
    <div class="no-selection">
        <Icon icon={ISgSymbols} />
        <p>Select a symbol in the code panel to view references</p>
    </div>
{:else}
    <PanelGroup id="references">
        <Panel id="references-sidebar" defaultSize={25} minSize={20} maxSize={60}>
            <div class="sidebar">
                <fieldset>
                    <legend hidden>Select usage kind</legend>
                    {#each Object.values(SymbolUsageKind) as usageKind}
                        {@const checked = usageKind === $inputs.usageKindFilter}
                        {@const id = `usage-kind-${usageKind}`}
                        <input
                            type="radio"
                            bind:group={$inputs.usageKindFilter}
                            name="usageKind"
                            value={usageKind}
                            {id}
                            {checked}
                        />
                        <label for={id}>{usageKind.toLowerCase()}s</label>
                    {/each}
                </fieldset>
                <div class="outline">
                    {#if repoGroups.length > 0}
                        <h4>Filter by location</h4>
                        <TreeView treeProvider={outlineTree} on:select={event => handleSelect(event.detail)}>
                            <svelte:fragment let:entry>
                                {#if entry.type === 'repo'}
                                    <span class="repo-entry" data-repo-name={entry.name}>
                                        <CodeHostIcon repository={entry.name} />
                                        {displayRepoName(entry.name)}
                                    </span>
                                {:else}
                                    <span class="path-entry" data-repo-name={entry.repo} data-path={entry.name}>
                                        {entry.name}
                                    </span>
                                {/if}
                            </svelte:fragment>
                            <Alert slot="error" let:error variant="danger">
                                {error.message}
                            </Alert>
                        </TreeView>
                    {/if}
                </div>
            </div>
        </Panel>
        <PanelResizeHandle />
        <Panel id="references-content">
            {#if loading}
                <div class="loading">
                    <LoadingSkeleton />
                    <LoadingSkeleton />
                    <LoadingSkeleton />
                </div>
            {:else}
                <Scroller bind:viewport={referencesScroller} margin={600} on:more={() => connection?.fetchMore()}>
                    {#if displayGroups.length > 0}
                        <ul>
                            {#each displayGroups as pathGroup}
                                <li>
                                    <ExplorePanelFileUsages scrollContainer={referencesScroller} {...pathGroup} />
                                </li>
                            {/each}
                        </ul>
                        {#if loading}
                            <LoadingSpinner center />
                        {/if}
                    {:else}
                        <div class="no-results">No results.</div>
                    {/if}
                </Scroller>
            {/if}
        </Panel>
    </PanelGroup>
{/if}

<style lang="scss">
    .sidebar {
        height: 100%;
        overflow-y: auto;
    }

    fieldset {
        display: flex;
        flex-direction: column;
        border-bottom: 1px solid var(--border-color);
        padding: 0.25rem 0;

        input {
            appearance: none;
        }

        label {
            text-transform: capitalize;
            cursor: pointer;
            padding: 0.375rem 0.75rem;
            background-color: transparent;
        }

        input:checked + label {
            background-color: var(--primary);
            color: var(--light-text);
        }

        input:not(:checked) + label:hover {
            background-color: var(--secondary-4);
        }
    }

    .outline {
        h4 {
            padding: 0.5rem 0.75rem;
            margin: 0;
        }
        padding: 0rem;

        :global([data-treeitem]) > :global([data-treeitem-label]) {
            cursor: pointer;

            &:hover {
                background-color: var(--secondary-4);
            }

            word-break: break-all;
        }

        :global([data-treeitem][aria-selected='true']) > :global([data-treeitem-label]) {
            --tree-node-expand-icon-color: var(--body-bg);
            --file-icon-color: var(--body-bg);
            --tree-node-label-color: var(--body-bg);

            background-color: var(--primary);
            &:hover {
                background-color: var(--primary);
            }
        }
    }

    .repo-entry {
        display: flex;
        align-items: center;
        gap: 0.375em;
        font-size: var(--font-size-base);
    }
    .path-entry {
        font-size: var(--font-size-small);
    }

    ul {
        all: unset;
        li {
            all: unset;
        }
    }

    .no-selection {
        :global([data-icon]) {
            flex: none;
        }
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: 1rem;
        padding: 1rem;
        text-align: center;

        height: 100%;
        width: 100%;

        --icon-color: currentColor;

        color: var(--text-muted);
        font-weight: 500;
    }

    .no-results {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        width: 100%;
        color: var(--text-muted);
        font-weight: 500;
    }

    .loading {
        display: flex;
        flex-direction: column;
        align-items: stretch;
        width: 100%;
        height: 100%;
        padding: 1rem;
    }
</style>
