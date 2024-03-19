<script lang="ts">
    import {
        mdiAccount,
        mdiCodeTags,
        mdiCog,
        mdiHistory,
        mdiSourceBranch,
        mdiSourceCommit,
        mdiSourceRepository,
        mdiTag,
    } from '@mdi/js'

    import { page } from '$app/stores'
    import Icon from '$lib/Icon.svelte'

    import type { LayoutData } from './$types'
    import { DropdownMenu, MenuLink } from '$lib/wildcard'
    import { computeFit } from '$lib/dom'
    import { writable } from 'svelte/store'
    import { getButtonClassName } from '@sourcegraph/wildcard'
    import RepoSearchInput from './RepoSearchInput.svelte'

    export let data: LayoutData

    const menuOpen = writable(false)
    const navEntries: { path: string; icon: string; title: string }[] = [
        { path: '', icon: mdiCodeTags, title: 'Code' },
        { path: '/-/commits', icon: mdiSourceCommit, title: 'Commits' },
        { path: '/-/branches', icon: mdiSourceBranch, title: 'Branches' },
        { path: '/-/tags', icon: mdiTag, title: 'Tags' },
        { path: '/-/stats/contributors', icon: mdiAccount, title: 'Contributors' },
    ]
    const menuEntries: { path: string; icon: string; title: string }[] = [
        { path: '/-/compare', icon: mdiHistory, title: 'Compare' },
        { path: '/-/own', icon: mdiAccount, title: 'Ownership' },
        { path: '/-/embeddings', icon: '', title: 'Embeddings' },
        { path: '/-/batch-changes', icon: '', title: 'Batch changes' },
        { path: '/-/settings', icon: mdiCog, title: 'Settings' },
    ]

    let visibleNavEntries = navEntries.length
    $: navEntriesToShow = visibleNavEntries === navEntries.length ? navEntries : navEntries.slice(0, visibleNavEntries)
    $: overflowMenu = visibleNavEntries !== navEntries.length ? navEntries.slice(visibleNavEntries) : []
    $: allMenuEntries = [...overflowMenu, ...menuEntries]

    function isCodePage(repoURL: string, pathname: string) {
        return (
            pathname === repoURL || pathname.startsWith(`${repoURL}/-/blob`) || pathname.startsWith(`${repoURL}/-/tree`)
        )
    }

    function isActive(href: string, url: URL): boolean {
        return href === data.repoURL ? isCodePage(data.repoURL, $page.url.pathname) : url.pathname.startsWith(href)
    }

    $: ({ repoName, displayRepoName } = data)
</script>

<svelte:head>
    <title>{data.displayRepoName} - Sourcegraph</title>
</svelte:head>

<nav aria-label="repository">
    <h1><a href="/{repoName}"><Icon svgPath={mdiSourceRepository} inline /> {displayRepoName}</a></h1>
    <!--
        TODO: Add back revision
        {#if revisionLabel}
            @ <span class="button">{revisionLabel}</span>
        {/if}
        -->
    <ul use:computeFit on:fit={event => (visibleNavEntries = event.detail.itemCount)}>
        {#each navEntriesToShow as entry}
            {@const href = data.repoURL + entry.path}
            <li>
                <a {href} aria-current={isActive(href, $page.url) ? 'page' : undefined}>
                    {#if entry.icon}
                        <Icon svgPath={entry.icon} inline />
                    {/if}
                    <span class="ml-1">{entry.title}</span>
                </a>
            </li>
        {/each}
    </ul>
    <DropdownMenu
        open={menuOpen}
        triggerButtonClass={getButtonClassName({ variant: 'secondary', outline: true, size: 'sm' })}
        aria-label="{$menuOpen ? 'Close' : 'Open'} repo navigation"
    >
        <svelte:fragment slot="trigger">&hellip;</svelte:fragment>
        {#each allMenuEntries as entry}
            {@const href = data.repoURL + entry.path}
            <MenuLink {href}>
                <span class="overflow-entry" class:active={isActive(href, $page.url)}>
                    {#if entry.icon}
                        <Icon svgPath={entry.icon} inline />
                    {/if}
                    <span class="ml-1">{entry.title}</span>
                </span>
            </MenuLink>
        {/each}
    </DropdownMenu>
    <RepoSearchInput repoName={data.repoName} />
</nav>
<slot />

<style lang="scss">
    nav {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        overflow: hidden;
        padding: 0.5rem;
        border-bottom: 1px solid var(--border-color);
        flex-shrink: 0;

        a {
            color: var(--body-color);
            text-decoration: none;
        }
    }

    h1 {
        margin: 0;
        font-size: 1.3rem;
        white-space: nowrap;
    }

    ul {
        list-style: none;
        padding: 0;
        margin: 0;
        display: flex;
        gap: 0.5rem;
        overflow: hidden;
        flex: 1;

        li a {
            display: flex;
            height: 100%;
            align-items: center;
            padding: 0.25rem 0.5rem;
            border-radius: var(--border-radius);
            white-space: nowrap;

            &:hover {
                background-color: var(--color-bg-2);
            }

            &[aria-current='page'] {
                background-color: var(--color-bg-3);
            }
        }
    }

    .overflow-entry {
        width: 100%;
        display: inline-block;
        padding: 0 0.25rem;
        border-radius: var(--border-radius);
    }

    .active {
        background-color: var(--color-bg-3);
    }

    nav {
        color: var(--body-color);
    }
</style>
