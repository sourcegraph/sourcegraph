<script lang="ts">
    import {
        mdiAlertCircle,
        mdiAlert,
        mdiChevronDown,
        mdiChevronLeft,
        mdiInformationOutline,
        mdiMagnify,
    } from '@mdi/js'

    import { limitHit, sortBySeverity } from '$lib/branded'
    import { renderMarkdown, pluralize } from '$lib/common'
    import Icon from '$lib/Icon.svelte'
    import Popover from '$lib/Popover.svelte'
    import ResultsIndicator from '$lib/search/resultsIndicator/ResultsIndicator.svelte'
    import SyntaxHighlightedQuery from '$lib/search/SyntaxHighlightedQuery.svelte'
    import type { Progress, Skipped } from '$lib/shared'
    import { Button } from '$lib/wildcard'

    export let progress: Progress
    export let state: 'complete' | 'error' | 'loading'

    const icons: Record<string, string> = {
        info: mdiInformationOutline,
        warning: mdiAlert,
        error: mdiAlertCircle,
    }
    let searchAgainDisabled = true

    function updateButton(event: Event) {
        const element = event.target as HTMLInputElement
        searchAgainDisabled = Array.from(element.form?.querySelectorAll('[name="query"]') ?? []).every(
            input => !(input as HTMLInputElement).checked
        )
    }

    $: hasSkippedItems = progress.skipped.length > 0
    $: sortedItems = sortBySeverity(progress.skipped)
    $: openItems = sortedItems.map((_, index) => index === 0)
    $: suggestedItems = sortedItems.filter((skipped): skipped is Required<Skipped> => !!skipped.suggested)
    $: hasSuggestedItems = suggestedItems.length > 0
    $: severity = progress.skipped.some(skipped => skipped.severity === 'warn' || skipped.severity === 'error')
        ? 'error'
        : 'info'
    $: isError = severity === 'error' || state === 'error'
</script>

<Popover let:registerTrigger let:toggle placement="bottom-start">
    <Button variant={isError ? 'danger' : 'secondary'} size="sm" outline>
        <svelte:fragment slot="custom" let:buttonClass>
            <button use:registerTrigger class="{buttonClass} progress-button" on:click={() => toggle()}>
                <ResultsIndicator {state} {suggestedItems} {progress} {severity} />
            </button>
        </svelte:fragment>
    </Button>
    <div slot="content" class="streaming-popover">
        <p>
            Found {limitHit(progress) ? 'more than ' : ''}
            {progress.matchCount}
            {pluralize('result', progress.matchCount)}
            {#if progress.repositoriesCount !== undefined}
                from {progress.repositoriesCount} {pluralize('repository', progress.repositoriesCount, 'repositories')}.
            {/if}
        </p>
        {#if hasSkippedItems}
            <h3>Some results skipped</h3>
            {#each sortedItems as item, index (item.reason)}
                {@const open = openItems[index]}
                <Button variant="primary" outline>
                    <svelte:fragment slot="custom" let:buttonClass>
                        <button
                            type="button"
                            class="{buttonClass} p-2 w-100 bg-transparent border-0"
                            aria-expanded={open}
                            on:click={() => (openItems[index] = !open)}
                        >
                            <h4 class="d-flex align-items-center mb-0 w-100">
                                <span class="mr-1 flex-shrink-0"><Icon svgPath={icons[item.severity]} inline /></span>
                                <span class="flex-grow-1 text-left">{item.title}</span>
                                {#if item.message}
                                    <span class="chevron flex-shrink-0"
                                        ><Icon svgPath={open ? mdiChevronDown : mdiChevronLeft} inline /></span
                                    >
                                {/if}
                            </h4>
                        </button>
                    </svelte:fragment>
                </Button>
                {#if item.message && open}
                    <div class="message">
                        {@html renderMarkdown(item.message)}
                    </div>
                {/if}
            {/each}
        {/if}
        {#if hasSuggestedItems}
            <p>Search again:</p>
            <form on:submit|preventDefault>
                {#each suggestedItems as item (item.suggested.queryExpression)}
                    <label>
                        <input
                            type="checkbox"
                            name="query"
                            value={item.suggested.queryExpression}
                            on:click={updateButton}
                        />
                        {item.suggested.title} (
                        <SyntaxHighlightedQuery query={item.suggested.queryExpression} />
                        )
                    </label>
                {/each}
                <Button variant="primary">
                    <svelte:fragment slot="custom" let:buttonClass>
                        <button class="{buttonClass} mt-3" disabled={searchAgainDisabled}>
                            <Icon svgPath={mdiMagnify} />
                            <span>Search again</span>
                        </button>
                    </svelte:fragment>
                </Button>
            </form>
        {/if}
    </div>
</Popover>

<style lang="scss">
    .chevron > :global(svg) {
        fill: currentColor !important;
    }

    label {
        display: block;
    }

    .message {
        border-left: 2px solid var(--primary);
        padding-left: 0.5rem;
        margin: 0 1rem 1rem 1rem;
    }

    .progress-button {
        border: 1px solid var(--border-color-2);
        border-radius: 4px;
        margin-left: 0.3rem;
    }

    .streaming-popover {
        width: 24rem;

        p,
        h3,
        form {
            margin: 1rem;
        }
    }
</style>
