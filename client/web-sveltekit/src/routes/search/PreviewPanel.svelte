<script lang="ts" context="module">
    export interface Range {
        start: Location
        end: Location
    }

    export interface Location {
        // A zero-based line number
        line: number
        // A zero-based column number
        column: number
    }
</script>

<script lang="ts">
    import { mdiClose } from '@mdi/js'
    import { from } from 'rxjs'

    import CodeMirrorBlob from '$lib/CodeMirrorBlob.svelte'
    import { isErrorLike } from '$lib/common'
    import { getGraphQLClient, mapOrThrow, toGraphQLResult } from '$lib/graphql'
    import Icon from '$lib/Icon.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import {
        getFileMatchUrl,
        createCodeIntelAPI,
        type ContentMatch,
        type PathMatch,
        type SymbolMatch,
    } from '$lib/shared'
    import { settings } from '$lib/stores'
    import { toReadable } from '$lib/utils/promises'
    import { Alert } from '$lib/wildcard'

    // We are reusing the queries from the blob page to ensure we take advantage of
    // the cache. It may make sense to pass loaders into this function
    // rather than adding a dependency on the blob page.
    import {
        BlobPageQuery,
        BlobSyntaxHighlightQuery,
    } from '../[...repo=reporev]/(validrev)/(code)/-/blob/[...path]/page.gql'

    import { getSearchResultsContext } from './searchResultsContext'

    export let result: ContentMatch | SymbolMatch | PathMatch

    const searchResultContext = getSearchResultsContext()

    const client = getGraphQLClient()
    $: codeIntelAPI = createCodeIntelAPI({
        settings: setting => (isErrorLike($settings?.final) ? undefined : $settings?.final?.[setting]),
        requestGraphQL(options) {
            return from(client.query(options.request, options.variables).then(toGraphQLResult))
        },
    })

    $: blobStore = toReadable(
        client
            .query(BlobPageQuery, {
                repoName: result.repository,
                revspec: result.commit ?? '',
                path: result.path,
            })
            .then(mapOrThrow(result => result.data?.repository?.commit?.blob ?? null))
    )

    $: highlightStore = toReadable(
        client
            .query(BlobSyntaxHighlightQuery, {
                repoName: result.repository,
                revspec: result.commit ?? '',
                path: result.path,
                disableTimeout: false,
            })
            .then(mapOrThrow(result => result.data?.repository?.commit?.blob?.highlight.lsif ?? ''))
    )
</script>

<div class="preview-container">
    <div class="header">
        <h3>File Preview</h3>
        <button data-testid="preview-close" on:click={() => searchResultContext.setPreview(null)}>
            <Icon svgPath={mdiClose} class="close-icon" size={16} inline />
        </button>
    </div>
    <div class="file-link">
        <small>
            <a href={getFileMatchUrl(result)}>{result.path}</a>
        </small>
    </div>
    <div class="content">
        {#if $blobStore.pending}
            <LoadingSpinner />
        {:else if $blobStore.error}
            <Alert variant="danger">
                Unable to load file data: {$blobStore.error}
            </Alert>
        {:else if $blobStore.value}
            <CodeMirrorBlob
                blobInfo={{
                    repoName: result.repository,
                    commitID: result.commit ?? '',
                    revision: '',
                    filePath: result.path,
                    content: $blobStore.value.content ?? '',
                    languages: $blobStore.value.languages ?? [],
                }}
                highlights={$highlightStore.value ?? ''}
                {codeIntelAPI}
            />
        {/if}
    </div>
</div>

<style lang="scss">
    .preview-container {
        display: flex;
        flex-direction: column;
        height: 100%;
    }

    .header {
        // Keep in sync with the action bar
        height: 3rem;
        flex: none;

        display: flex;
        align-items: center;
        padding: 0 0.5rem;
        border-bottom: 1px solid var(--border-color);

        h3 {
            margin: 0;
            margin-right: auto;
            height: fit-content;
        }

        button {
            background-color: transparent;
            color: var(--text-muted);
            border: none;
            &:hover {
                color: var(--body-color);
            }
        }
    }

    .file-link {
        padding: 0.25rem 0.5rem;
        border-bottom: 1px solid var(--border-color);
    }

    .content {
        overflow: auto;
        background-color: var(--code-bg);
    }
</style>
