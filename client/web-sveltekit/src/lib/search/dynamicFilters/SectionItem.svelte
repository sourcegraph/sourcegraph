<script lang="ts">
    import { createEventDispatcher, type ComponentProps } from 'svelte'

    import Icon from '$lib/Icon.svelte'

    import CountBadge from './CountBadge.svelte'

    export let label: string
    export let value: string
    export let href: URL
    export let count: ComponentProps<CountBadge> | undefined = undefined
    export let selected: boolean

    const dispatch = createEventDispatcher<{ select: { label: string; value: string } }>()
</script>

<!-- TODO: a11y. This should expose the aria selected state and use the proper roles -->
<a href={href.toString()} class:selected on:click={() => dispatch('select', { label, value })}>
    <slot name="icon" />
    <span class="label">
        <slot name="label" {label} {value}>
            {label}
        </slot>
    </span>
    {#if count}
        <CountBadge {...count} />
    {/if}
    {#if selected}
        <Icon icon={ILucideX} inline aria-hidden />
    {/if}
</a>

<style lang="scss">
    a {
        --icon-color: currentColor;

        display: flex;
        width: 100%;
        align-items: center;
        border: none;
        text-align: left;
        text-decoration: none;
        border-radius: var(--border-radius);
        color: inherit;
        white-space: nowrap;
        gap: 0.5rem;

        padding: 0.25rem 0.5rem;
        margin: 0;
        font-weight: 400;

        .label {
            flex: 1;
            text-overflow: ellipsis;
            overflow: hidden;
            color: var(--text-body);
        }

        &:hover {
            background-color: var(--color-bg-3);

            .label {
                color: var(--text-title);
            }
        }

        &.selected {
            // Explicitly override icon color to ensure that icons with custom colors
            // are visible on the primary background
            --file-icon-color: currentColor;

            background-color: var(--primary);
            color: var(--light-text);

            .label {
                color: var(--light-text);
            }
        }
    }
</style>
