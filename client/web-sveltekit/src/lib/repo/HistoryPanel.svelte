<script lang="ts" context="module">
    export interface Capture {
        scroller?: ScrollerCapture
    }
</script>

<script lang="ts">
    import { mdiClose } from '@mdi/js'
    import { tick } from 'svelte'

    import { page } from '$app/stores'
    import { scrollIntoView } from '$lib/actions'
    import Avatar from '$lib/Avatar.svelte'
    import Icon from '$lib/Icon.svelte'
    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import Scroller, { type Capture as ScrollerCapture } from '$lib/Scroller.svelte'
    import Timestamp from '$lib/Timestamp.svelte'
    import Tooltip from '$lib/Tooltip.svelte'

    import type { HistoryPanel_HistoryConnection } from './HistoryPanel.gql'

    export let history: HistoryPanel_HistoryConnection | null
    export let fetchMore: (afterCursor: string | null) => void
    export let loading: boolean = false
    export let enableInlineDiffs: boolean

    export function capture(): Capture {
        return {
            scroller: scroller?.capture(),
        }
    }

    export async function restore(data: Capture) {
        if (data.scroller) {
            // Wait until DOM was update before updating the scroll position
            await tick()
            // restore might be called when the history panel is closed
            // in which case scroller doesn't exist
            scroller?.restore(data.scroller)
        }
    }

    function getClearURL(): string {
        const url = new URL($page.url ?? window.location)
        url.searchParams.delete('rev')
        return url.href
    }

    function loadMore() {
        if (history?.pageInfo.hasNextPage) {
            fetchMore(history.pageInfo.endCursor)
        }
    }

    let scroller: Scroller

    // If the selected revision is not in the set of currently loaded commits, load more
    $: if (
        selectedRev &&
        history &&
        history.nodes.length > 0 &&
        !history.nodes.some(commit => commit.abbreviatedOID === selectedRev)
    ) {
        loadMore()
    }

    $: selectedRev = $page.url?.searchParams.get('rev')
    $: clearURL = getClearURL()
</script>

<Scroller bind:this={scroller} margin={200} on:more={loadMore}>
    {#if history}
        <table>
            {#each history.nodes as commit (commit.id)}
                {@const selected = commit.abbreviatedOID === selectedRev}
                <tr class:selected use:scrollIntoView={selected}>
                    <td>
                        <Avatar avatar={commit.author.person} />&nbsp;
                        {commit.author.person.displayName}
                    </td>
                    <td class="subject">
                        {#if enableInlineDiffs}
                            <a href="?rev={commit.abbreviatedOID}">{commit.subject}</a>
                        {:else}
                            {commit.subject}
                        {/if}
                    </td>
                    <td><Timestamp date={new Date(commit.author.date)} strict /></td>
                    <td><a href={commit.canonicalURL}>{commit.abbreviatedOID}</a></td>
                    <td>
                        {#if selected}
                            <Tooltip tooltip="Hide comparison">
                                <a href={clearURL}><Icon svgPath={mdiClose} inline /></a>
                            </Tooltip>
                        {/if}
                    </td>
                </tr>
            {/each}
        </table>
    {/if}
    {#if !history || loading}
        <LoadingSpinner />
    {/if}
</Scroller>

<style lang="scss">
    table {
        width: 100%;
        max-width: 100%;
    }

    td {
        padding: 0.25rem;
        white-space: nowrap;

        &.subject {
            white-space: normal;
        }
    }

    tr {
        border-bottom: 1px solid var(--border-color);

        &.selected {
            background-color: var(--color-bg-2);
        }
    }
</style>
