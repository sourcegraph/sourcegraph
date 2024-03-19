<script lang="ts">
    import type { Placement } from '@floating-ui/dom'

    import { popover, onClickOutside, portal } from './dom'
    import type { Action } from 'svelte/action'

    export let placement: Placement = 'bottom'
    /**
     * Show the popover when hovering over the trigger.
     */
    export let showOnHover = false

    let isOpen = false
    let trigger: HTMLElement | null
    let popoverContainer: HTMLElement | null

    function toggle(open?: boolean): void {
        isOpen = open === undefined ? !isOpen : open
    }

    function handleClickOutside(event: { detail: HTMLElement }): void {
        if (event.detail !== trigger && !trigger?.contains(event.detail)) {
            isOpen = false
        }
    }

    const registerTrigger: Action<HTMLElement> = node => {
        trigger = node

        function handleMouseEnterTrigger(): void {
            isOpen = true
        }

        function handleMouseLeaveTrigger(event: MouseEvent): void {
            // It should be possible to move the mouse from the trigger to the popover without closing it
            if (event.relatedTarget && !popoverContainer?.contains(event.relatedTarget as Node)) {
                isOpen = false
            }
        }

        if (showOnHover) {
            node.addEventListener('mouseenter', handleMouseEnterTrigger)
            node.addEventListener('mouseleave', handleMouseLeaveTrigger)
        }

        return {
            destroy() {
                trigger = null
                node.removeEventListener('mouseenter', handleMouseEnterTrigger)
                node.removeEventListener('mouseleave', handleMouseLeaveTrigger)
            },
        }
    }

    const registerPopoverContainer: Action<HTMLElement> = node => {
        popoverContainer = node
        function handleMouseLeavePopover(event: MouseEvent): void {
            // It should be possible to move the mouse from the popover to the trigger without closing it
            if (event.relatedTarget && !trigger?.contains(event.relatedTarget as Node)) {
                toggle(false)
            }
        }
        if (showOnHover) {
            node.addEventListener('mouseleave', handleMouseLeavePopover)
        }
        return {
            destroy() {
                popoverContainer = null
                node.removeEventListener('mouseleave', handleMouseLeavePopover)
            },
        }
    }
</script>

<slot {toggle} {registerTrigger} />
{#if trigger && isOpen}
    <div
        use:registerPopoverContainer
        use:portal
        use:popover={{ reference: trigger, options: { placement, shift: { padding: 4 } } }}
        use:onClickOutside
        on:click-outside={handleClickOutside}
    >
        <slot name="content" {toggle} />
    </div>
{/if}

<style lang="scss">
    div {
        position: absolute;
        isolation: isolate;
        min-width: 10rem;
        font-size: 0.875rem;
        background-clip: padding-box;
        background-color: var(--dropdown-bg);
        border: 1px solid var(--dropdown-border-color);
        border-radius: var(--popover-border-radius);
        color: var(--body-color);
        box-shadow: var(--dropdown-shadow);
    }
</style>
