<script lang="ts">
    import { getContext, onDestroy } from 'svelte'
    import * as uuid from 'uuid'

    import { type TabsContext, KEY } from './Tabs.svelte'

    export let title: string

    const context = getContext<TabsContext>(KEY)
    const id = uuid.v4()
    const tabId = `${context.id}-tab-${id}`
    onDestroy(
        context.register({
            id: tabId,
            title,
        })
    )
    $: selectedId = context.selectedTabID
    $: selected = $selectedId === tabId
</script>

{#if selected}
    <div id="{context.id}-panel-{id}" aria-labelledby={tabId} role="tabpanel" tabindex={selected ? 0 : -1}>
        <slot />
    </div>
{/if}

<style lang="scss">
    div {
        flex: 1;
        min-height: 0;
    }
</style>
