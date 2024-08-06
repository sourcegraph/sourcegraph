<script lang="ts">
    import { get } from 'svelte/store'

    import { pluralize } from '@sourcegraph/common'

    import { afterNavigate, beforeNavigate } from '$app/navigation'
    import { navigating } from '$app/stores'
    import Changelist from '$lib/Changelist.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import FileDiff from '$lib/repo/FileDiff.svelte'
    import Scroller, { type Capture as ScrollerCapture } from '$lib/Scroller.svelte'
    import { isViewportMobile } from '$lib/stores'
    import Alert from '$lib/wildcard/Alert.svelte'
    import Badge from '$lib/wildcard/Badge.svelte'
    import CopyButton from '$lib/wildcard/CopyButton.svelte'

    import { getRepositoryPageContext } from '../../../context'

    import type { PageData, Snapshot } from './$types'

    interface Capture {
        scroll: ScrollerCapture
        diffs?: ReturnType<NonNullable<typeof data.diff>['capture']>
        expandedDiffs: Array<[number, boolean]>
    }

    export let data: PageData

    export const snapshot: Snapshot<Capture> = {
        capture: () => ({
            scroll: scroller.capture(),
            diffs: diffQuery?.capture(),
            expandedDiffs: expandedDiffsSnapshot,
        }),
        restore: async capture => {
            expandedDiffs = new Map(capture.expandedDiffs)
            if (get(navigating)?.type === 'popstate') {
                await data.diff?.restore(capture.diffs)
            }
            scroller.restore(capture.scroll)
        },
    }

    const repositoryContext = getRepositoryPageContext()
    let scroller: Scroller
    let expandedDiffs = new Map<number, boolean>()
    let expandedDiffsSnapshot: Array<[number, boolean]> = []

    $: diffQuery = data.diff
    $: diffs = $diffQuery?.data
    $: cid = data.changelist.cid

    afterNavigate(() => {
        repositoryContext.set({ revision: data.changelist.commit.oid })
    })
    beforeNavigate(() => {
        expandedDiffsSnapshot = Array.from(expandedDiffs.entries())
        expandedDiffs = new Map()

        repositoryContext.set({})
    })
</script>

<svelte:head>
    <title>Changelist: {data.changelist.commit.message ?? ''} - {data.displayRepoName} - Sourcegraph</title>
</svelte:head>

<section>
    {#if data.changelist}
        <Scroller bind:this={scroller} margin={600} on:more={diffQuery?.fetchMore}>
            <div class="header">
                <div class="info">
                    <Changelist changelist={data.changelist.commit} alwaysExpanded={!$isViewportMobile} />
                </div>
                <ul class="actions">
                    <li>
                        <span>Changelist:</span>
                        <Badge variant="secondary"><code>{cid}</code></Badge>&nbsp;<CopyButton value={cid} />
                    </li>
                    <li>
                        <span>{pluralize('Parent', data.changelist.commit.parents.length)}:</span>
                        {#each data.changelist.commit.parents as parent}
                            <Badge variant="link"><a href={parent.parent?.canonicalURL}>{parent.parent?.cid}</a></Badge
                            >&nbsp;<CopyButton value={parent.parent?.cid ?? ''} />{' '}
                        {/each}
                    </li>
                    <li>
                        <a href="/{data.repoName}@changelist/{cid}"
                            >Browse files at <Badge variant="link">{cid}</Badge></a
                        >
                    </li>
                </ul>
            </div>
            <hr />
            {#if diffs}
                <ul class="diffs">
                    {#each diffs as node, index (index)}
                        <li>
                            <FileDiff
                                fileDiff={node}
                                expanded={expandedDiffs.get(index)}
                                on:toggle={event => {
                                    expandedDiffs.set(index, event.detail.expanded)
                                    // This is needed to for Svelte to consider that expandedDiffs has changed
                                    expandedDiffs = expandedDiffs
                                }}
                            />
                        </li>
                    {/each}
                </ul>
            {/if}
            {#if $diffQuery?.fetching}
                <LoadingSpinner />
            {:else if $diffQuery?.error}
                <div class="error">
                    <Alert variant="danger">
                        Unable to fetch file diffs: {$diffQuery.error.message}
                    </Alert>
                </div>
            {/if}
        </Scroller>
    {/if}
</section>

<style lang="scss">
    section {
        overflow: auto;
        isolation: isolate;
    }

    .header {
        display: flex;
        margin: 1rem;

        @media (--mobile) {
            flex-direction: column;
            margin: 1rem 0.5rem;
        }
    }

    ul.actions {
        --icon-color: currentColor;
        all: unset;
        list-style: none;

        span,
        a {
            vertical-align: middle;
        }

        @media (--mobile) {
            display: flex;
            flex-flow: row wrap;
            gap: 0.5rem;
            margin-top: 1rem;

            li:not(:first-child)::before {
                content: '•';
                padding-right: 0.5rem;
                color: var(--text-muted);
            }
        }
    }

    .info {
        flex: 1;
        --avatar-size: 2.5rem;
        // This seems necessary to ensure that the commit message is
        // overlaying the sticky file diff headers on mobile.
        z-index: 1;
    }

    code {
        font-family: monospace;
        font-size: inherit;
    }

    .error,
    ul.diffs {
        margin: 1rem;

        @media (--mobile) {
            margin: 0rem;
        }
    }

    ul.diffs {
        padding: 0;
        list-style: none;

        li + li {
            margin-top: 1rem;

            @media (--mobile) {
                margin-top: 0;
                border-top: 1px solid var(--border-color);
            }
        }
    }
</style>
