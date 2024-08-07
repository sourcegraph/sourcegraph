<script context="module" lang="ts">
    import type { Keys } from '$lib/Hotkey'
    import type { Capture as HistoryCapture } from '$lib/repo/HistoryPanel.svelte'
    import { TELEMETRY_RECORDER } from '$lib/telemetry'

    enum TabPanels {
        History,
        References,
    }

    interface Capture {
        selectedTab: number | null
        historyPanel: HistoryCapture
        exploreInputs: ExplorePanelInputs | undefined
        // TODO: consider also capturing the file in the references panel
    }

    // Not ideal solution, [TODO] Improve Tabs component API in order
    // to expose more info about nature of switch tab / close tab actions
    function trackHistoryPanelTabAction(selectedTab: number | null, nextSelectedTab: number | null) {
        if (nextSelectedTab === 0) {
            TELEMETRY_RECORDER.recordEvent('repo.historyPanel', 'show')
            return
        }

        if (nextSelectedTab === null && selectedTab == 0) {
            TELEMETRY_RECORDER.recordEvent('repo.historyPanel', 'hide')
            return
        }
    }

    const historyHotkey: Keys = {
        key: 'alt+h',
    }

    const referenceHotkey: Keys = {
        key: 'alt+r',
    }
</script>

<script lang="ts">
    import { onMount } from 'svelte'
    import { get, writable } from 'svelte/store'

    import { goto } from '$app/navigation'
    import { page } from '$app/stores'
    import ExplorePanel, {
        setExplorePanelContext,
        getUsagesStore,
        entryIDForFilter,
        type ExplorePanelInputs,
        type ActiveOccurrence,
    } from '$lib/codenav/ExplorePanel.svelte'
    import CodySidebar from '$lib/cody/CodySidebar.svelte'
    import { isErrorLike } from '$lib/common'
    import { openFuzzyFinder } from '$lib/fuzzyfinder/FuzzyFinderContainer.svelte'
    import { filesHotkey } from '$lib/fuzzyfinder/keys'
    import { getGraphQLClient } from '$lib/graphql'
    import { SymbolUsageKind } from '$lib/graphql-types'
    import Icon from '$lib/Icon.svelte'
    import KeyboardShortcut from '$lib/KeyboardShortcut.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import { fetchSidebarFileTree } from '$lib/repo/api/tree'
    import HistoryPanel from '$lib/repo/HistoryPanel.svelte'
    import LastCommit from '$lib/repo/LastCommit.svelte'
    import RepositoryRevPicker from '$lib/repo/RepositoryRevPicker.svelte'
    import { rightSidePanelOpen, fileTreeSidePanel } from '$lib/repo/stores'
    import { isViewportMobile } from '$lib/stores'
    import TabPanel from '$lib/TabPanel.svelte'
    import Tabs from '$lib/Tabs.svelte'
    import Tooltip from '$lib/Tooltip.svelte'
    import { createEmptySingleSelectTreeState, type SingleSelectTreeState } from '$lib/TreeView'
    import { Alert, PanelGroup, Panel, PanelResizeHandle, Button } from '$lib/wildcard'
    import { getButtonClassName } from '$lib/wildcard/Button'

    import type { LayoutData, Snapshot } from './$types'
    import FileTree from './FileTree.svelte'
    import { createFileTreeStore } from './fileTreeStore'

    export let data: LayoutData

    export const snapshot: Snapshot<Capture> = {
        capture() {
            return {
                selectedTab,
                historyPanel: historyPanel?.capture(),
                exploreInputs: get(exploreInputs),
            }
        },
        restore(data) {
            selectedTab = data.selectedTab
            if (data.exploreInputs) {
                exploreInputs.set(data.exploreInputs)
            }
            if (data.historyPanel) {
                historyPanel?.restore(data.historyPanel)
            }
        },
    }

    let bottomPanel: Panel
    let rightSidePanel: Panel
    let historyPanel: HistoryPanel
    let selectedTab: number | null = null
    const fileTreeStore = createFileTreeStore({ fetchFileTreeData: fetchSidebarFileTree })

    $: ({ revision = '', parentPath, repoName, resolvedRevision, isCodyAvailable } = data)
    $: fileTreeStore.set({ repoName, revision: resolvedRevision.commitID, path: parentPath })

    const exploreTreeState = writable<SingleSelectTreeState>({
        ...createEmptySingleSelectTreeState(),
        disableScope: true,
    })
    const exploreInputs = writable<ExplorePanelInputs>({})
    function openExploreTab(usageKindFilter: SymbolUsageKind, occurrence: ActiveOccurrence) {
        exploreInputs.set({ activeOccurrence: occurrence, usageKindFilter })
        // Open the tab when we find references
        selectedTab = TabPanels.References
    }
    setExplorePanelContext({
        openReferences: openExploreTab.bind(null, SymbolUsageKind.REFERENCE),
        openDefinitions: openExploreTab.bind(null, SymbolUsageKind.DEFINITION),
        openImplementations: openExploreTab.bind(null, SymbolUsageKind.IMPLEMENTATION),
    })
    $: usagesConnection = $exploreInputs.activeOccurrence
        ? getUsagesStore(
              getGraphQLClient(),
              $exploreInputs.activeOccurrence.documentInfo,
              $exploreInputs.activeOccurrence.occurrence
          )
        : undefined
    $: exploreTreeState.update(old => ({
        ...old,
        selected: $exploreInputs.treeFilter ? entryIDForFilter($exploreInputs.treeFilter) : '',
    }))
    function selectTab(event: { detail: number | null }) {
        trackHistoryPanelTabAction(selectedTab, event.detail)

        if (event.detail === null) {
            handleBottomPanelCollapse().catch(() => {})
        }
        selectedTab = event.detail
    }

    function handleBottomPanelExpand() {
        if (selectedTab == null) {
            selectedTab = TabPanels.History
        }
    }

    async function handleBottomPanelCollapse() {
        // Removing the URL parameter causes the diff view to close
        if ($page.url.searchParams.has('rev')) {
            const url = new URL($page.url)
            url.searchParams.delete('rev')
            await goto(url, { replaceState: true, keepFocus: true, noScroll: true })
        }
        selectedTab = null
    }

    function toggleFileSidePanel() {
        if ($fileTreeSidePanel?.isExpanded()) {
            $fileTreeSidePanel?.collapse()
        } else {
            $fileTreeSidePanel?.expand()
        }
    }

    $: {
        if (selectedTab == null) {
            bottomPanel?.collapse()
        } else if (!bottomPanel?.isExpanded()) {
            bottomPanel?.expand()
        }
    }

    $: if ($isViewportMobile) {
        if ($rightSidePanelOpen) {
            rightSidePanel?.expand()
        } else {
            rightSidePanel?.collapse()
        }
    }

    onMount(() => {
        if ($isViewportMobile) {
            // Ensure that cody sidebar is closed on mobile
            $rightSidePanelOpen = false
        }
    })
    const sidebarButtonClass = getButtonClassName({ variant: 'secondary', outline: true, size: 'sm' })
</script>

<PanelGroup id="blob-page-panels" direction="horizontal">
    <Panel
        bind:this={$fileTreeSidePanel}
        id="sidebar-panel"
        order={1}
        defaultSize={1}
        minSize={15}
        maxSize={35}
        collapsible
        collapsedSize={1}
        overlayOnMobile
        let:isCollapsed
    >
        <div class="sidebar" class:collapsed={isCollapsed}>
            <header>
                <Tooltip tooltip="{isCollapsed ? 'Open' : 'Close'} sidebar">
                    <button class="{sidebarButtonClass} collapse-button" on:click={toggleFileSidePanel}>
                        <Icon icon={isCollapsed ? ILucidePanelLeftOpen : ILucidePanelLeftClose} inline aria-hidden />
                    </button>
                </Tooltip>

                {#if data.isPerforceDepot}
                    <RepositoryRevPicker
                        display="block"
                        repoURL={data.repoURL}
                        revision={data.revision}
                        commitID={data.resolvedRevision.commitID}
                        defaultBranch={data.defaultBranch}
                        getDepotChangelists={data.getDepotChangelists}
                    />
                {:else}
                    <RepositoryRevPicker
                        display="block"
                        repoURL={data.repoURL}
                        revision={data.revision}
                        commitID={data.resolvedRevision.commitID}
                        defaultBranch={data.defaultBranch}
                        getRepositoryBranches={data.getRepoBranches}
                        getRepositoryCommits={data.getRepoCommits}
                        getRepositoryTags={data.getRepoTags}
                    />
                {/if}

                <Tooltip tooltip={isCollapsed ? 'Open search fuzzy finder' : ''}>
                    <button class="{sidebarButtonClass} search-files-button" on:click={() => openFuzzyFinder('files')}>
                        {#if isCollapsed}
                            <Icon icon={ILucideSquareSlash} inline aria-hidden />
                        {:else}
                            <span>Search files</span>
                            <KeyboardShortcut shortcut={filesHotkey} />
                        {/if}
                    </button>
                </Tooltip>
            </header>

            <div class="sidebar-file-tree">
                {#if $fileTreeStore}
                    {#if isErrorLike($fileTreeStore)}
                        <Alert variant="danger">
                            Unable to fetch file tree data:
                            {$fileTreeStore.message}
                        </Alert>
                    {:else}
                        <FileTree
                            {repoName}
                            {revision}
                            treeProvider={$fileTreeStore}
                            selectedPath={data.filePath ?? ''}
                        />
                    {/if}
                {:else}
                    <LoadingSpinner />
                {/if}
            </div>
        </div>
    </Panel>

    <PanelResizeHandle id="blob-page-panels-separator" />

    <Panel id="blob-content-panels" order={2}>
        <PanelGroup id="content-panels" direction="vertical">
            <Panel id="content-panel" order={1}>
                <PanelGroup id="content-sidebar-panels">
                    <Panel order={1} id="main-content-panel">
                        <slot />
                    </Panel>
                    {#if $isCodyAvailable && $rightSidePanelOpen}
                        <PanelResizeHandle id="right-sidebar-resize-handle" />
                        <Panel
                            id="right-sidebar-panel"
                            bind:this={rightSidePanel}
                            order={2}
                            minSize={20}
                            maxSize={70}
                            overlayOnMobile
                            onClose={() => ($rightSidePanelOpen = false)}
                        >
                            <CodySidebar
                                repository={data.resolvedRevision.repo}
                                filePath={data.filePath}
                                lineOrPosition={data.lineOrPosition}
                                on:close={() => ($rightSidePanelOpen = false)}
                            />
                        </Panel>
                    {/if}
                </PanelGroup>
            </Panel>
            <PanelResizeHandle />
            <Panel
                bind:this={bottomPanel}
                id="bottom-actions-panel"
                order={2}
                defaultSize={1}
                minSize={20}
                maxSize={70}
                collapsible
                collapsedSize={1}
                onExpand={handleBottomPanelExpand}
                onCollapse={handleBottomPanelCollapse}
                let:isCollapsed
            >
                <Tabs selected={selectedTab} toggable on:select={selectTab}>
                    <svelte:fragment slot="header-actions">
                        {#if !isCollapsed}
                            <Button
                                variant="text"
                                size="sm"
                                aria-label="Hide bottom panel"
                                on:click={handleBottomPanelCollapse}
                            >
                                <Icon icon={ILucideArrowDownFromLine} inline aria-hidden /> Hide
                            </Button>
                        {:else}
                            {#await data.lastCommit then lastCommit}
                                {#if lastCommit && isCollapsed}
                                    <div class="last-commit">
                                        <LastCommit {lastCommit} />
                                    </div>
                                {/if}
                            {/await}
                        {/if}
                    </svelte:fragment>
                    <TabPanel title="History" shortcut={historyHotkey}>
                        {#key data.filePath}
                            <HistoryPanel
                                bind:this={historyPanel}
                                history={data.commitHistory}
                                enableInlineDiff={$page.data.enableInlineDiff}
                                enableViewAtCommit={$page.data.enableViewAtCommit}
                            />
                        {/key}
                    </TabPanel>
                    <TabPanel title="Explore" shortcut={referenceHotkey}>
                        <ExplorePanel
                            inputs={exploreInputs}
                            connection={usagesConnection}
                            treeState={exploreTreeState}
                        />
                    </TabPanel>
                </Tabs>
            </Panel>
        </PanelGroup>
    </Panel>
</PanelGroup>

<style lang="scss">
    :global([data-panel-group-id='blob-page-panels']) {
        overflow: hidden;
        background-color: var(--code-bg);
    }

    // Forcing the left sidebar panel (file-tree) to be over
    // right content panel to make sure that box-shadow of the
    // sidebar is rendered over content panel.
    :global([data-panel-id='sidebar-panel']) {
        z-index: 1;
        isolation: isolate;
        box-shadow: var(--sidebar-shadow);
        min-width: 2.92rem;
    }

    :global([data-panel-id='blob-content-panels']) {
        z-index: 0;
        isolation: isolate;
    }

    :global([data-panel-resize-handle-id='right-sidebar-resize-handle']),
    :global([data-panel-resize-handle-id='blob-page-panels-separator']) {
        &::before {
            // Even though side-panel shadow should be rendered over
            // the right content panel, resize handle still should be rendered
            // over any panel elements
            z-index: 2 !important;
        }

        @media (--mobile) {
            // !important is necessary to overwrit the elements own style
            display: none !important;
        }
    }

    .sidebar {
        display: flex;
        flex-direction: column;
        background-color: var(--color-bg-1);

        // Allow sidebar to shrink
        min-height: 0;
        height: 100%;

        header {
            display: grid;
            grid-template-columns: min-content 1fr;
            grid-template-areas:
                'collapse-button rev-picker'
                'search-files search-files';
            gap: 0.375rem;
            padding: 0.5rem;

            .collapse-button {
                grid-area: collapse-button;
            }

            :global([data-repo-rev-picker-trigger]) {
                grid-area: rev-picker;
                min-width: 0;
            }

            .search-files-button {
                grid-area: search-files;

                display: flex;
                align-items: center;
                justify-content: space-between;
                gap: 0.25rem;

                font-size: var(--font-size-xs);
                color: var(--text-body);

                span {
                    white-space: nowrap;
                    text-overflow: ellipsis;
                    overflow: hidden;
                    flex-grow: 1;
                    flex-shrink: 1;
                    text-align: left;
                }
            }

            @media (--mobile) {
                grid-template-columns: 1fr;
                grid-template-areas: 'rev-picker' 'search-files';

                .collapse-button {
                    display: none;
                }
            }
        }

        .sidebar-file-tree {
            flex-grow: 1;
            min-height: 0;
            overflow: auto;
            padding: 0.25rem 0 0.5rem 0;
            border-top: 1px solid var(--border-color);
        }

        &.collapsed {
            header {
                grid-template-columns: min-content;
                grid-template-areas:
                    'collapse-button'
                    'search-files';

                :global([data-repo-rev-picker-trigger]) {
                    display: none;
                }

                .search-files-button span {
                    display: none;
                }
            }
            .sidebar-file-tree {
                display: none;
            }
        }
    }

    :global([data-panel-id='main-content-panel']) {
        display: flex;
        flex-direction: column;
        min-width: 0;
        overflow: hidden;
    }

    :global([data-panel-id='bottom-actions-panel']) {
        background: var(--color-bg-1);
        min-height: 2.5625rem; // 41px which is bottom panel compact size
        box-shadow: var(--bottom-panel-shadow);
    }

    :global([data-panel-id='right-sidebar-panel']) {
        z-index: 1;
        box-shadow: 0 0 4px rgba(0, 0, 0, 0.1);
    }

    .info {
        padding: 1rem;
    }
</style>
