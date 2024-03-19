<script lang="ts">
    import { Story } from '@storybook/addon-svelte-csf'
    import { graphql } from 'msw'
    import { SvelteComponent, setContext } from 'svelte'
    import { readable } from 'svelte/store'

    import type { HighlightedFileResult, HighlightedFileVariables } from '$lib/graphql-operations'
    import { queryStateStore } from '$lib/search/state'
    import type { ContentMatch, PathMatch, SearchMatch, SymbolMatch } from '$lib/shared'
    import { KEY, type SourcegraphContext } from '$lib/stores'
    import { createTemporarySettingsStorage } from '$lib/temporarySettings'
    import {
        createCommitMatch,
        createContentMatch,
        createPathMatch,
        createPersonMatch,
        createSymbolMatch,
        createTeamMatch,
    } from '$testing/search-testdata'
    import { createHighlightedFileResult } from '$testing/testdata'

    import CommitSearchResult from './CommitSearchResult.svelte'
    import FileContentSearchResult from './FileContentSearchResult.svelte'
    import FilePathSearchResult from './FilePathSearchResult.svelte'
    import PersonSearchResult from './PersonSearchResult.svelte'
    import SearchResults from './SearchResults.svelte'
    import { setSearchResultsContext } from './searchResultsContext'
    import SymbolSearchResult from './SymbolSearchResult.svelte'
    import TeamSearchResult from './TeamSearchResult.svelte'

    export const meta = {
        title: 'search/SearchResults',
        component: SearchResults,
        parameters: {
            msw: {
                handlers: {
                    highlightedFile: graphql.query<HighlightedFileResult, HighlightedFileVariables>(
                        'HighlightedFile',
                        (req, res, ctx) => res(ctx.data(createHighlightedFileResult(req.variables.ranges)))
                    ),
                },
            },
        },
    }

    setContext<SourcegraphContext>(KEY, {
        user: readable(null),
        settings: readable({}),
        featureFlags: readable([]),
        temporarySettingsStorage: createTemporarySettingsStorage(),
    })

    setSearchResultsContext({
        isExpanded(_match) {
            return false
        },
        setExpanded(_match, _expanded) {},
        queryState: queryStateStore(undefined, {}),
        setPreview(_props: PathMatch | ContentMatch | SymbolMatch | null): void {},
    })
    // TS complains about up MockSuitFunctions which is not relevant here
    // @ts-ignore
    window.context = { xhrHeaders: {} }

    const results: [string, typeof SvelteComponent<{ result: SearchMatch }>, () => SearchMatch][] = [
        ['Path match', FilePathSearchResult, createPathMatch],
        ['Content match', FileContentSearchResult, createContentMatch],
        ['Commit match', CommitSearchResult, () => createCommitMatch('commit')],
        ['Commit match (diff)', CommitSearchResult, () => createCommitMatch('diff')],
        ['Symbol match', SymbolSearchResult, createSymbolMatch],
        ['Person match', PersonSearchResult, createPersonMatch],
        ['Team match', TeamSearchResult, createTeamMatch],
    ]

    const data = results.map(([, , generator]) => generator())

    function randomizeData(i: number) {
        data[i] = results[i][2]()
    }
</script>

<Story name="Default">
    {#each results as [title, component], i}
        <div>
            <h2>{title}</h2>
            <button on:click={() => randomizeData(i)}>Randomize</button>
        </div>
        <svelte:component this={component} result={data[i]} />
    {/each}
</Story>

<style lang="scss">
    div {
        display: flex;
        align-items: center;
        justify-content: space-between;
    }

    h2 {
        margin: 1rem 0;
    }
</style>
