<script lang="ts" context="module">
    // TODO(fkling): Add support for missing features
    //  - History more
    //  - Default context support
    //  - Global keyboard shortcut

    import { EditorSelection, EditorState, Prec, type Extension } from '@codemirror/state'
    import { EditorView } from '@codemirror/view'
    import { mdiCodeBrackets, mdiFormatLetterCase, mdiRegex } from '@mdi/js'

    import { goto, invalidate } from '$app/navigation'
    import {
        type Option,
        type Action,
        applyAction,
        modeScope,
        queryDiagnostic,
        overrideContextOnPaste,
        filterDecoration,
        tokenInfo,
        placeholder,
        showWhenEmptyWithoutContext,
        suggestions,
        searchHistoryExtension,
        onModeChange,
        setMode,
    } from '$lib/branded'
    import { query, type DocumentInput } from '$lib/graphql'
    import { SearchPatternType } from '$lib/graphql-operations'
    import Icon from '$lib/Icon.svelte'
    import BaseCodeMirrorQueryInput from '$lib/search/BaseQueryInput.svelte'
    import { user } from '$lib/stores'
    import Tooltip from '$lib/Tooltip.svelte'
    import { createSuggestionsSource } from '$lib/web'

    import { type QueryStateStore, getQueryURL, QueryState } from '../state'

    import { createRecentSearchesStore } from './recentSearches'
    import Suggestions from './Suggestions.svelte'

    const placeholderText = 'Search for code or files...'

    // Creates extensions that don't depend on props
    const position0 = EditorSelection.single(0)
    const staticExtensions: Extension = [
        placeholder(placeholderText, showWhenEmptyWithoutContext),
        EditorState.transactionFilter.of(transaction => {
            // This is a hacky way to "fix" the cursor position when the input receives
            // focus by clicking outside of it in Chrome.
            // Debugging has revealed that in such a case the transaction has a user event
            // 'select', the new selection is set to `0` and 'scrollIntoView' is 'false'.
            // This is different from other events that change the cursor position:
            // - Clicking on text inside the input (whether focused or not) will be a 'select.pointer'
            //   user event.
            // - Moving the cursor with arrow keys will be a 'select' user event but will also set
            //   'scrollIntoView' to 'true'
            // - Entering new characters will be of user type 'input'
            // - Selecting a text range will be of user type 'select.pointer'
            // - Tabbing to the input seems to only trigger a 'select' user event transaction when
            //   the user clicked outside the input (also only in Chrome, this transaction doesn't
            //   occur in Firefox)

            if (
                !transaction.isUserEvent('select.pointer') &&
                transaction.isUserEvent('select') &&
                !transaction.scrollIntoView &&
                transaction.selection?.eq(position0)
            ) {
                return [transaction, { selection: EditorSelection.single(transaction.newDoc.length) }]
            }
            return transaction
        }),
        modeScope([queryDiagnostic(), overrideContextOnPaste], [null]),
        Prec.low([modeScope([tokenInfo(), filterDecoration], [null])]),
        EditorView.theme({
            '&': {
                flex: 1,
                backgroundColor: 'var(--input-bg)',
                borderRadius: 'var(--border-radius)',
                borderColor: 'var(--border-color)',
                // To ensure that the input doesn't overflow the parent
                minWidth: 0,
                marginRight: '0.5rem',
            },
            '&.cm-editor.cm-focused': {
                outline: 'none',
            },
            '.cm-scroller': {
                overflowX: 'hidden',
            },
            '.cm-content': {
                paddingLeft: '0.25rem',
            },
            '.cm-content.focus-visible': {
                boxShadow: 'none',
            },
            '.sg-decorated-token-hover': {
                borderRadius: '3px',
            },
            '.sg-query-filter-placeholder': {
                color: 'var(--text-muted)',
                fontStyle: 'italic',
            },
        }),
    ]

    async function graphqlQuery<T, V extends Record<string, any>>(request: DocumentInput, variables: V) {
        const result = await query<T, V>(request, variables)
        // This is a hack to make urlq work with the API that createSuggestionsSource expects
        return result.data ?? ({} as any)
    }
</script>

<script lang="ts">
    import { mdiClockOutline } from '@mdi/js'

    export let queryState: QueryStateStore
    export let autoFocus = false

    export function focus() {
        input?.focus()
    }

    const popoverID = 'main-search'
    const recentSearches = createRecentSearchesStore()

    let input: BaseCodeMirrorQueryInput
    let editor: EditorView | null = null
    let mode = ''
    let suggestionsPaddingTop = 0
    let suggestionsUI: Extension = []

    // When autofocus is set we only show suggestions when the user has interacted with the input
    let userHasInteracted = !autoFocus
    const hasInteractedExtension = EditorView.updateListener.of(update => {
        if (!userHasInteracted) {
            if (update.transactions.some(tr => tr.isUserEvent('select') || tr.isUserEvent('input'))) {
                userHasInteracted = true
            }
        }
    })
    const searchHistory = searchHistoryExtension({
        mode: {
            name: 'History',
            placeholder: 'Filter history',
        },
        source: () => $recentSearches ?? [],
        submitQuery: (query, view) => {
            void submitQuery($queryState.setQuery(query))
            view.contentDOM.blur()
        },
    })

    $: regularExpressionEnabled = $queryState.patternType === SearchPatternType.regexp
    $: structuralEnabled = $queryState.patternType === SearchPatternType.structural
    $: suggestionsExtension = suggestions({
        id: popoverID,
        source: createSuggestionsSource({
            graphqlQuery,
            authenticatedUser: $user,
            isSourcegraphDotCom: false,
        }),
        navigate: goto,
    })

    $: extension = [
        onModeChange((_view, newMode) => (mode = newMode ?? '')),
        hasInteractedExtension,
        suggestionsExtension,
        suggestionsUI,
        searchHistory,
        staticExtensions,
    ]

    function setOrUnsetPatternType(patternType: SearchPatternType): void {
        queryState.setPatternType(currentPatternType =>
            currentPatternType === patternType ? SearchPatternType.keyword : patternType
        )
    }

    async function submitQuery(state: QueryState): Promise<void> {
        // This ensures that the same query can be resubmitted from the search input. Without
        // this, SvelteKit will not re-run the loader because the URL hasn't changed.
        await invalidate(`query:${state.query}--${state.caseSensitive}`)
        void goto(getQueryURL(state))
    }

    async function handleSubmit(event: Event) {
        event.preventDefault()
        if (!mode) {
            // Only submit query if you are not in history mode
            void submitQuery($queryState)
        }
    }

    function selectOption(event: { detail: { option: Option; action: Action } }): void {
        if (editor) {
            applyAction(editor, event.detail.action, event.detail.option, 'mouse')
            window.requestAnimationFrame(() => editor?.focus())
        }
    }

    function toggleMode() {
        if (editor) {
            setMode(editor, currentMode => (currentMode === 'History' ? null : 'History'))
            editor.focus()
        }
    }
</script>

<form
    bind:clientHeight={suggestionsPaddingTop}
    class="search-box"
    action="/search"
    method="get"
    on:submit={handleSubmit}
>
    <input class="hidden" value={$queryState.query} name="q" />
    <div class="focus-container" class:userHasInteracted>
        <div class="mode-switcher" class:active={!!mode}>
            <Tooltip tooltip="Recent searches">
                <button class="icon" type="button" on:click={toggleMode}>
                    <Icon svgPath={mdiClockOutline} inline />
                    {#if mode}
                        <span>{mode}:</span>
                    {/if}
                </button>
            </Tooltip>
        </div>
        <BaseCodeMirrorQueryInput
            {autoFocus}
            bind:this={input}
            bind:view={editor}
            placeholder="Search for code or files"
            value={$queryState.query}
            on:change={event => queryState.setQuery(event.detail)}
            on:enter={handleSubmit}
            patternType={$queryState.patternType}
            interpretComments={false}
            {extension}
        />
        {#if !mode}
            <Tooltip tooltip={`${$queryState.caseSensitive ? 'Disable' : 'Enable'} case sensitivity`}>
                <button
                    class="toggle icon"
                    type="button"
                    class:active={$queryState.caseSensitive}
                    on:click={() => queryState.setCaseSensitive(caseSensitive => !caseSensitive)}
                >
                    <Icon svgPath={mdiFormatLetterCase} inline />
                </button>
            </Tooltip>
            <Tooltip tooltip="{regularExpressionEnabled ? 'Disable' : 'Enable'} regular expression">
                <button
                    class="toggle icon"
                    type="button"
                    class:active={regularExpressionEnabled}
                    on:click={() => setOrUnsetPatternType(SearchPatternType.regexp)}
                >
                    <Icon svgPath={mdiRegex} inline />
                </button>
            </Tooltip>
            {#if structuralEnabled}
                <Tooltip tooltip="Disable structural search">
                    <button
                        class="toggle icon"
                        type="button"
                        class:active={structuralEnabled}
                        on:click={() => setOrUnsetPatternType(SearchPatternType.structural)}
                    >
                        <Icon svgPath={mdiCodeBrackets} inline />
                    </button>
                </Tooltip>
            {/if}
        {/if}
    </div>
    <div class="suggestions" style:padding-top="{suggestionsPaddingTop}px">
        <Suggestions bind:this={suggestionsUI} on:select={selectOption} />
    </div>
</form>

<style lang="scss">
    @use '$lib/breakpoints';

    form {
        isolation: isolate;
        width: 100%;
        position: relative;
        padding: 0.75rem;

        &:focus-within {
            .userHasInteracted + .suggestions {
                display: block;
            }
        }
    }

    .hidden {
        display: none;
    }

    .focus-container {
        flex: 1;
        display: flex;
        align-items: center;
        min-height: 32px;
        padding: 0 0.25rem;
        border-radius: 4px;
        border: 1px solid var(--border-color-2);
        background-color: var(--input-bg);
        position: relative;
        // This is necessary to ensure that the input is shown above the suggestions container
        z-index: 1;

        &:focus-within {
            @media (--sm-breakpoint-up) {
                outline: 2px solid var(--primary-2);
                outline-offset: 0;
                border-color: var(--primary-2);
            }
        }

        @media (--xs-breakpoint-down) {
            flex-direction: column;
            align-items: start;
            padding: 0.5rem;
            gap: 0.5rem;
        }
    }

    .suggestions {
        display: none;
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        border-radius: 8px;
        background-color: var(--color-bg-1);
        box-shadow: 0 10px 50px rgba(0, 0, 0, 0.15);

        // Set a default paddings to the suggestion panel (see Suggestions.module.scss)
        --suggestions-padding: 0.75rem;

        :global(.theme-dark) & {
            box-shadow: 0 10px 60px rgba(0, 0, 0, 0.8);
        }
    }

    button.toggle {
        width: 1.5rem;
        height: 1.5rem;
        cursor: pointer;
        border-radius: var(--border-radius);
        display: flex;
        align-items: center;
        justify-content: center;

        &.active {
            background-color: var(--primary);
            color: var(--light-text);
        }

        :global(svg) {
            transform: scale(1.172);
        }
    }

    button.icon {
        padding: 0;
        margin: 0;
        border: 0;
        background-color: transparent;
        cursor: pointer;
    }

    .mode-switcher {
        display: flex;
        align-items: center;
        padding-right: 0.1875rem;
        margin-right: 0.25rem;
        border-right: 1px solid var(--border-color-2);
        font-family: var(--code-font-family);
        font-size: var(--code-font-size);
        --color: var(--text-muted);

        button {
            padding: 0.0625rem 0.125rem;
            color: var(--color);
            border-radius: var(--border-radius);
            font-size: 0.875rem;
            display: flex;
            align-items: center;
            gap: 0.25rem;

            &:hover {
                background-color: var(--color-bg-2);
            }

            span {
                font-family: var(--code-font-family);
                font-size: var(--code-font-size);
            }
        }

        &.active {
            --color: var(--logo-purple);
            padding: 0;
            border: 0;
        }

        @media (--xs-breakpoint-down) {
            border: 0;
        }
    }
</style>
