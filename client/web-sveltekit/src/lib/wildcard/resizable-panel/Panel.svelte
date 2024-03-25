<script lang="ts">
    import { getContext, onDestroy } from 'svelte'
    import type { PanelGroupContext } from './types'
    import { getId } from './utils/common'

    export let id: string | null = null
    export let minSize: number | undefined = undefined
    export let maxSize: number | undefined = undefined
    export let defaultSize: number | undefined = undefined
    export let order: number | undefined = undefined

    const panelId = id ?? getId()
    let panelElement: HTMLElement

    const { groupId, getPanelStyles, registerPanel } = getContext<PanelGroupContext>('panel-group-context')

    onDestroy(
        registerPanel({
            id: panelId,
            order,
            constraints: { defaultSize, minSize, maxSize },
            getPanelElement: () => panelElement,
        })
    )

    $: styles = getPanelStyles(panelId)
</script>

<div
    id={panelId}
    class="panel"
    style={$styles}
    bind:this={panelElement}
    data-panel
    data-panel-id={panelId}
    data-panel-group-id={groupId}
>
    <slot />
</div>

<style>
    .panel {
        flex-basis: 0;
        flex-shrink: 0;

        /* Without this, Panel sizes may be unintentionally overridden
         by their content */
        overflow: hidden;
    }
</style>
