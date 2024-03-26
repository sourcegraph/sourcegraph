<script lang="ts">
    import { onMount, tick } from 'svelte'

    import { afterNavigate, disableScrollHandling, goto } from '$app/navigation'
    import { page } from '$app/stores'
    import { isErrorLike } from '$lib/common'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import { fetchSidebarFileTree } from '$lib/repo/api/tree'
    import HistoryPanel, { type Capture as HistoryCapture } from '$lib/repo/HistoryPanel.svelte'
    import SidebarToggleButton from '$lib/repo/SidebarToggleButton.svelte'
    import { sidebarOpen } from '$lib/repo/stores'
    import Separator, { getSeparatorPosition } from '$lib/Separator.svelte'
    import { scrollAll } from '$lib/stores'
    import TabPanel from '$lib/TabPanel.svelte'
    import Tabs from '$lib/Tabs.svelte'
    import { Alert } from '$lib/wildcard'

    import type { LayoutData, Snapshot } from './$types'
    import FileTree from './FileTree.svelte'
    import { createFileTreeStore } from './fileTreeStore'
    import type { GitHistory_HistoryConnection } from './layout.gql'

    interface Capture {
        selectedTab: number | null
        historyPanel: HistoryCapture
        scrollTop: number
    }

    export let data: LayoutData

    export const snapshot: Snapshot<Capture> = {
        capture() {
            return {
                selectedTab,
                historyPanel: historyPanel?.capture(),
                // This works because this specific page is fully scrollable
                scrollTop: window.scrollY,
            }
        },
        async restore(data) {
            selectedTab = data.selectedTab
            // Wait until DOM was updated
            await tick()
            // `restore` is called before `afterNavigate`, which resets the scroll position
            // Restore the scroll position after the componentent was updated
            window.scrollTo(0, data.scrollTop)

            // Restore history panel state if it is open
            if (data.historyPanel) {
                historyPanel?.restore(data.historyPanel)
            }
        },
    }

    async function selectTab(event: { detail: number | null }) {
        if (event.detail === null) {
            const url = new URL($page.url)
            url.searchParams.delete('rev')
            await goto(url, { replaceState: true, keepFocus: true, noScroll: true })
        }
        selectedTab = event.detail
    }

    const fileTreeStore = createFileTreeStore({ fetchFileTreeData: fetchSidebarFileTree })
    let selectedTab: number | null = null
    let historyPanel: HistoryPanel
    let rootElement: HTMLElement | null = null
    let commitHistory: GitHistory_HistoryConnection | null

    $: ({ revision = '', parentPath, repoName, resolvedRevision } = data)
    $: fileTreeStore.set({ repoName, revision: resolvedRevision.commitID, path: parentPath })
    $: commitHistoryQuery = data.commitHistory
    $: latestCommit = commitHistory?.nodes[0]
    $: if (!!commitHistoryQuery) {
        // Reset commit history when the query observable changes. Without
        // this we are showing the commit history of the previously selected
        // file/folder until the new commit history is loaded.
        commitHistory = null
    }
    $: commitHistory = $commitHistoryQuery?.data?.repository?.commit?.ancestors ?? null

    const sidebarSize = getSeparatorPosition('repo-sidebar', 0.2)
    $: sidebarWidth = `max(200px, ${$sidebarSize * 100}%)`

    onMount(() => {
        // We want the whole page to be scrollable and hide page and repo navigation
        scrollAll.set(true)
        return () => scrollAll.set(false)
    })

    afterNavigate(() => {
        // When navigating to a new page we want to ensure two things:
        // - The file sidebar doesn't move. It feels bad when you clicked on a file entry
        //   and the click target moves away because the page is scrolled all the way to the top.
        // - The beginning of the content should be visible (e.g. the top of the file or the
        //   top of the file table).
        // In other words, we want to scroll to the top but not all the way

        // Prevents SvelteKit from resetting the scroll position to the very top of the page
        disableScrollHandling()

        if (rootElement) {
            // Because the whole page is scrollable we can get the current scroll position from
            // the window object
            const top = rootElement.offsetTop
            if (window.scrollY > top) {
                // Reset scroll to top of the content
                window.scrollTo(0, top)
            }
        }
    })
</script>

<section bind:this={rootElement}>
    <div class="sidebar" class:open={$sidebarOpen} style:min-width={sidebarWidth} style:max-width={sidebarWidth}>
        <h3>
            <SidebarToggleButton />&nbsp; Files
        </h3>
        {#if $fileTreeStore}
            {#if isErrorLike($fileTreeStore)}
                <Alert variant="danger">
                    Unable to fetch file tree data:
                    {$fileTreeStore.message}
                </Alert>
            {:else}
                <FileTree {repoName} {revision} treeProvider={$fileTreeStore} selectedPath={$page.params.path ?? ''} />
            {/if}
        {:else}
            <LoadingSpinner center={false} />
        {/if}
    </div>
    {#if $sidebarOpen}
        <Separator currentPosition={sidebarSize} />
    {/if}
    <div class="main">
        <slot />
        <div class="bottom-panel" class:open={selectedTab !== null}>
            <Tabs selected={selectedTab} toggable on:select={selectTab}>
                <TabPanel title="History">
                    {#key $page.params.path}
                        <HistoryPanel
                            bind:this={historyPanel}
                            history={commitHistory}
                            loading={$commitHistoryQuery?.fetching ?? true}
                            fetchMore={commitHistoryQuery.fetchMore}
                            enableInlineDiffs={$page.route.id?.includes('/blob/') ?? false}
                        />
                    {/key}
                </TabPanel>
                <TabPanel title="Blame">hi</TabPanel>
            </Tabs>
            <div class="latest-commit">
                {latestCommit?.author?.person?.avatarURL}
                {latestCommit?.author?.person?.avatarURL}
                {latestCommit?.author?.person?.displayName}
                {latestCommit?.subject}
                {latestCommit?.author?.date}
                | no owner
            </div>
        </div>
    </div>
</section>

<style lang="scss">
    section {
        display: flex;
        flex: 1;
        flex-shrink: 0;
        background-color: var(--code-bg);
        min-height: 100vh;
    }

    .sidebar {
        &.open {
            display: flex;
            flex-direction: column;
        }
        display: none;
        overflow: hidden;
        background-color: var(--body-bg);
        padding: 0.5rem;
        padding-bottom: 0;
        position: sticky;
        top: 0px;
        max-height: 100vh;
    }

    .main {
        flex: 1;
        display: flex;
        flex-direction: column;
        min-width: 0;
    }

    h3 {
        display: flex;
        align-items: center;
        margin-bottom: 0.5rem;
    }

    .bottom-panel {
        position: sticky;
        bottom: 0px;
        background-color: var(--code-bg);
        --align-tabs: flex-start;
        border-top: 1px solid var(--border-color);
        max-height: 50vh;
        overflow: hidden;
        display: flex;
        flex-flow: row nowrap;
        justify-content: space-between;

        :global(.tabs) {
            height: 100%;
            max-height: 100%;
            overflow: hidden;
        }

        :global(.tabs-header) {
            border-bottom: 1px solid var(--border-color);
        }

        &.open {
            height: 30vh;
        }
    }
</style>
