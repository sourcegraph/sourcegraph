<script lang="ts">
    import { writable } from 'svelte/store'

    import { browser, dev } from '$app/environment'
    import { isErrorLike } from '$lib/common'
    import { TemporarySettingsStorage } from '$lib/shared'
    import { isLightTheme, setAppContext, scrollAll } from '$lib/stores'
    import { createTemporarySettingsStorage } from '$lib/temporarySettings'
    import { classNames } from '$lib/dom'

    import Header from './Header.svelte'

    import './styles.scss'

    import type { LayoutData } from './$types'
    import { createFeatureFlagStore, featureFlag } from '$lib/featureflags'
    import GlobalNotification from '$lib/global-notifications/GlobalNotifications.svelte'
    import { getGraphQLClient } from '$lib/graphql/apollo'
    import { isRouteRolledOut } from '$lib/navigation'
    import { beforeNavigate } from '$app/navigation'
    import { onDestroy } from 'svelte'

    export let data: LayoutData

    const user = writable(data.user ?? null)
    const settings = writable(isErrorLike(data.settings) ? null : data.settings)
    // It's OK to set the temporary storage during initialization time because
    // sign-in/out currently performs a full page refresh
    const temporarySettingsStorage = createTemporarySettingsStorage(
        data.user
            ? new TemporarySettingsStorage(getGraphQLClient(), true)
            : // Logged out storage
              new TemporarySettingsStorage(null, false)
    )

    setAppContext({
        user,
        settings,
        temporarySettingsStorage,
        featureFlags: createFeatureFlagStore(data.featureFlags, data.fetchEvaluatedFeatureFlags),
    })

    // We need to manually subscribe instead of using $isLightTheme because
    // at the moment Svelte tries to automatically subscribe to the store
    // the app context is not yet set.
    let lightTheme = false
    onDestroy(isLightTheme.subscribe(value => (lightTheme = value)))

    // Update stores when data changes
    $: $user = data.user ?? null
    $: $settings = isErrorLike(data.settings) ? null : data.settings

    $: if (browser) {
        document.documentElement.classList.toggle('theme-light', lightTheme)
        document.documentElement.classList.toggle('theme-dark', !lightTheme)
    }

    $: allRoutesEnabled = featureFlag('web-next')
    $: rolledoutRoutesEnabled = featureFlag('web-next-rollout')

    // Redirect the user to the react app when they navigate to a page that is
    // supported but not enabled.
    // (Routes that are not supported, i.e. don't exist in `routes/` are already
    // handled by SvelteKit (by triggering a browser refresh)).
    beforeNavigate(navigation => {
        if (navigation.willUnload || !navigation.to) {
            // Nothing to do here, request is already handled by the server
            return
        }

        if (dev || $allRoutesEnabled || ($rolledoutRoutesEnabled && isRouteRolledOut(navigation.to?.route.id ?? ''))) {
            // Routes are handled by SvelteKit
            return
        }

        // Trigger page refresh to fetch the React app from the server
        navigation.cancel()
        window.location.href = navigation.to.url.toString()
    })
</script>

<svelte:head>
    <title>Sourcegraph</title>
    <meta name="description" content="Code search" />
</svelte:head>

<svelte:body use:classNames={$scrollAll ? '' : 'overflowHidden'} />

{#await data.globalSiteAlerts then globalSiteAlerts}
    {#if globalSiteAlerts}
        <GlobalNotification globalAlerts={globalSiteAlerts} />
    {/if}
{/await}

<Header authenticatedUser={$user} />

<main>
    <slot />
</main>

<style lang="scss">
    :global(body.overflowHidden) {
        display: flex;
        flex-direction: column;
        height: 100vh;
        overflow: hidden;

        main {
            overflow-y: auto;
        }
    }

    main {
        isolation: isolate;
        flex: 1;
        display: flex;
        flex-direction: column;
        box-sizing: border-box;
    }
</style>
