<script lang="ts" context="module">
    import { EditorView, drawSelection, keymap } from '@codemirror/view'
    import { history, defaultKeymap, historyKeymap } from '@codemirror/commands'
    import { querySyntaxHighlighting } from '$lib/branded'
    import { defaultTheme } from '$lib/codemirror/theme'

    // These extensions do not depend on any component props
    const staticExtensions: Extension = [
        // Default keybindings to ensure the editor behaves correctly
        keymap.of(defaultKeymap),
        // Additional keybindings history support
        keymap.of(historyKeymap),
        // History support. It allows the user, together with the history keybindings
        // to redo/undo input changes.
        history(),
        // Let CodeMirror style selected text to make it work better with decorations
        // that change the background color.
        drawSelection(),
        // Apply syntax highlighting to query elements
        querySyntaxHighlighting,
        // The input is styled deliberately without a border so that it
        // can be integrated with various other UI elements.
        EditorView.theme({
            '.cm-content': {
                caretColor: 'var(--search-query-text-color)',
                color: 'var(--search-query-text-color)',
                fontFamily: 'var(--code-font-family)',
                fontSize: 'var(--code-font-size)',
                // Reset default padding
                padding: 0,
                // We need 1px padding to make the cursor visible at position 0
                paddingLeft: '1px',
            },
            '.cm-line': {
                // Reset default padding
                padding: 0,
            },
            '&.cm-focused .cm-selectionLayer .cm-selectionBackground': {
                backgroundColor: 'var(--code-selection-bg-2) !important',
            },
            '.cm-selectionLayer .cm-selectionBackground': {
                backgroundColor: 'var(--code-selection-bg) !important',
            },
        }),
        defaultTheme,
    ]
</script>

<script lang="ts">
    import { EditorState, type Extension } from '@codemirror/state'
    import { createEventDispatcher } from 'svelte'

    import { browser } from '$app/environment'
    import { multiline, parseInputAsQuery, searchInputEventHandlers, toSingleLine } from '$lib/branded'
    import type { SearchPatternType } from '$lib/graphql-operations'
    import { createCompartments } from '$lib/codemirror/utils'
    import { afterNavigate } from '$app/navigation'

    export let value: string
    export let patternType: SearchPatternType
    export let interpretComments: boolean = false
    export let placeholder: string = ''
    export let autoFocus: boolean = false
    export let readOnly: boolean = false
    export let multiLine: boolean = false
    export let extension: Extension = []

    /**
     * Bind to this properly to get a reference to CodeMirror
     */
    export let view: EditorView | null = null

    export function focus() {
        view?.focus()
        view?.dispatch({ selection: { anchor: view.state.doc.length } })
    }

    const dispatch = createEventDispatcher<{ change: string; enter: EditorView }>()

    let container: HTMLDivElement | null = null
    let empty: Extension = []

    const compartments = createCompartments({
        additionalExtension: empty,
        parsedQueryExtension: empty,
        readOnlyExtension: empty,
        multilineExtension: empty,
    })

    let eventHandlerExtension = searchInputEventHandlers.init(() => ({
        onChange(value) {
            dispatch('change', value)
        },
        onEnter(view) {
            dispatch('enter', view)
            return true
        },
    }))

    function createEditor(container: HTMLDivElement, doc: string, extensions: Extension): EditorView {
        const view = new EditorView({
            state: EditorState.create({ doc, extensions }),
            parent: container,
        })
        return view
    }

    $: parsedQueryExtension = parseInputAsQuery({ patternType, interpretComments })
    $: readOnlyExtension = EditorView.editable.of(!readOnly)
    $: multilineExtension = multiline(multiLine)
    $: additionalExtension = extension
    $: extensions = { additionalExtension, parsedQueryExtension, readOnlyExtension, multilineExtension }
    $: if (view) {
        compartments.update(view, extensions)
    }
    $: normalizedValue = multiLine ? value : toSingleLine(value)

    $: if (view && view.state.sliceDoc() !== normalizedValue) {
        view.dispatch({
            changes: { from: 0, to: view.state.doc.length, insert: normalizedValue },
        })
    }

    $: if (container && !view) {
        view = createEditor(container, normalizedValue, [
            compartments.init(extensions),
            eventHandlerExtension,
            staticExtensions,
        ])

        // This is only run once when the view is created
        if (view && autoFocus) {
            view.focus()
        }
    }

    afterNavigate(() => {
        // We also set focus after navigation because SvelteKit resets the focus
        // when navigating to a new page.
        if (autoFocus) {
            view?.focus()
        }
    })
</script>

{#if browser}
    <div bind:this={container} class="root test-query-input test-editor" data-editor="codemirror6" />
{:else}
    <div class="root">
        <input value={normalizedValue} {placeholder} />
    </div>
{/if}

<style lang="scss">
    input {
        border: 0;
        font-family: var(--code-font-family);
        font-size: var(--code-font-size);
        width: 100%;
    }

    .root {
        flex: 1;
        box-sizing: border-box;
        min-width: 0;
        padding-left: 1px;

        :global(.cm-placeholder) {
            // CodeMirror uses display: inline-block by default, but that causes
            // Chrome to render a larger cursor if the placeholder holder spans
            // multiple lines. Firefox doesn't have this problem (but
            // setting display: inline doesn't seem to have a negative effect
            // either)
            display: inline;
            // Once again, Chrome renders the placeholder differently than
            // Firefox. CodeMirror sets 'word-break: break-word' (which is
            // deprecated) and 'overflow-wrap: anywhere' but they don't seem to
            // have an effect in Chrome (at least not in this instance).
            // Setting 'word-break: break-all' explicitly makes appearances a
            // bit better for example queries with long tokens.
            word-break: break-all;
        }
    }
</style>
