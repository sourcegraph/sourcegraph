<svelte:options immutable />

<script lang="ts" context="module">
    import hljs from 'highlight.js/lib/core'
    import diff from 'highlight.js/lib/languages/diff'

    import { highlightRanges } from '$lib/dom'

    hljs.registerLanguage('diff', diff)

    const highlightCommit: Action<HTMLElement, { ranges: [number, number][] }> = (node: HTMLElement, { ranges }) => {
        hljs.highlightElement(node)
        highlightRanges(node, { ranges })
    }

    function unwrapMarkdownCodeBlock(content: string): string {
        return content.replace(/^```[_a-z]*\n/i, '').replace(/\n```$/i, '')
    }

    function getMatches(result: CommitMatch): [number, number][] {
        const lines = unwrapMarkdownCodeBlock(result.content).split('\n')

        const lineOffsets: number[] = [0]
        for (let i = 1; i < lines.length; i++) {
            // Convert line to array of codepoints to get correct length
            lineOffsets[i] = lineOffsets[i - 1] + [...lines[i - 1]].length + 1
        }

        return result.ranges.map(([line, start, length]) => [
            lineOffsets[line - 1] + start,
            lineOffsets[line - 1] + start + length,
        ])
    }
</script>

<script lang="ts">
    import type { Action } from 'svelte/action'

    import RepoStars from '$lib/repo/RepoStars.svelte'
    import { type CommitMatch, getMatchUrl } from '$lib/shared'
    import Timestamp from '$lib/Timestamp.svelte'

    import RepoRev from './RepoRev.svelte'
    import SearchResult from './SearchResult.svelte'

    export let result: CommitMatch

    $: commitURL = getMatchUrl(result)
    $: subject = result.message.split('\n', 1)[0]
    $: commitOid = result.oid.slice(0, 7)
    $: content = unwrapMarkdownCodeBlock(result.content)
    $: matches = getMatches(result)
    let highlightCls: string
    $: {
        const lang = /```(\S+)\s/.exec(result.content)?.[1]
        // highlight.js logs a warning if the defined language isn't
        // registers, which is noisy in the console and in tests
        highlightCls = lang?.toLowerCase() === 'diff' ? `language-${lang}` : 'no-highlight'
    }
</script>

<SearchResult>
    <div slot="title" data-sveltekit-preload-data="tap">
        <RepoRev repoName={result.repository} rev={commitOid} />
        <span aria-hidden={true} class="interpunct">·</span>
        <a href={commitURL} data-focusable-search-result>
            {result.authorName}: {subject}
        </a>
    </div>
    <svelte:fragment slot="info">
        <a href={commitURL} data-sveltekit-preload-data="tap">
            <Timestamp date={result.committerDate} strict utc />
        </a>
        {#if result.repoStars}
            <span class="divider" />
            <RepoStars repoStars={result.repoStars} />
        {/if}
    </svelte:fragment>
    <!-- #key is needed here to recreate the element because use:highlightCommit changes the DOM -->
    {#key content}
        <pre class={highlightCls} use:highlightCommit={{ ranges: matches }}>{content}</pre>
    {/key}
</SearchResult>

<style lang="scss">
    .divider {
        border-left: 1px solid var(--border-color);
        padding-left: 0.5rem;
        margin-left: 0.5rem;
    }

    .interpunct {
        margin: 0 0.5rem;
        color: var(--text-muted);
    }

    pre {
        padding: 0.5rem;
        margin: 0;
        font-family: var(--code-font-family);
        font-size: var(--code-font-size);
    }

    [data-focusable-search-result]:focus {
        box-shadow: var(--focus-shadow);
    }
</style>
