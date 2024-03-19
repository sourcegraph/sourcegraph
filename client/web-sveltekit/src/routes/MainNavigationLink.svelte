<script lang="ts">
    import Icon from '$lib/Icon.svelte'
    import { type NavigationEntry, Status } from './mainNavigation'
    import { Badge } from '$lib/wildcard'

    export let entry: NavigationEntry
</script>

<a href={entry.href}>
    {#if typeof entry.icon === 'string'}
        <Icon svgPath={entry.icon} aria-hidden="true" inline />&nbsp;
    {:else if entry.icon}
        <span class="icon"><svelte:component this={entry.icon} /></span>&nbsp;
    {/if}
    {entry.label}
    {#if entry.status && entry.status & Status.BETA}
        &nbsp;
        <Badge variant="info">Beta</Badge>
    {/if}
</a>

<style lang="scss">
    a {
        display: flex;
        height: 100%;
        align-items: center;
        text-decoration: none;
        color: var(--body-color);

        &:hover {
            color: inherit;
            text-decoration: none;
        }
    }

    .icon {
        width: var(--icon-inline-size);
        height: var(--icon-inline-size);
        color: var(--header-icon-color);
        display: flex;
        align-items: center;
    }
</style>
