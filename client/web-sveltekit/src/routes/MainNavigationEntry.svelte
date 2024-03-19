<script lang="ts">
    import Icon from '$lib/Icon.svelte'
    import { createDropdownMenu } from '@melt-ui/svelte'
    import { isCurrent, type NavigationEntry, type NavigationMenu } from './mainNavigation'
    import { mdiChevronDown, mdiChevronUp } from '@mdi/js'
    import { page } from '$app/stores'
    import MainNavigationLink from './MainNavigationLink.svelte'

    export let entry: NavigationEntry | NavigationMenu

    function isNavigationEntry(entry: NavigationEntry | NavigationMenu): entry is NavigationEntry {
        return entry && 'href' in entry
    }

    const {
        elements: { menu, item, trigger },
        states: { open },
    } = createDropdownMenu({
        positioning: {
            placement: 'bottom-start',
            gutter: 0,
        },
    })
    $: current = isNavigationEntry(entry) ? isCurrent(entry, $page) : entry.isCurrent($page)
</script>

<li class="toplevel-naventry" aria-current={current ? 'page' : 'false'}>
    {#if isNavigationEntry(entry)}
        <MainNavigationLink {entry} />
    {:else}
        <button {...$trigger} use:trigger>
            {#if typeof entry.icon === 'string'}
                <Icon svgPath={entry.icon} aria-hidden="true" inline />&nbsp;
            {:else if entry.icon}
                <span class="icon"><svelte:component this={entry.icon} /></span>&nbsp;
            {/if}
            {entry.label}
            <Icon svgPath={$open ? mdiChevronUp : mdiChevronDown} inline />
        </button>
        <ul {...$menu} use:menu>
            {#each entry.children as subEntry (subEntry.label)}
                <li {...$item} use:item>
                    <MainNavigationLink entry={subEntry} />
                </li>
            {/each}
        </ul>
    {/if}
</li>

<style lang="scss">
    li.toplevel-naventry {
        --color: var(--header-icon-color);

        position: relative;
        display: flex;
        align-items: stretch;
        margin: 0 0.5rem;
        white-space: nowrap;
        border-color: transparent;

        &:hover {
            border-color: var(--border-color-2);
        }

        &[aria-current='page'] {
            border-color: var(--brand-secondary);
        }

        > button,
        :global(a) {
            border-bottom: 2px solid;
            border-color: inherit;
        }
    }

    button {
        all: unset;
        display: flex;
        align-items: center;
        cursor: pointer;

        // Since this button is part navigation links blocks
        // we should override focus ring with inset to avoid
        // visual cropping with parent border.
        &:focus-visible {
            box-shadow: 0 0 0 2px var(--primary-2) inset;
        }
    }

    [role='menu'] {
        font-size: 0.875rem;
        min-width: 10rem;
        background-clip: padding-box;
        background-color: var(--dropdown-bg);
        border: 1px solid var(--dropdown-border-color);
        border-radius: var(--popover-border-radius);
        color: var(--body-color);
        box-shadow: var(--dropdown-shadow);
        padding: 0.25rem 0;
    }

    [role^='menuitem'] {
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
</style>
