<script lang="ts" context="module">
    import { createContextAccessors } from '$lib/utils/context'
    type DropdownMenu = ReturnType<typeof createDropdownMenu>

    interface DropdownMenuContext {
        item: DropdownMenu['elements']['item']
        trigger: DropdownMenu['elements']['trigger']
        separator: DropdownMenu['elements']['separator']
        open: DropdownMenu['states']['open']
        builders: DropdownMenu['builders']
    }

    const [setContext, getContext] = createContextAccessors<DropdownMenuContext>()

    export { getContext }
</script>

<script lang="ts">
    import { createDropdownMenu } from '@melt-ui/svelte'
    import type { HTMLButtonAttributes } from 'svelte/elements'
    import type { Writable } from 'svelte/store'

    interface $$Props extends HTMLButtonAttributes {
        open: Writable<boolean>
        triggerButtonClass: string
    }

    export let open: Writable<boolean>
    export let triggerButtonClass: string

    const {
        elements: { menu, item, trigger, separator },
        builders,
    } = createDropdownMenu({
        open,
    })
    setContext({ item, trigger, separator, builders, $open })
</script>

<button {...$trigger} use:trigger class={triggerButtonClass} {...$$restProps}>
    <slot name="trigger" />
</button>

<div {...$menu} use:menu>
    <slot />
</div>

<style lang="scss">
    div,
    div :global([role='menu']) {
        isolation: isolate;
        z-index: 1000;
        min-width: 12rem;
        font-size: 0.875rem;
        background-clip: padding-box;
        background-color: var(--dropdown-bg);
        border: 1px solid var(--dropdown-border-color);
        border-radius: var(--popover-border-radius);
        color: var(--body-color);
        box-shadow: var(--dropdown-shadow);
        padding: 0.25rem 0;

        :global([role^='menuitem']) {
            cursor: pointer;
            display: block;
            width: 100%;
            padding: var(--dropdown-item-padding);
            white-space: nowrap;
            color: var(--dropdown-link-hover-color);

            &:hover,
            &:focus {
                background-color: var(--dropdown-link-hover-bg);
                color: var(--dropdown-link-hover-color);
                text-decoration: none;
            }
        }
    }
</style>
