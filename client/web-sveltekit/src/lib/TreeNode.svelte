<svelte:options immutable />

<script lang="ts" context="module">
    interface TreeNodeContext {
        level: number
    }
</script>

<script lang="ts" generics="T">
    import { createEventDispatcher, getContext, setContext } from 'svelte'

    import Icon from '$lib/Icon.svelte'
    import { Button } from '$lib/wildcard'

    import LoadingSpinner from './LoadingSpinner.svelte'
    import { updateTreeState, type TreeProvider, TreeStateUpdate } from './TreeView'
    import { getTreeContext } from './TreeView.svelte'

    const dispatch = createEventDispatcher<{ 'scope-change': { provider: TreeProvider<T> } }>()

    export let entry: T
    export let treeProvider: TreeProvider<T>

    let label: HTMLElement | undefined

    $: treeState = getTreeContext()
    $: nodeID = treeProvider.getNodeID(entry)
    $: expandable = treeProvider.isExpandable(entry)
    $: expanded = $treeState.expandedNodes.has(nodeID)
    $: selectable = treeProvider.isSelectable(entry)
    $: selected = $treeState.selected === nodeID
    $: tabindex = $treeState.focused === nodeID ? 0 : -1
    $: children = expandable && expanded ? treeProvider.fetchChildren(entry) : null
    $: disableScope = $treeState.disableScope

    let level = getContext<TreeNodeContext>('tree-node-nesting')?.level ?? 0
    setContext('tree-node-nesting', { level: level + 1 })

    function toggleOpen(expand?: boolean) {
        if (expandable) {
            $treeState = updateTreeState(
                $treeState,
                nodeID,
                expand ?? !expanded ? TreeStateUpdate.EXPANDANDFOCUS : TreeStateUpdate.COLLAPSEANDFOCUS
            )
        }
    }

    function handleScopeChange(event: Event) {
        event.stopPropagation()

        treeProvider.fetchChildren(entry).then(childTreeProvider => {
            dispatch('scope-change', { provider: childTreeProvider })
        })
    }

    $: if (selected && label) {
        const container = label.closest('[role="tree"]')
        if (container) {
            // Only scroll the active tree entry into the 'center' if the selected entry changed
            // by something other than user interaction. If we always 'center' then the tree
            // will "jump" as the user selects an entry with the keyboard or mouse, which is
            // disorienting.
            // But if we never 'center' then going back and forth might position the selected
            // entry at the top or bottom of the scroll container, which is not very visible.
            // So we only 'center' if focus is not on the tree container, which likely means
            // that the user is not interacting with the tree.
            label.scrollIntoView({ block: container.contains(document.activeElement) ? 'nearest' : 'center' })
        }
    }
</script>

<li
    role="treeitem"
    aria-selected={selectable ? selected : undefined}
    aria-expanded={expandable ? expanded : undefined}
    {tabindex}
    data-treeitem
    data-node-id={nodeID}
    class:disable-scope={disableScope}
    style="--tree-node-nested-level: {level}"
>
    <div bind:this={label} class="label" data-treeitem-label class:expandable>
        <!-- TODO: scoping is an operation specific to the file tree, but this
        is intended to be a generic tree component. We should not add a scope
        button here. -->
        <Button variant="icon" on:click={handleScopeChange} data-scope-button>
            <Icon icon={ILucideFocus} inline aria-hidden="true" />
        </Button>
        <!-- hide the open/close button to preserve alignment with expandable entries -->
        <div class="indented">
            {#if expandable}
                <!-- We have to stop even propagation because the tree root
                listens for click events for selecting items. We don't want the
                item to be selected when the open/close button is pressed. -->
                <Button
                    variant="icon"
                    on:click={event => {
                        event.stopPropagation()
                        toggleOpen()
                    }}
                    tabindex={-1}
                    aria-label="{expanded ? 'Collapse' : 'Expand'} subtree"
                >
                    <Icon icon={expanded ? ILucideChevronDown : ILucideChevronRight} inline aria-hidden="true" />
                </Button>
            {/if}
            <slot {entry} {expanded} toggle={toggleOpen} {label} />
        </div>
    </div>
    {#if expanded && children}
        {#await children}
            <div class="loading">
                <LoadingSpinner center={false} />
            </div>
        {:then treeProvider}
            <ul role="group">
                {#each treeProvider.getEntries() as entry (treeProvider.getNodeID(entry))}
                    <svelte:self {entry} {treeProvider} let:entry let:toggle let:expanded let:label on:scope-change>
                        <slot {entry} {toggle} {expanded} {label} />
                    </svelte:self>
                {/each}
            </ul>
        {:catch error}
            <slot name="error" {error} />
        {/await}
    {/if}
</li>

<style lang="scss">
    $shiftWidth: 1.25rem;
    $gap: 0.25rem;

    li[role='treeitem'] {
        --indent-size: calc(var(--tree-node-nested-level) * #{$shiftWidth});

        --scope-size: calc(var(--icon-inline-size) + #{$gap} - 1px);
        &.disable-scope {
            --scope-size: 0px;
            :global([data-scope-button]) {
                display: none;
            }
        }

        &[tabindex='0']:focus {
            box-shadow: none;

            > .label {
                box-shadow: var(--focus-shadow-inset);
            }
        }

        .label {
            display: flex;
            gap: $gap;
            padding: 0.2rem $gap;
            align-items: center;

            // Change icon color based on selected item state
            --icon-color: var(--tree-node-expand-icon-color);
            color: var(--tree-node-label-color, var(--text-body));

            :global([data-scope-button]) {
                visibility: hidden;
            }

            &.expandable:hover,
            &.expandable:focus {
                :global([data-scope-button]) {
                    visibility: visible;
                }
            }

            .indented {
                display: inherit;
                gap: inherit;
                margin-left: var(--indent-size);
                width: 100%;
            }
        }

        .loading {
            // Indent with two rem since loading represents next nested level
            margin-left: calc(var(--scope-size) + var(--indent-size) + 2 * #{$gap});
            margin-top: 0.25rem;
        }

        ul {
            position: relative;
            isolation: isolate;
            &::before {
                position: absolute;
                content: '';
                border-left: 1px solid var(--secondary);
                height: 100%;
                transform: translateX(calc(#{$gap} + var(--scope-size) + var(--icon-inline-size) / 2 - 1px));
                z-index: 1;
            }
        }
    }
</style>
