/**
 * An experimental implementation of the Blob view using CodeMirror
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { search, searchKeymap } from '@codemirror/search'
import { Compartment, EditorState, Extension } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import { isEqual } from 'lodash'

import { addLineRangeQueryParameter, LineOrPositionOrRange, toPositionOrRangeQueryParameter } from '@sourcegraph/common'
import { createUpdateableField, editorHeight, useCodeMirror } from '@sourcegraph/shared/src/components/CodeMirrorEditor'
import { parseQueryAndHash, UIPositionSpec } from '@sourcegraph/shared/src/util/url'

import { enableExtensionsDecorationsColumnViewFromSettings } from '../../util/settings'

import { blameDecorationType, BlobInfo, BlobProps, updateBrowserHistoryIfChanged } from './Blob'
import { blobPropsFacet } from './codemirror'
import {
    enableExtensionsDecorationsColumnView as enableColumnView,
    showTextDocumentDecorations,
} from './codemirror/decorations'
import { syntaxHighlight } from './codemirror/highlight'
import { hovercardRanges } from './codemirror/hovercard'
import { selectLines, selectableLineNumbers, SelectedLineRange } from './codemirror/linenumbers'
import { sourcegraphExtensions } from './codemirror/sourcegraph-extensions'
import { offsetToUIPosition, uiPositionToOffset } from './codemirror/utils'

const staticExtensions: Extension = [
    // Using EditorState.readOnly instead of EditorView.editable allows us to
    // focus the editor and placing a text cursor
    EditorState.readOnly.of(true),
    editorHeight({ height: '100%' }),
    EditorView.theme({
        '&': {
            fontFamily: 'var(--code-font-family)',
            fontSize: 'var(--code-font-size)',
            backgroundColor: 'var(--code-bg)',
        },
        '.selected-line': {
            backgroundColor: 'var(--code-selection-bg)',
        },
        '.cm-gutters': {
            backgroundColor: 'var(--code-bg)',
            borderRight: 'initial',
        },
    }),
    // Note that these only work out-of-the-box because the editor is
    // *focusable* but read-only (see EditorState.readOnly above).
    search({ top: true }),
    keymap.of(searchKeymap),
]

// Compartments are used to reconfigure some parts of the editor withoug
// affecting others.

// Compartment to update various smaller settings
const settingsCompartment = new Compartment()
// Compartment to update blame information
const blameDecorationsCompartment = new Compartment()
// Compartment for propagating component props
const blobPropsCompartment = new Compartment()

export const Blob: React.FunctionComponent<BlobProps> = props => {
    const {
        className,
        wrapCode,
        isLightTheme,
        ariaLabel,
        role,
        extensionsController,
        settingsCascade,
        location,
        history,
        blameDecorations: blameTextDocumentDecorations,

        // Reference panel specific props
        disableStatusBar,
        disableDecorations,
        disableHovercards,
    } = props

    const [container, setContainer] = useState<HTMLDivElement | null>(null)
    // This is used to avoid reinitializing the editor when new locations in the
    // same file are opened inside the reference panel.
    const blobInfo = useDistinctBlob(props.blobInfo)
    const blobIsLoading = useBlobIsLoading(blobInfo, location.pathname)
    const position = useMemo(() => parseQueryAndHash(location.search, location.hash), [location.search, location.hash])
    const hasPin = useMemo(() => urlIsPinned(location.search), [location.search])

    const blobProps = useMemo(() => blobPropsFacet.of(props), [props])

    const enableExtensionsDecorationsColumnView = enableExtensionsDecorationsColumnViewFromSettings(settingsCascade)
    const settings = useMemo(
        () => [
            wrapCode ? EditorView.lineWrapping : [],
            EditorView.darkTheme.of(isLightTheme === false),
            enableColumnView.of(enableExtensionsDecorationsColumnView),
        ],
        [wrapCode, isLightTheme, enableExtensionsDecorationsColumnView]
    )

    const blameDecorations = useMemo(
        () =>
            blameTextDocumentDecorations
                ? [
                      // Force column view if blameDecorations is set
                      enableColumnView.of(true),
                      showTextDocumentDecorations.of([[blameDecorationType, blameTextDocumentDecorations]]),
                  ]
                : [],

        [blameTextDocumentDecorations]
    )

    // Keep history and location in a ref so that we can use the latest value in
    // the onSelection callback without having to recreate it and having to
    // reconfigure the editor extensions
    const historyRef = useRef(history)
    historyRef.current = history
    const locationRef = useRef(location)
    locationRef.current = location

    const onSelection = useCallback((range: SelectedLineRange) => {
        const parameters = new URLSearchParams(locationRef.current.search)
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

        updateBrowserHistoryIfChanged(
            historyRef.current,
            locationRef.current,
            addLineRangeQueryParameter(parameters, query)
        )
    }, [])

    const extensions: Extension[] = useMemo(
        () =>
            [
                staticExtensions,
                selectableLineNumbers({ onSelection, initialSelection: position.line !== undefined ? position : null }),
                syntaxHighlight.of(blobInfo),
                pinnedRangeField.init(() => (hasPin ? position : null)),
                extensionsController !== null
                    ? sourcegraphExtensions({
                          blobInfo,
                          initialSelection: position,
                          extensionsController,
                          disableStatusBar,
                          disableDecorations,
                          disableHovercards,
                      })
                    : null,
                blobPropsCompartment.of(blobProps),
                blameDecorationsCompartment.of(blameDecorations),
                settingsCompartment.of(settings),
            ].filter(Boolean) as Extension[],
        // A couple of values are not dependencies (blameDecorations, blobProps,
        // hasPin, position and settings) because those are updated in effects
        // further below. However they are still needed here because we need to
        // set initial values when we re-initialize the editor.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [onSelection, blobInfo, extensionsController, disableStatusBar, disableDecorations]
    )

    const editor = useCodeMirror(container, blobInfo.content, extensions, {
        updateValueOnChange: false,
        updateOnExtensionChange: false,
    })

    // Reconfigure editor when blobInfo or core extensions changed
    useEffect(() => {
        if (editor) {
            // We use setState here instead of dispatching a transaction because
            // the new document has nothing to do with the previous one and so
            // any existing state should be discarded.
            editor.setState(
                EditorState.create({
                    doc: blobInfo.content,
                    extensions,
                })
            )
        }
        // editor is not provided because this should only be triggered after the
        // editor was created (i.e. not on first render)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [blobInfo, extensions])

    // Propagate props changes to extensions
    useEffect(() => {
        if (editor) {
            editor.dispatch({ effects: blobPropsCompartment.reconfigure(blobProps) })
        }
        // editor is not provided because this should only be triggered after the
        // editor was created (i.e. not on first render)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [blobProps])

    // Update blame information
    useEffect(() => {
        if (editor) {
            editor.dispatch({ effects: blameDecorationsCompartment.reconfigure(blameDecorations) })
        }
        // editor is not provided because this should only be triggered after the
        // editor was created (i.e. not on first render)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [blameDecorations])

    // Update settings
    useEffect(() => {
        if (editor) {
            editor.dispatch({ effects: settingsCompartment.reconfigure(settings) })
        }
        // editor is not provided because this should only be triggered after the
        // editor was created (i.e. not on first render)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [settings])

    // Update selected lines when URL changes
    useEffect(() => {
        if (editor && !blobIsLoading) {
            selectLines(editor, position.line ? position : null)
        }
        // editor is not provided because this should only be triggered after the
        // editor was created (i.e. not on first render)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [position, blobIsLoading])

    // Update pinned hovercard range
    useEffect(() => {
        if (editor && !blobIsLoading) {
            updatePinnedRangeField(editor, hasPin ? position : null)
        }
        // editor is not provided because this should only be triggered after the
        // editor was created (i.e. not on first render)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [position, hasPin, blobIsLoading])

    return <div ref={setContainer} aria-label={ariaLabel} role={role} className={`${className} overflow-hidden`} />
}

/**
 * Returns true when the URL indicates that the hovercard at the URL position
 * should be shown on load (the hovercard is "pinned").
 */
function urlIsPinned(search: string): boolean {
    return new URLSearchParams(search).get('popover') === 'pinned'
}

/**
 * Because the location changes before new blob info is available we often apply
 * updates to the old document, which can be problematic or throw errors. This
 * helper hook keeps track of which path is set when the blob info updates
 * and compares it against the current path.
 */
function useBlobIsLoading(blobInfo: BlobInfo, pathname: string): boolean {
    // pathname is intentionally ignored to make this functionality work
    // eslint-disable-next-line react-hooks/exhaustive-deps
    return pathname !== useMemo(() => pathname, [blobInfo])
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
 * Field used by the CodeMirror blob view to provide hovercard range information
 * for pinned cards. Since we have to use the editor's current state to compute
 * the final position we are using a field instead of a compartment to provide
 * this information.
 */
const [pinnedRangeField, updatePinnedRangeField] = createUpdateableField<LineOrPositionOrRange | null>(null, field =>
    hovercardRanges.computeN([field], state => {
        const position = state.field(field)
        if (!position) {
            return []
        }

        if (!position.line || !position.character) {
            return []
        }
        const startLine = state.doc.line(position.line)

        const startPosition = {
            line: position.line,
            character: position.character,
        }
        const from = uiPositionToOffset(state.doc, startPosition, startLine)

        let endPosition: UIPositionSpec['position']
        let to: number

        if (position.endLine && position.endCharacter) {
            endPosition = {
                line: position.endLine,
                character: position.endCharacter,
            }
            to = uiPositionToOffset(state.doc, endPosition)
        } else {
            // To determine the end position we have to find the word at the
            // start position
            const word = state.wordAt(from)
            if (!word) {
                return []
            }
            to = word.to
            endPosition = offsetToUIPosition(state.doc, word.to)
        }

        return [
            {
                to,
                from,
                range: {
                    start: startPosition,
                    end: endPosition,
                },
            },
        ]
    })
)
