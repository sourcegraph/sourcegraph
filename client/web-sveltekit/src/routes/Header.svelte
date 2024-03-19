<script lang="ts">
    import { mark } from '$lib/images'

    import { Badge, Button } from '$lib/wildcard'
    import UserMenu from './UserMenu.svelte'
    import type { Header_User } from './Header.gql'
    import { mainNavigation } from './mainNavigation'
    import MainNavigationEntry from './MainNavigationEntry.svelte'
    import Popover from '$lib/Popover.svelte'
    import { browser } from '$app/environment'
    import { page } from '$app/stores'

    export let authenticatedUser: Header_User | null | undefined

    const isDevOrS2 =
        (browser && window.location.hostname === 'localhost') ||
        window.location.hostname === 'sourcegraph.sourcegraph.com'

    $: reactURL = (function (url) {
        const urlCopy = new URL(url)
        urlCopy.searchParams.delete('feat')
        for (let feature of urlCopy.searchParams.getAll('feat')) {
            if (feature !== 'web-next' && feature !== 'web-next-rollout') {
                urlCopy.searchParams.append('feat', feature)
            }
        }
        urlCopy.searchParams.append('feat', '-web-next')
        urlCopy.searchParams.append('feat', '-web-next-rollout')
        return urlCopy.toString()
    })($page.url)
</script>

<header>
    <a class="logo" href="/search">
        <img src={mark} alt="Sourcegraph" width="25" height="25" />
    </a>
    <nav>
        <ul>
            {#each mainNavigation as entry (entry.label)}
                <MainNavigationEntry {entry} />
            {/each}
        </ul>
    </nav>
    <Popover let:registerTrigger showOnHover>
        <span class="web-next-badge" use:registerTrigger>
            <Badge variant="warning">Experimental</Badge>
        </span>
        <div slot="content" class="web-next-content">
            <h3>Experimental web app</h3>
            <p>
                You are using an experimental version of the Sourcegraph web app. This version is under active
                development and may contain bugs or incomplete features.
            </p>
            {#if isDevOrS2}
                <p>
                    If you encounter any issues, please report them in our <a
                        href="https://sourcegraph.slack.com/archives/C05MHAP318B">Slack channel</a
                    >.
                </p>
            {/if}
            <p>
                You can temporarily switch back to the stable version of the web app by clicking <a href={reactURL}
                    >here</a
                >.
            </p>
        </div>
    </Popover>
    <div>
        {#if authenticatedUser}
            <UserMenu user={authenticatedUser} />
        {:else}
            <Button variant="secondary" outline>
                <svelte:fragment slot="custom" let:buttonClass>
                    <a class={buttonClass} href="/sign-in">Sign in</a>
                </svelte:fragment>
            </Button>
        {/if}
    </div>
</header>

<style lang="scss">
    header {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        height: var(--navbar-height);
        min-height: 40px;
        padding: 0 0.5rem;
        border-bottom: 1px solid var(--border-color-2);
        background-color: var(--color-bg-1);
    }

    .logo img {
        &:hover {
            @keyframes spin {
                50% {
                    transform: rotate(180deg) scale(1.2);
                }
                100% {
                    transform: rotate(180deg) scale(1);
                }
            }

            @media (prefers-reduced-motion: no-preference) {
                animation: spin 0.5s ease-in-out 1;
            }
        }
    }

    nav {
        display: flex;
        align-self: stretch;
        flex: 1;
        overflow-y: auto;
    }

    ul {
        position: relative;
        padding: 0;
        margin: 0;
        display: flex;
        justify-content: center;
        list-style: none;
        background-size: contain;
    }

    .web-next-badge {
        cursor: pointer;
        padding: 0.5rem;
    }

    .web-next-content {
        padding: 1rem;
        width: 20rem;

        p:last-child {
            margin-bottom: 0;
        }
    }
</style>
