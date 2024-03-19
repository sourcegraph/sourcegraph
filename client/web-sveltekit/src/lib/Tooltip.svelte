<script lang="ts" context="module">
    import type { Placement } from '@floating-ui/dom'

    export type { Placement }
</script>

<script lang="ts">
    import { popover, portal, uniqueID } from './dom'

    /**
     * The content of the tooltip.
     */
    export let tooltip: string
    /**
     * On which side to show the tooltip by default.
     */
    export let placement: Placement = 'bottom'
    /**
     * Force the tooltip to be always visible
     * (only used for stories).
     */
    export let alwaysVisible = false

    const id = uniqueID('tooltip')

    let visible = false
    let wrapper: HTMLElement
    let target: Element | null

    function show() {
        visible = true
    }

    function hide() {
        visible = false
    }

    $: options = {
        placement,
        offset: 8,
        shift: {
            padding: 4,
        },
    }
    $: target = wrapper?.firstElementChild
    $: if (target && tooltip) {
        target.setAttribute('aria-label', tooltip)
    }
</script>

<!-- TODO: close tooltip on escape -->
<!--
    These event handlers listen for bubbled events from the trigger. The element
    itself is not interactable.
    svelte-ignore a11y-no-static-element-interactions
-->
<div class="wrapper" bind:this={wrapper} on:mouseenter={show} on:mouseleave={hide} on:focusin={show} on:focusout={hide}>
    <slot />
</div>
{#if (alwaysVisible || visible) && target && tooltip}
    <div role="tooltip" {id} use:popover={{ reference: target, options }} use:portal>
        <div class="content">{tooltip}</div>
        <div data-arrow />
    </div>
{/if}

<style lang="scss">
    .wrapper {
        display: contents;
    }

    [role='tooltip'] {
        --tooltip-font-size: 0.75rem; // 12px
        --tooltip-line-height: 1.02rem; // 16.32px / 16px, per Figma
        --tooltip-max-width: 256px;
        --tooltip-color: var(--light-text);
        --tooltip-border-radius: var(--border-radius);
        --tooltip-padding-y: 0.25rem;
        --tooltip-padding-x: 0.5rem;
        --tooltip-margin: 0;

        --tooltip-arrow-side: 8px solid transparent;
        --tooltip-arrow-main: 8px solid var(--tooltip-bg);

        all: initial;
        position: absolute;
        isolation: isolate;
        font-family: inherit;
        font-size: var(--tooltip-font-size);
        font-style: normal;
        font-weight: normal;
        line-height: var(--tooltip-line-height);
        max-width: var(--tooltip-max-width);
        color: var(--tooltip-color);
        user-select: text;
        word-wrap: break-word;
        border: none;
        min-width: 0;
        width: max-content;

        .content {
            background-color: var(--tooltip-bg);
            padding: var(--tooltip-padding-y) var(--tooltip-padding-x);
            border-radius: var(--tooltip-border-radius);
        }

        :global([data-arrow][data-placement^='top']) {
            bottom: -4px;
        }

        :global([data-arrow][data-placement^='bottom']) {
            top: -4px;
        }

        :global([data-arrow][data-placement^='left']) {
            right: -4px;
        }

        :global([data-arrow][data-placement^='right']) {
            left: -4px;
        }
    }

    [data-arrow] {
        position: absolute;
        width: 8px;
        height: 8px;
        transform: rotate(45deg);
        background-color: var(--tooltip-bg);
    }
</style>
