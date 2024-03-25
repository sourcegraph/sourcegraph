<script lang="ts">
    // This is a forked ported to svelte version of react-resizable-panels package
    // It's licenced under MIT license, original source - https://github.com/bvaughn/react-resizable-panels/tree/main

    import { onDestroy, setContext } from 'svelte'
    import { derived, type Readable, type Unsubscriber, writable, type Writable } from 'svelte/store'

    import { Exceed, PanelResizeHandleRegistry } from './PanelResizeHandleRegistry'
    import { assert } from './utils/assert'
    import { findPanelDataIndex, getId, sortPanels } from './utils/common'
    import { calculateUnsafeDefaultLayout } from './utils/calculateUnsafeDefaultLayout'
    import { validatePanelGroupLayout } from './utils/validatePanelGroupLayout'
    import { computePanelFlexBoxStyle } from './utils/computePanelFlexBoxStyle'
    import { determinePivotIndices } from './utils/determinePivotIndices'
    import { calculateDeltaPercentage } from './utils/calculateDeltaPercentage'
    import { adjustLayoutByDelta } from './utils/adjustLayoutByDelta'
    import { compareLayouts } from './utils/compareLayouts'
    import { getResizeHandleElement } from './utils/dom'
    import { getResizeEventCursorPosition } from './event/getResizeEventCursorPosition'
    import { isKeyDown, isMouseEvent, isTouchEvent } from './event'
    import type {
        Direction,
        DragState,
        PanelGroupContext,
        PanelId,
        PanelInfo,
        PanelsLayout,
        ResizeEvent,
    } from './types'
    import { PanelGroupDirection } from './types'

    // Props
    export let direction: Direction = PanelGroupDirection.Horizontal

    // Local state
    let groupElement: HTMLElement

    // Used in resize handler to avoid cursor panel UI flickering
    let prevDelta: number | null = null
    const groupId = `resizable-group-panel-${getId()}`

    const layoutStore: Writable<PanelsLayout> = writable([])
    const panelsStore: Writable<PanelInfo[]> = writable([])
    const dragStateStore: Writable<DragState | null> = writable(null)

    onDestroy(
        // Connect registered panels store with layout store
        // set initial layout as we populate new panel elements
        panelsStore.subscribe(panels => {
            const unsafeLayout = calculateUnsafeDefaultLayout(panels)

            // TODO [VK]: Add auto saved layout support here.

            // Validate even saved layouts in case something has changed since last render
            // e.g. for pixel groups, this could be the size of the window
            $layoutStore = validatePanelGroupLayout(
                unsafeLayout,
                panels.map(panelData => panelData.constraints)
            )
        })
    )

    function registerPanel(panel: PanelInfo): Unsubscriber {
        panelsStore.update(panels => {
            if (panels.some(existingPanel => existingPanel.id === panel.id)) {
                return panels
            }

            // Since ordering plays a big role when it comes to hide/show
            // and restore panel size we should keep panels list ordered
            // If order field isn't provided we rely on Svelte rendering order
            return sortPanels([...panels, panel])
        })

        return () => {
            panelsStore.update(panels => panels.filter(existingTab => existingTab.id !== panel.id))
        }
    }

    function getResizeHandler(dragHandleId: string) {
        return function resizeHandler(event: ResizeEvent): void {
            event.preventDefault()

            if (!groupElement) {
                return
            }

            const { initialLayout } = $dragStateStore ?? {}
            const pivotIndices = determinePivotIndices(groupId, dragHandleId, groupElement)
            const delta = calculateDeltaPercentage(
                event,
                dragHandleId,
                direction as PanelGroupDirection,
                $dragStateStore,
                null,
                groupElement
            )

            if (delta === 0) {
                return
            }

            const isHorizontal = direction === 'horizontal'
            const panelConstraints = $panelsStore.map(panel => panel.constraints)

            const nextLayout = adjustLayoutByDelta({
                delta,
                initialLayout: initialLayout ?? $layoutStore,
                panelConstraints,
                pivotIndices,
                prevLayout: $layoutStore,
                trigger: isKeyDown(event) ? 'keyboard' : 'mouse-or-touch',
            })

            const layoutChanged = !compareLayouts($layoutStore, nextLayout)

            // Only update the cursor for layout changes triggered by touch/mouse events (not keyboard)
            // Update the cursor even if the layout hasn't changed (we may need to show an invalid cursor state)
            if (isMouseEvent(event) || isTouchEvent(event)) {
                // Watch for multiple subsequent deltas; this might occur for tiny cursor movements.
                // In this case, Panel sizes might not change–
                // but updating cursor in this scenario would cause a flicker.
                if (prevDelta !== delta) {
                    prevDelta = delta

                    if (!layoutChanged) {
                        // If the pointer has moved too far to resize the panel any further, note
                        // this so we can update the cursor. This mimics VS Code behavior.
                        if (isHorizontal) {
                            PanelResizeHandleRegistry.reportConstraintsViolation(
                                dragHandleId,
                                delta < 0 ? Exceed.HORIZONTAL_MIN : Exceed.HORIZONTAL_MAX
                            )
                        } else {
                            PanelResizeHandleRegistry.reportConstraintsViolation(
                                dragHandleId,
                                delta < 0 ? Exceed.VERTICAL_MIN : Exceed.VERTICAL_MAX
                            )
                        }
                    } else {
                        PanelResizeHandleRegistry.reportConstraintsViolation(dragHandleId, Exceed.NO_CONSTRAINT)
                    }
                }
            }

            if (layoutChanged) {
                $layoutStore = nextLayout
            }
        }
    }

    function startDragging(dragHandleId: string, event: ResizeEvent): void {
        const handleElement = getResizeHandleElement(dragHandleId, groupElement)

        assert(handleElement, `Drag handle element not found for id "${dragHandleId}"`)

        const initialCursorPosition = getResizeEventCursorPosition(direction as PanelGroupDirection, event)

        $dragStateStore = {
            dragHandleId,
            dragHandleRect: handleElement.getBoundingClientRect(),
            initialCursorPosition,
            initialLayout: $layoutStore,
        }
    }

    function stopDragging() {
        $dragStateStore = null
    }

    function getPanelStyles(panelId: PanelId): Readable<string> {
        return derived([layoutStore, panelsStore, dragStateStore], ([layout, panels, dragState]) => {
            return computePanelFlexBoxStyle({
                dragState,
                layout,
                panels,
                panelIndex: findPanelDataIndex(panels, panelId),
            })
        })
    }

    setContext<PanelGroupContext>('panel-group-context', {
        groupId,
        registerPanel,
        getResizeHandler,
        startDragging,
        stopDragging,
        getPanelStyles,
        panelsStore,
        dragStateStore,
        getPanelGroupElement: () => groupElement,
        direction: direction as PanelGroupDirection,
    })
</script>

<div
    class="root"
    bind:this={groupElement}
    style:flex-direction={direction === PanelGroupDirection.Horizontal ? 'row' : 'column'}
    data-panel-group
    data-panel-group-id={groupId}
    data-panel-group-direction={direction}
>
    <slot />
</div>

<style lang="scss">
    .root {
        display: flex;
        width: 100%;
        height: 100%;
        overflow: hidden;
    }
</style>
