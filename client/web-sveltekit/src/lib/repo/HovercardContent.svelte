<script lang="ts">
    import type { HoverMerged } from '@sourcegraph/client-api'
    import { asError, renderMarkdown } from '$lib/common'
    import { Badge } from '$lib/wildcard'
    import Tooltip from '$lib/Tooltip.svelte'

    export let content: HoverMerged['contents'][number]
    export let aggregatedBages: HoverMerged['aggregatedBadges']

    function parseContent(content: HoverMerged['contents'][number]): {
        value: string
        error: Error | null
        isMarkdown: boolean
    } {
        if (content.kind === 'markdown') {
            try {
                return {
                    value: renderMarkdown(content.value),
                    error: null,
                    isMarkdown: true,
                }
            } catch (error) {
                return { value: '', error: asError(error), isMarkdown: true }
            }
        }
        return { value: content.value, error: null, isMarkdown: false }
    }

    $: ({ value, error, isMarkdown } = parseContent(content))
</script>

{#if !isMarkdown}
    <pre class="content">{value}</pre>
{:else if error}
    <!-- TOOD: Implement Alert component -->
    <div class="alert alert-danger">{error.message}</div>
{:else}
    {#if aggregatedBages}
        {#each aggregatedBages as { text, linkURL, hoverMessage } (text)}
            <small class="badge">
                <Tooltip tooltip={hoverMessage ?? ''}>
                    <Badge variant="secondary" small>
                        <svelte:fragment slot="custom" let:class={className}>
                            {#if linkURL}
                                <a class={className} href={linkURL} target="_blank" rel="noopener noreferrer">{text}</a>
                            {:else}
                                <span class={className}>{text}</span>
                            {/if}
                        </svelte:fragment>
                    </Badge>
                </Tooltip>
            </small>
        {/each}
    {/if}
    <div class="content">
        {@html value}
    </div>
{/if}

<style lang="scss">
    .badge {
        float: right;
        // Align badge vertically with the close button and first row of the text content.
        margin-top: var(--hover-overlay-content-margin-top);
        margin-left: 0.5rem;
        margin-right: 0.25rem;
        // Small margin-bottom to add some space between the badge and long content that wraps around it.
        margin-bottom: 0.25rem;
        // Needs to be absolute value to align well with the content
        // because it's wrapped into a `small` which might have different font-size.
        line-height: 1rem;
        text-transform: uppercase;
    }

    .content {
        font-size: 0.75rem;
        line-height: (16/12);
        color: var(--hover-overlay-content-color);
        word-wrap: normal;

        > :global(*:first-child) {
            margin-top: var(--hover-overlay-content-margin-top);
            margin-bottom: 0.5rem;
        }

        // Descendant selectors are needed here to style rendered markdown.
        :global(p) {
            margin-top: 0.75rem;
            margin-bottom: 0.75rem;
        }

        :global(pre) {
            margin-top: var(--hover-overlay-content-margin-top);
            margin-bottom: 0.5rem;
            // Required for the correct line-height of the `<code>` element.
            line-height: 1rem;
        }

        :global(code) {
            font-size: 0.75rem;
        }

        :global(pre),
        :global(code) {
            padding: 0;
            // We want code to wrap, not scroll (but whitespace needs to be preserved).
            white-space: pre-wrap;
            // Any other value would create a new block formatting context,
            // which would prevent wrapping around the floating buttons.
            overflow: visible;
        }

        // Table styles (see https://github.com/sourcegraph/sourcegraph/pull/27599)
        :global(td) {
            padding-right: 1rem;
        }
        :global(tbody) {
            :global(tr) {
                border-top: 1px solid var(--border-color);
            }
        }

        :global(a) {
            color: var(--link-color);
        }

        // We use <hr>s as a divider between multiple contents.
        // This has the nice property of having floating buttons that text wraps around.
        :global(hr) {
            // `<p>` and `<pre>` define their own margins, `<hr>` is only concerned with rendering the separator itself.
            margin-top: 0;
            margin-bottom: 0;
            // Enlarge `<hr>` width on the right to span across extra left and right padding.
            margin-left: calc(var(--hover-overlay-horizontal-padding) * -1);
            margin-right: calc(var(--hover-overlay-contents-right-padding) * -1);
            // stylelint-disable-next-line declaration-property-unit-allowed-list
            height: 1px;
            overflow: visible;
            border: none;
            // The <hr> acts like a border, which should always be exactly 1px
            // @quinn keast
            // By using one colour for the border and another for the internal separators,
            // we create better distinction between the popover and its background content, without making it too strongly contrasted within.
            background-color: var(--hover-overlay-separator-color);
        }
    }
</style>
