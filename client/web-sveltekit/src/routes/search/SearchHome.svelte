<script lang="ts">
    import { setContext } from 'svelte'

    import { logoLight, logoDark } from '$lib/images'
    import SearchInput from '$lib/search/input/SearchInput.svelte'
    import type { QueryStateStore } from '$lib/search/state'
    import type { SearchPageContext } from '$lib/search/utils'
    import { isLightTheme } from '$lib/stores'

    import SearchHomeNotifications from './SearchHomeNotifications.svelte'

    export let queryState: QueryStateStore

    setContext<SearchPageContext>('search-context', {
        setQuery(newQuery) {
            queryState.setQuery(newQuery)
        },
    })
</script>

<section>
    <div class="content">
        <img class="logo" src={$isLightTheme ? logoLight : logoDark} alt="Sourcegraph Logo" />
        <div class="search">
            <SearchInput {queryState} autoFocus />
            <SearchHomeNotifications/>
        </div>
    </div>
</section>

<style lang="scss">
    section {
        overflow-y: auto;
        padding: 0 1rem;
        display: flex;
        flex-direction: column;
        flex: 1;
        align-items: center;
    }

    div.content {
        flex-shrink: 0;
        display: flex;
        flex-direction: column;
        align-items: center;
        width: 100%;
        max-width: 64rem;

        :global(.search-box) {
            align-self: stretch;
        }
    }

    .search {
        width: 100%;
        display: flex;
        flex-direction: column;
        gap: 2rem;
    }

    img.logo {
        width: 20rem;
        margin-top: 6rem;
        max-width: 90%;
        min-height: 54px;
        margin-bottom: 3rem;
    }
</style>
