<script lang="ts">
    import { mdiChevronLeft, mdiChevronRight } from '@mdi/js'

    import { pluralize } from '$lib/common'
    import Icon from '$lib/Icon.svelte'
    import { Button } from '$lib/wildcard'

    import type { HighlightedRange } from './static-highlights'

    export let ranges: HighlightedRange[]
    export let handlePrevious: () => void
    export let handleNext: () => void

    $: totalMatches = ranges.length
    $: selectedIdx = ranges.findIndex(range => range.selected)
</script>

<div class="root">
    {#if totalMatches > 1}
        <div class="buttons">
            <div class="left">
                <Button
                    size="sm"
                    outline
                    variant="secondary"
                    on:click={handlePrevious}
                    data-testid="blob-view-static-previous"
                    aria-label="previous result"
                >
                    <Icon inline svgPath={mdiChevronLeft} aria-hidden={true} />
                </Button>
            </div>
            <div class="right">
                <Button
                    size="sm"
                    outline
                    variant="secondary"
                    on:click={handleNext}
                    data-testid="blob-view-static-next"
                    aria-label="next result"
                >
                    <Icon inline svgPath={mdiChevronRight} aria-hidden={true} />
                </Button>
            </div>
        </div>
    {/if}
    <div>
        {selectedIdx === -1 ? '' : `${selectedIdx + 1} of `}
        {totalMatches}
        {pluralize('result', totalMatches)}
    </div>
</div>

<style lang="scss">
    .root {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        padding: 0.5rem;
        color: var(--text-muted);

        background-color: var(--color-bg);

        .buttons {
            display: flex;
            .left :global(button) {
                padding: 0.25rem;
                border-top-right-radius: 0;
                border-bottom-right-radius: 0;
            }
            .right :global(button) {
                padding: 0.25rem;
                border-top-left-radius: 0;
                border-bottom-left-radius: 0;
                border-left: none;
            }
        }
    }
</style>
