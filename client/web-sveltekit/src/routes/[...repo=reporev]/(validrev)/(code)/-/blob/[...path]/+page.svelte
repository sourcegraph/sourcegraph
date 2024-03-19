<svelte:options immutable />

<script lang="ts">
    import { mdiCodeBracesBox, mdiFileCodeOutline, mdiMapSearch } from '@mdi/js'
    import { from } from 'rxjs'

    import { goto } from '$app/navigation'
    import { page } from '$app/stores'
    import CodeMirrorBlob from '$lib/CodeMirrorBlob.svelte'
    import { isErrorLike, type LineOrPositionOrRange } from '$lib/common'
    import { toGraphQLResult } from '$lib/graphql'
    import Icon from '$lib/Icon.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import { updateSearchParamsWithLineInformation, createBlobDataHandler } from '$lib/repo/blob'
    import FileDiff from '$lib/repo/FileDiff.svelte'
    import FileHeader from '$lib/repo/FileHeader.svelte'
    import Permalink from '$lib/repo/Permalink.svelte'
    import { createCodeIntelAPI, parseQueryAndHash } from '$lib/shared'
    import { Alert } from '$lib/wildcard'

    import type { PageData } from './$types'
    import FormatAction from './FormatAction.svelte'
    import WrapLinesAction, { lineWrap } from './WrapLinesAction.svelte'

    export let data: PageData

    const combinedBlobData = createBlobDataHandler()
    let selectedPosition: LineOrPositionOrRange | null = null

    $: ({
        revision,
        resolvedRevision: { commitID },
        repoName,
        filePath,
        settings,
        graphQLClient,
    } = data)
    $: combinedBlobData.set(data.blob, data.highlights)
    $: ({ blob, highlights, blobPending } = $combinedBlobData)
    $: formatted = !!blob?.richHTML
    $: fileNotFound = !blob && !blobPending
    $: fileLoadingError = (!blobPending && !blob && $combinedBlobData.blobError) || null
    $: showRaw = $page.url.searchParams.get('view') === 'raw'
    $: codeIntelAPI = createCodeIntelAPI({
        settings: setting => (isErrorLike(settings?.final) ? undefined : settings?.final?.[setting]),
        requestGraphQL(options) {
            return from(graphQLClient.query(options.request, options.variables).then(toGraphQLResult))
        },
    })
    $: if (!blobPending) {
        // Update selected position as soon as blob is loaded
        selectedPosition = parseQueryAndHash($page.url.search, $page.url.hash)
    }
</script>

<svelte:head>
    <title>{filePath} - {data.displayRepoName} - Sourcegraph</title>
</svelte:head>

<FileHeader>
    <Icon slot="icon" svgPath={data.compare ? mdiCodeBracesBox : mdiFileCodeOutline} />
    <svelte:fragment slot="actions">
        {#if data.compare}
            <span>{data.compare.revisionToCompare}</span>
        {:else}
            {#if !formatted || showRaw}
                <WrapLinesAction />
            {/if}
            {#if formatted}
                <FormatAction />
            {/if}
            <Permalink {commitID} />
        {/if}
    </svelte:fragment>
</FileHeader>

<div class="content" class:loading={blobPending} class:compare={!!data.compare} class:fileNotFound>
    {#if !$combinedBlobData.highlightsPending && $combinedBlobData.highlightsError}
        <Alert variant="danger">
            Unable to load syntax highlighting: {$combinedBlobData.highlightsError.message}
        </Alert>
    {/if}
    {#if data.compare}
        {#await data.compare.diff}
            <LoadingSpinner />
        {:then fileDiff}
            {#if fileDiff}
                <FileDiff {fileDiff} />
            {:else}
                Unable to load iff
            {/if}
        {/await}
    {:else if blob}
        {#if blob.richHTML && !showRaw}
            <div class="rich">
                {@html blob.richHTML}
            </div>
        {:else}
            <CodeMirrorBlob
                blobInfo={{
                    ...blob,
                    revision: revision ?? '',
                    commitID,
                    repoName: repoName,
                    filePath,
                }}
                {highlights}
                wrapLines={$lineWrap}
                selectedLines={selectedPosition?.line ? selectedPosition : null}
                on:selectline={event => {
                    goto('?' + updateSearchParamsWithLineInformation($page.url.searchParams, event.detail))
                }}
                {codeIntelAPI}
            />
        {/if}
    {:else if !blobPending}
        {#if fileLoadingError}
            <Alert variant="danger">
                Unable to load file data: {fileLoadingError.message}
            </Alert>
        {:else if fileNotFound}
            <div class="circle">
                <Icon svgPath={mdiMapSearch} size={80} />
            </div>
            <h2>File not found</h2>
        {/if}
    {/if}
</div>

<style lang="scss">
    .content {
        display: flex;
        flex-direction: column;
        overflow-x: auto;
        flex: 1;

        &.compare {
            flex-direction: column;
        }

        &.fileNotFound {
            background-color: var(--body-bg);
            flex-direction: column;
            align-items: center;
        }
    }

    .loading {
        filter: blur(1px);
    }

    .rich {
        padding: 1rem;
        overflow: auto;
    }

    .circle {
        background-color: var(--color-bg-2);
        border-radius: 50%;
        padding: 1.5rem;
        margin: 1rem;
    }
</style>
