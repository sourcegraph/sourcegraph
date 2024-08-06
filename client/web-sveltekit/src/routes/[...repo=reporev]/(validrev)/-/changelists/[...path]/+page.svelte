<script lang="ts">
    // @sg EnableRollout
    import { get } from 'svelte/store'

    import { navigating } from '$app/stores'
    import Changelist from '$lib/Changelist.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import RepositoryRevPicker from '$lib/repo/RepositoryRevPicker.svelte'
    import Scroller, { type Capture as ScrollerCapture } from '$lib/Scroller.svelte'
    import { Alert, Badge } from '$lib/wildcard'

    import type { PageData, Snapshot } from './$types'

    export let data: PageData

    // This tracks the number of changelists that have been loaded and the current scroll
    // position, so both can be restored when the user refreshes the page or navigates
    // back to it.
    export const snapshot: Snapshot<{
        changelists: ReturnType<typeof data.changelistsQuery.capture>
        scroller: ScrollerCapture
    }> = {
        capture() {
            return {
                changelists: changelistsQuery.capture(),
                scroller: scroller.capture(),
            }
        },
        async restore(snapshot) {
            if (get(navigating)?.type === 'popstate') {
                await changelistsQuery?.restore(snapshot.changelists)
            }
            scroller.restore(snapshot.scroller)
        },
    }

    function fetchMore() {
        changelistsQuery?.fetchMore()
    }

    let scroller: Scroller

    $: changelistsQuery = data.changelistsQuery
    $: changelists = $changelistsQuery.data
    $: pageTitle = (() => {
        const parts = ['Changelists']
        if (data.path) {
            parts.push(data.path)
        }
        parts.push(data.displayRepoName, 'Sourcegraph')
        return parts.join(' - ')
    })()
</script>

<svelte:head>
    <title>{pageTitle}</title>
</svelte:head>

<header>
    <h2>
        Changelists
        {#if data.path}
            in <code>{data.path}</code>
        {/if}
    </h2>
    <div>
        <RepositoryRevPicker
            repoURL={data.repoURL}
            revision={data.revision}
            commitID={data.resolvedRevision.commitID}
            defaultBranch={data.defaultBranch}
            placement="bottom-start"
            getDepotChangelists={data.getDepotChangelists}
        />
    </div>
</header>
<section>
    <Scroller bind:this={scroller} margin={600} on:more={fetchMore}>
        {#if changelists}
            <ul class="changelists">
                {#each changelists as changelist (changelist.perforceChangelist?.canonicalURL)}
                    <li>
                        <div class="changelist">
                            <Changelist {changelist} />
                        </div>
                        <ul class="actions">
                            <li>
                                Changelist ID:
                                <Badge variant="link">
                                    <a href={changelist.perforceChangelist?.canonicalURL} title="View changelist"
                                        >{changelist.perforceChangelist?.cid}</a
                                    >
                                </Badge>
                            </li>
                            <li>
                                <a href="/{data.repoName}@changelist/{changelist.perforceChangelist?.cid}"
                                    >Browse files</a
                                ></li
                            >
                        </ul>
                    </li>
                {:else}
                    <li>
                        <Alert variant="info">No changelists found</Alert>
                    </li>
                {/each}
            </ul>
        {/if}
        {#if $changelistsQuery.fetching}
            <div class="footer">
                <LoadingSpinner />
            </div>
        {:else if !$changelistsQuery.fetching && $changelistsQuery.error}
            <div class="footer">
                <Alert variant="danger">
                    Unable to fetch changelists: {$changelistsQuery.error.message}
                </Alert>
            </div>
        {/if}
    </Scroller>
</section>

<style lang="scss">
    section {
        overflow: hidden;
    }

    ul {
        margin: 0;
        padding: 0;
    }

    h2 {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    header {
        border-bottom: 1px solid var(--border-color);
        div {
            width: min-content;
        }
    }

    header,
    ul.changelists,
    .footer {
        max-width: var(--viewport-xl);
        width: 100%;
        margin: 0 auto;
        padding: 1rem;

        @media (--mobile) {
            padding: 0.5rem;
        }
    }

    ul {
        list-style: none;
    }

    ul.changelists {
        --avatar-size: 2.5rem;
        padding-top: 0;

        > li {
            border-bottom: 1px solid var(--border-color);
            display: flex;
            padding: 0.5rem 0;
            gap: 1rem;

            @media (--mobile) {
                display: block;

                .actions {
                    display: flex;
                    gap: 0.5rem;
                    margin-top: 0.5rem;

                    li:not(:last-child)::after {
                        content: '•';
                        padding-left: 0.5rem;
                        color: var(--text-muted);
                    }
                }
            }

            .changelist {
                flex: 1;
                min-width: 0;
            }

            .actions {
                --icon-color: currentColor;
                text-align: right;

                flex-shrink: 0;
            }

            &:last-child {
                border: none;
            }
        }
    }

    .footer {
        &:not(:first-child) {
            border-top: 1px solid var(--border-color);
        }
    }
</style>
