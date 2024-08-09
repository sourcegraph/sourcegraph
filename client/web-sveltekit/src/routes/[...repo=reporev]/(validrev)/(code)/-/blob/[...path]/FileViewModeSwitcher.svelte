<script lang="ts" generics="T extends string">
    import { createEventDispatcher } from 'svelte'
    import type { AriaAttributes } from 'svelte/elements'

    import { nextSibling, previousSibling, uniqueID } from '$lib/dom'

    type $$Props = {
        value: T
        options: T[]
    } & AriaAttributes

    export let value: T
    export let options: T[]

    const id = uniqueID()
    const dispatch = createEventDispatcher<{ change: T; preload: T }>()

    // Keyboard interaction was modeled after https://www.w3.org/WAI/ARIA/apg/patterns/radio

    function handleMousedown(event: Event) {
        const target = event.currentTarget as HTMLElement
        value = target.dataset.value as T
        dispatch('change', value)
    }

    function handlePreload(event: Event) {
        const target = event.currentTarget as HTMLElement
        dispatch('preload', target.dataset.value as T)
    }

    function handleKeydown(event: KeyboardEvent) {
        const target = event.currentTarget as HTMLElement
        let newTarget: Element | null = null

        switch (event.key) {
            case 'ArrowUp':
            case 'ArrowLeft':
                newTarget = previousSibling(target, '[role="radio"]', true)
                break
            case 'ArrowDown':
            case 'ArrowRight':
                newTarget = nextSibling(target, '[role="radio"]', true)
                break
            case ' ':
            case 'Enter':
                newTarget = target
                break
        }

        if (newTarget) {
            ;(newTarget as HTMLElement).focus()
            value = (newTarget as HTMLElement).dataset.value as T
            dispatch('change', value)
        }
    }
</script>

<div role="radiogroup" {...$$restProps} tabindex="-1">
    {#each options as option}
        {@const checked = option === value}
        <span
            role="radio"
            aria-checked={checked}
            aria-labelledby="{id}-{option}-label"
            tabindex={checked ? 0 : -1}
            data-value={option}
            on:mousedown={handleMousedown}
            on:keydown={handleKeydown}
            on:mouseover={handlePreload}
            on:focus={handlePreload}
        >
            <span id="{id}-{option}-label">
                <slot name="label" value={option}>{option}</slot>
            </span>
        </span>
    {/each}
</div>

<style lang="scss">
    [role='radiogroup'] {
        --border-radius: 0.5rem;
        --border-width: 1px;

        display: inline-flex;
        background-color: var(--secondary-4);
        border-radius: var(--border-radius);
        align-items: center;
        border: var(--border-width) solid var(--border-color);
    }

    [role='radio'] {
        border: var(--border-width) solid transparent;
        cursor: pointer;
        padding: 0.25rem 0.75rem;
        border-radius: var(--border-radius);
        color: var(--text-body);
        margin: calc(var(--border-width) * -1);
        user-select: none;

        &:focus-visible {
            box-shadow: var(--focus-shadow-inset);
        }
    }

    [aria-checked='true'] {
        border-color: var(--border-color-2);
        background-color: var(--color-bg-1);
        color: var(--body-color);
    }
</style>
