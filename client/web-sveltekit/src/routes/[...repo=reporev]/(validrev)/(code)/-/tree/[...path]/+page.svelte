<script lang="ts">
    // @sg EnableRollout
    import { afterNavigate, beforeNavigate } from '$app/navigation'
    import Icon from '$lib/Icon.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import FileHeader from '$lib/repo/FileHeader.svelte'
    import type { TreeEntryWithCommitInfo } from '$lib/repo/FileTable.gql'
    import FileTable from '$lib/repo/FileTable.svelte'
    import OpenCodyAction from '$lib/repo/OpenCodyAction.svelte'
    import Permalink from '$lib/repo/Permalink.svelte'
    import Readme from '$lib/repo/Readme.svelte'
    import { createPromiseStore } from '$lib/utils'
    import { Alert } from '$lib/wildcard'

    import { getRepositoryPageContext } from '../../../../../context'

    import type { PageData } from './$types'

    export let data: PageData

    const repositoryContext = getRepositoryPageContext()
    const treeEntriesWithCommitInfo = createPromiseStore<TreeEntryWithCommitInfo[]>()

    $: treeEntriesWithCommitInfo.set(data.treeEntriesWithCommitInfo)
    $: isCodyAvailable = data.isCodyAvailable

    afterNavigate(() => {
        repositoryContext.set({ directoryPath: data.filePath })
    })
    beforeNavigate(() => {
        repositoryContext.set({})
    })
</script>

<svelte:head>
    <title>{data.filePath} - {data.displayRepoName} - Sourcegraph</title>
</svelte:head>

<FileHeader type="tree" repoName={data.repoName} revision={data.revision} path={data.filePath}>
    <svelte:fragment slot="actions">
        <Permalink revID={data.resolvedRevision.commitID} />
        {#if isCodyAvailable}
            <OpenCodyAction />
        {/if}
    </svelte:fragment>
</FileHeader>

<div class="content">
    {#await data.treeEntries}
        <LoadingSpinner />
    {:then result}
        <!-- File path does not exist -->
        {#if result === null}
            <div class="error-wrapper">
                <div class="circle">
                    <Icon icon={ILucideSearchX} --icon-size="80px" />
                </div>
                <h2>Directory not found</h2>
            </div>
        {:else if result.entries.length === 0}
            <Alert variant="info">This directory is empty.</Alert>
        {:else}
            {#if $treeEntriesWithCommitInfo}
                {#if $treeEntriesWithCommitInfo.error}
                    <Alert variant="danger">
                        Unable to load commit information: {$treeEntriesWithCommitInfo.error.message}
                    </Alert>
                {/if}
            {/if}
            <FileTable
                revision={data.revision ?? ''}
                entries={result.entries}
                commitInfo={$treeEntriesWithCommitInfo.value ?? []}
            />
        {/if}
    {:catch error}
        <Alert variant="danger">
            Unable to load directory information: {error.message}
        </Alert>
    {/await}
    {#await data.readme then readme}
        {#if readme}
            <h4 class="header">
                {readme.name}
            </h4>
            <div class="readme">
                <Readme file={readme} />
            </div>
        {/if}
    {:catch error}
        <Alert variant="danger">
            Unable to load README: {error.message}
        </Alert>
    {/await}
</div>

<style lang="scss">
    .content {
        flex: 1;
        overflow: auto;
    }

    .header {
        background-color: var(--body-bg);
        position: sticky;
        top: 0;
        padding: 0.5rem;
        border-bottom: 1px solid var(--border-color);
        margin: 0;
    }

    .readme {
        padding: 1rem;
        flex: 1;
    }

    .error-wrapper {
        display: flex;
        flex-direction: column;
        align-items: center;
    }

    .circle {
        background-color: var(--color-bg-2);
        border-radius: 50%;
        padding: 1.5rem;
        margin: 1rem;
    }
</style>
