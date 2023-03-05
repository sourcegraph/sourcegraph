/**
 * An experimental implementation of the Blob view using CodeMirror
 */

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef } from 'react'

import { openSearchPanel } from '@codemirror/search'
import { Compartment, EditorState, Extension } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { isEqual } from 'lodash'
import { createPath, NavigateFunction, useLocation, useNavigate, Location } from 'react-router-dom'

import {
    addLineRangeQueryParameter,
    formatSearchParameters,
    toPositionOrRangeQueryParameter,
} from '@sourcegraph/common'
import { editorHeight, useCodeMirror } from '@sourcegraph/shared/src/components/CodeMirrorEditor'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { AbsoluteRepoFile, ModeSpec, parseQueryAndHash } from '@sourcegraph/shared/src/util/url'
import { useLocalStorage } from '@sourcegraph/wildcard'

import { ExternalLinkFields, Scalars } from '../../graphql-operations'
import { BlameHunkData } from '../blame/useBlameHunks'
import { HoverThresholdProps } from '../RepoContainer'

import { blobPropsFacet } from './codemirror'
import { createBlameDecorationsExtension } from './codemirror/blame-decorations'
import { codeFoldingExtension } from './codemirror/code-folding'
import { syntaxHighlight } from './codemirror/highlight'
import { selectableLineNumbers, SelectedLineRange, selectLines } from './codemirror/linenumbers'
import { lockFirstVisibleLine } from './codemirror/lock-line'
import { navigateToLineOnAnyClickExtension } from './codemirror/navigate-to-any-line-on-click'
import { occurrenceAtPosition, positionAtCmPosition } from './codemirror/occurrence-utils'
import { search } from './codemirror/search'
import { pin, updatePin, selectOccurrence } from './codemirror/token-selection/code-intel-tooltips'
import { tokenSelectionExtension } from './codemirror/token-selection/extension'
import { languageSupport } from './codemirror/token-selection/languageSupport'
import { selectionFromLocation } from './codemirror/token-selection/selections'
import { isValidLineRange } from './codemirror/utils'
import { setBlobEditView } from './use-blob-store'

// Logical grouping of props that are only used by the CodeMirror blob view
// implementation.
interface CodeMirrorBlobProps {
    overrideBrowserSearchKeybinding?: boolean
}

export interface BlobProps
    extends SettingsCascadeProps,
        PlatformContextProps,
        TelemetryProps,
        HoverThresholdProps,
        CodeMirrorBlobProps {
    className: string
    wrapCode: boolean
    /** The current text document to be rendered and provided to extensions */
    blobInfo: BlobInfo
    'data-testid'?: string

    // When navigateToLineOnAnyClick=true, the code intel popover is disabled
    // and clicking on any line should navigate to that specific line.
    navigateToLineOnAnyClick?: boolean

    // If set, nav is called when a user clicks on a token highlighted by
    // WebHoverOverlay
    nav?: (url: string) => void
    role?: string
    ariaLabel?: string

    supportsFindImplementations?: boolean

    isBlameVisible?: boolean
    blameHunks?: BlameHunkData

    activeURL?: string
}

export interface BlobPropsFacet extends BlobProps {
    navigate: NavigateFunction
    location: Location
}

export interface BlobInfo extends AbsoluteRepoFile, ModeSpec {
    /** The raw content of the blob. */
    content: string

    /** The trusted syntax-highlighted code as HTML */
    html: string

    /** LSIF syntax-highlighting data */
    lsif?: string

    /** If present, the file is stored in Git LFS (large file storage). */
    lfs?: { byteSize: Scalars['BigInt'] } | null

    /** External URLs for the file */
    externalURLs?: ExternalLinkFields[]
}

const staticExtensions: Extension = [
    EditorState.readOnly.of(true),
    EditorView.editable.of(false),
    EditorView.contentAttributes.of({
        // This is required to make the blob view focusable and to make
        // triggering the in-document search (see below) work when Mod-f is
        // pressed
        tabindex: '0',
        // CodeMirror defaults to role="textbox" which does not produce the
        // desired screenreader behavior we want for this component.
        // See https://github.com/sourcegraph/sourcegraph/issues/43375
        role: 'generic',
    }),
    editorHeight({ height: '100%' }),
    EditorView.theme({
        '&': {
            backgroundColor: 'var(--code-bg)',
        },
        '.cm-scroller': {
            fontFamily: 'var(--code-font-family)',
            fontSize: 'var(--code-font-size)',
            lineHeight: '1rem',
        },
        '.cm-gutters': {
            backgroundColor: 'var(--code-bg)',
            borderRight: 'initial',
        },
        '.cm-content:focus-visible': {
            outline: 'none',
            boxShadow: 'none',
        },
        '.cm-line': {
            paddingLeft: '0',
        },
        '.selected-line': {
            backgroundColor: 'var(--code-selection-bg)',
        },
        '.selected-line:focus': {
            boxShadow: 'none',
        },
        '.highlighted-line': {
            backgroundColor: 'var(--code-selection-bg)',
        },
    }),
]

// Compartments are used to reconfigure some parts of the editor without
// affecting others.

// Compartment to update various smaller settings
const settingsCompartment = new Compartment()
// Compartment to update blame decorations
const blameDecorationsCompartment = new Compartment()
// Compartment for propagating component props
const blobPropsCompartment = new Compartment()
// Compartment for line wrapping.
const wrapCodeCompartment = new Compartment()

export const CodeMirrorBlob: React.FunctionComponent<BlobProps> = props => {
    const {
        className,
        wrapCode,
        ariaLabel,
        role,
        isBlameVisible,
        blameHunks,

        // Reference panel specific props
        navigateToLineOnAnyClick,

        overrideBrowserSearchKeybinding,
        'data-testid': dataTestId,
    } = props

    const navigate = useNavigate()
    const location = useLocation()

    const [useFileSearch, setUseFileSearch] = useLocalStorage('blob.overrideBrowserFindOnPage', true)

    const containerRef = useRef<HTMLDivElement | null>(null)
    // This is used to avoid reinitializing the editor when new locations in the
    // same file are opened inside the reference panel.
    const blobInfo = useDistinctBlob(props.blobInfo)
    const position = useMemo(() => {
        // When an activeURL is passed, it takes presedence over the react
        // router location API.
        //
        // This is needed to support the reference panel
        if (props.activeURL) {
            const url = new URL(props.activeURL, window.location.href)
            return parseQueryAndHash(url.search, url.hash)
        }
        return parseQueryAndHash(location.search, location.hash)
    }, [props.activeURL, location.search, location.hash])
    const hasPin = useMemo(() => urlIsPinned(location.search), [location.search])

    const blobProps = useMemo(
        () =>
            blobPropsFacet.of({
                ...props,
                navigate,
                location,
            }),
        [props, navigate, location]
    )

    const isLightTheme = useIsLightTheme()
    const themeSettings = useMemo(() => EditorView.darkTheme.of(isLightTheme === false), [isLightTheme])
    const wrapCodeSettings = useMemo<Extension>(() => (wrapCode ? EditorView.lineWrapping : []), [wrapCode])

    const blameDecorations = useMemo(
        () => createBlameDecorationsExtension(!!isBlameVisible, blameHunks, isLightTheme),
        [isBlameVisible, blameHunks, isLightTheme]
    )

    // Keep history and location in a ref so that we can use the latest value in
    // the onSelection callback without having to recreate it and having to
    // reconfigure the editor extensions
    const navigateRef = useRef(navigate)
    navigateRef.current = navigate
    const locationRef = useRef(location)
    locationRef.current = location

    const customHistoryAction = props.nav
    const onSelection = useCallback(
        (range: SelectedLineRange) => {
            const parameters = new URLSearchParams(locationRef.current.search)
            parameters.delete('popover')

            let query: string | undefined

            if (range?.line !== range?.endLine && range?.endLine) {
                query = toPositionOrRangeQueryParameter({
                    range: {
                        start: { line: range.line },
                        end: { line: range.endLine },
                    },
                })
            } else if (range?.line) {
                query = toPositionOrRangeQueryParameter({ position: { line: range.line } })
            }

            const newSearchParameters = addLineRangeQueryParameter(parameters, query)
            if (customHistoryAction) {
                customHistoryAction(
                    createPath({
                        ...locationRef.current,
                        search: formatSearchParameters(newSearchParameters),
                    })
                )
            } else {
                updateBrowserHistoryIfChanged(navigateRef.current, locationRef.current, newSearchParameters)
            }
        },
        [customHistoryAction]
    )
    const extensions = useMemo(
        () => [
            // Log uncaught errors that happen in callbacks that we pass to
            // CodeMirror. Without this exception sink, exceptions get silently
            // ignored making it difficult to debug issues caused by uncaught
            // exceptions.
            // eslint-disable-next-line no-console
            EditorView.exceptionSink.of(exception => console.log(exception)),
            staticExtensions,
            selectableLineNumbers({
                onSelection,
                initialSelection: position.line !== undefined ? position : null,
                navigateToLineOnAnyClick: navigateToLineOnAnyClick ?? false,
            }),
            codeFoldingExtension(),
            navigateToLineOnAnyClick ? navigateToLineOnAnyClickExtension : tokenSelectionExtension(),
            syntaxHighlight.of(blobInfo),
            languageSupport.of(blobInfo),
            pin.init(() => (hasPin ? position : null)),
            blobPropsCompartment.of(blobProps),
            blameDecorationsCompartment.of(blameDecorations),
            settingsCompartment.of(themeSettings),
            wrapCodeCompartment.of(wrapCodeSettings),
            search({
                // useFileSearch is not a dependency because the search
                // extension manages its own state. This is just the initial
                // value
                overrideBrowserFindInPageShortcut: useFileSearch,
                onOverrideBrowserFindInPageToggle: setUseFileSearch,
            }),
        ],
        // A couple of values are not dependencies (blameDecorations, blobProps,
        // hasPin, position and settings) because those are updated in effects
        // further below. However, they are still needed here because we need to
        // set initial values when we re-initialize the editor.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [onSelection, blobInfo]
    )

    const editorRef = useRef<EditorView | null>(null)

    // Reconfigure editor when blobInfo or core extensions changed
    useEffect(() => {
        const editor = editorRef.current
        if (editor) {
            // We use setState here instead of dispatching a transaction because
            // the new document has nothing to do with the previous one and so
            // any existing state should be discarded.
            const state = EditorState.create({ doc: blobInfo.content, extensions })
            editor.setState(state)

            if (navigateToLineOnAnyClick) {
                /**
                 * `navigateToLineOnAnyClick` is `true` when CodeMirrorBlob is rendered in the references panel.
                 * We don't need code intel and keyboard navigation in the references panel blob: https://github.com/sourcegraph/sourcegraph/pull/41615.
                 */
                return
            }

            // Sync editor selection/focus with the URL so that triggering
            // `history.goBack/goForward()` works similar to the "Go back"
            // command in VS Code.
            const { selection } = selectionFromLocation(editor, locationRef.current)
            if (selection) {
                const position = positionAtCmPosition(editor, selection.from)
                const occurrence = occurrenceAtPosition(editor.state, position)
                if (occurrence) {
                    selectOccurrence(editor, occurrence)
                    // Automatically focus the content DOM to enable keyboard
                    // navigation. Without this automatic focus, users need to click
                    // on the blob view with the mouse.
                    // NOTE: this focus statment does not seem to have an effect
                    // when using macOS VoiceOver.
                    editor.contentDOM.focus({ preventScroll: true })
                }
            }
        }
    }, [blobInfo, extensions, navigateToLineOnAnyClick, locationRef])

    // Propagate props changes to extensions
    useEffect(() => {
        const editor = editorRef.current
        if (editor) {
            editor.dispatch({ effects: blobPropsCompartment.reconfigure(blobProps) })
        }
    }, [blobProps])

    // Update blame decorations
    useLayoutEffect(() => {
        const editor = editorRef.current
        if (editor) {
            const effects = [blameDecorationsCompartment.reconfigure(blameDecorations), ...lockFirstVisibleLine(editor)]
            editor.dispatch({ effects })
        }
    }, [blameDecorations])

    // Update theme
    useEffect(() => {
        const editor = editorRef.current
        if (editor) {
            editor.dispatch({ effects: settingsCompartment.reconfigure(themeSettings) })
        }
    }, [themeSettings])

    // Update line wrapping
    useEffect(() => {
        const editor = editorRef.current
        if (editor) {
            const effects = [wrapCodeCompartment.reconfigure(wrapCodeSettings), ...lockFirstVisibleLine(editor)]
            editor.dispatch({ effects })
        }
    }, [wrapCodeSettings])

    // Update selected lines when URL changes
    useEffect(() => {
        const editor = editorRef.current
        if (editor) {
            selectLines(editor, position.line ? position : null)
        }
    }, [position])

    // Update pinned hovercard range
    useEffect(() => {
        const editor = editorRef.current
        if (editor && (!hasPin || (position.line && isValidLineRange(position, editor.state.doc)))) {
            // Only update range if position is valid inside the document.
            updatePin(editor, hasPin ? position : null)
        }
    }, [position, hasPin])

    useCodeMirror(
        editorRef,
        containerRef,
        // We update the value ourselves
        // eslint-disable-next-line react-hooks/exhaustive-deps
        useMemo(() => blobInfo.content, []),
        // We update extensions ourselves
        // eslint-disable-next-line react-hooks/exhaustive-deps
        useMemo(() => extensions, [])
    )

    // Sync editor store with global Zustand store API
    useEffect(() => setBlobEditView(editorRef.current ?? null), [])

    const openSearch = useCallback(() => {
        if (editorRef.current) {
            openSearchPanel(editorRef.current)
        }
    }, [])

    return (
        <>
            <div
                ref={containerRef}
                aria-label={ariaLabel}
                role={role}
                data-testid={dataTestId}
                className={`${className} overflow-hidden test-editor`}
                data-editor="codemirror6"
            />
            {overrideBrowserSearchKeybinding && useFileSearch && (
                <Shortcut ordered={['f']} held={['Mod']} onMatch={openSearch} ignoreInput={true} />
            )}
        </>
    )
}

/**
 * Returns true when the URL indicates that the hovercard at the URL position
 * should be shown on load (the hovercard is "pinned").
 */
function urlIsPinned(search: string): boolean {
    return new URLSearchParams(search).get('popover') === 'pinned'
}

/**
 * Helper hook to prevent resetting the editor view if the blob contents hasn't
 * changed.
 */
function useDistinctBlob(blobInfo: BlobInfo): BlobInfo {
    const blobRef = useRef(blobInfo)
    return useMemo(() => {
        if (!isEqual(blobRef.current, blobInfo)) {
            blobRef.current = blobInfo
        }
        return blobRef.current
    }, [blobInfo])
}

/**
 * Adds an entry to the browser history only if new search parameters differ
 * from the current ones. This prevents adding a new entry when e.g. the user
 * clicks the same line multiple times.
 */
export function updateBrowserHistoryIfChanged(
    navigate: NavigateFunction,
    location: Location,
    newSearchParameters: URLSearchParams,
    /** If set to true replace the current history entry instead of adding a new one. */
    replace: boolean = false
): void {
    const currentSearchParameters = [...new URLSearchParams(location.search).entries()]

    // Update history if the number of search params changes or if any parameter
    // value changes. This will also work for file position changes, which are
    // encoded as parameter without a value. The old file position will be a
    // non-existing key in the new search parameters and thus return `null`
    // (whereas it returns an empty string in the current search parameters).
    const needsUpdate =
        currentSearchParameters.length !== [...newSearchParameters.keys()].length ||
        currentSearchParameters.some(([key, value]) => newSearchParameters.get(key) !== value)

    if (needsUpdate) {
        const entry = {
            ...location,
            search: formatSearchParameters(newSearchParameters),
        }

        navigate(entry, { replace })
    }
}
