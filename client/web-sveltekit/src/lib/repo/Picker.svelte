<script context="module" lang="ts">
    interface Node {
        id: string
    }

    interface Result<T extends Node = Node> {
        nodes: T[]
        totalCount?: number
    }
</script>

<script lang="ts" generics="T extends Node">
    import { createCombobox, type ComboboxOptionProps } from '@melt-ui/svelte'

    import { createPromiseStore } from '$lib/utils'
    import { Input, Alert } from '$lib/wildcard'

    export let name: string
    export let seeAllItemsURL: string
    export let getData: (query: string) => PromiseLike<Result<T>>
    export let onSelect: (item: T) => void
    export let toOption: (item: T) => ComboboxOptionProps<string>

    const {
        elements: { menu, input, option },
        states: { inputValue },
    } = createCombobox<any>({
        portal: null,
        forceVisible: true,
        scrollAlignment: 'nearest',
        closeOnOutsideClick: false,
        onSelectedChange: ({ next }) => {
            const selectedItem = $data.value?.nodes.find(d => d.id === next?.value)
            if (selectedItem) {
                onSelect(selectedItem)
            }
            return next
        },
    })

    let debounceTimer: ReturnType<typeof setTimeout>
    const data = createPromiseStore<Result<T>>()
    // Start query initial suggestion
    data.set(getData($inputValue))
    const debounce = (callback: () => void) => {
        clearTimeout(debounceTimer)
        debounceTimer = setTimeout(callback, 450)
    }
    $: {
        debounce(() => {
            data.set(getData($inputValue))
        })
    }
    $: filteredData = $data.value ? $data.value.nodes : []
</script>

<div class="root" data-picker-root>
    <Input
        {...$input}
        actions={[input]}
        loading={$data.pending}
        autofocus={true}
        placeholder={`Search ${name}...`}
        data-picker-input
    />

    <div {...$menu} use:menu class="suggestions" data-picker-suggestions>
        <!-- Initial loading state (but don't show state if data from prev call is presented) -->
        {#if !$data.value && $data.pending}
            <span class="loading-state">Loading...</span>
        {/if}

        <!-- Error state (show error immediately) -->
        {#if !$data.pending && $data.error}
            <span class="error-state">
                <Alert variant="danger">
                    Unable to load {name} information: {$data.error.message}
                </Alert>
            </span>
        {/if}

        {#if !$data.error}
            <ul class="suggestions-list" data-picker-suggestions-list>
                {#each filteredData as value (value.id)}
                    <li
                        use:option
                        {...$option(toOption(value))}
                        class="suggestions-list-item"
                        data-picker-suggestions-list-item
                    >
                        <slot {value} />
                    </li>
                {/each}

                {#if filteredData.length === 0 && !$data.pending && !$data.error}
                    <li class="zero-data-state">
                        No {name} matching&nbsp;<b>{$inputValue}</b>, try different search query
                    </li>
                {/if}
            </ul>
        {/if}
    </div>

    <footer class="footer">
        <a href={seeAllItemsURL}>
            See all {name}
            {#if !$data.error && $data.value?.totalCount && $data.value?.totalCount !== 0}({$data.value
                    .totalCount}){/if}
            →
        </a>
    </footer>
</div>

<style lang="scss">
    .root {
        display: flex;
        flex-direction: column;

        :global([data-input-container]) {
            margin: 0.75rem;
        }
    }

    .loading-state,
    .error-state,
    .zero-data-state {
        grid-column: 1/-1;
        padding: 2rem;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--text-muted);
    }

    .suggestions {
        flex-grow: 1;
        min-height: 0;
        overflow: auto;

        // There is no way to turn off styles that come from
        // melt UI popover element, since we render suggestion
        // no in the melt UI popover we turn it off via CSS here.
        position: static !important;
        width: 100% !important;
    }

    .suggestions-list {
        height: 100%;
        margin: 0;
        padding: 0 0 0.5rem 0;
        list-style: none;
        overflow: auto;
        color: var(--text-body);
    }

    .suggestions-list-item {
        cursor: pointer;
        padding: 0.325rem 1rem;
        border-bottom: 1px solid var(--border-color);

        &:last-child {
            border-bottom: none;
        }

        &:hover,
        &[data-highlighted] {
            background: var(--color-bg-2);
            color: var(--text-title);
        }
    }

    .footer {
        border-top: 1px solid var(--border-color);

        a {
            padding: 0.75rem;
            width: 100%;
            height: 100%;
            display: flex;
            justify-content: center;
            align-items: center;
            font-weight: 500;

            &:hover {
                background: var(--color-bg-2);
            }
        }
    }
</style>
