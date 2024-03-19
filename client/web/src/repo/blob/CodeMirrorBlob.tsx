/**
 * An implementation of the Blob view using CodeMirror
 */

import { useCallback, useEffect, useMemo, useRef, useState, type MutableRefObject, type RefObject } from 'react'

import { useApolloClient } from '@apollo/client'
import { openSearchPanel } from '@codemirror/search'
import { EditorState, type Extension } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { createClient, type Annotation } from '@opencodegraph/client'
import { useOpenCodeGraphExtension } from '@opencodegraph/codemirror-extension'
import { isEqual } from 'lodash'
import { createRoot } from 'react-dom/client'
import { createPath, useLocation, useNavigate, type Location, type NavigateFunction } from 'react-router-dom'

import { NoopEditor } from '@sourcegraph/cody-shared/dist/editor'
import {
    addLineRangeQueryParameter,
    formatSearchParameters,
    toPositionOrRangeQueryParameter,
} from '@sourcegraph/common'
import { getOrCreateCodeIntelAPI, type CodeIntelAPI } from '@sourcegraph/shared/src/codeintel/api'
import { editorHeight, useCodeMirror, useCompartment } from '@sourcegraph/shared/src/components/CodeMirrorEditor'
import { useKeyboardShortcut } from '@sourcegraph/shared/src/keyboardShortcuts/useKeyboardShortcut'
import type { PlatformContext, PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TemporarySettingsSchema } from '@sourcegraph/shared/src/settings/temporary/TemporarySettings'
import { type TelemetryV2Props, noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { codeCopiedEvent } from '@sourcegraph/shared/src/tracking/event-log-creators'
import {
    parseQueryAndHash,
    toPrettyBlobURL,
    type AbsoluteRepoFile,
    type BlobViewState,
    type ModeSpec,
} from '@sourcegraph/shared/src/util/url'
import { useLocalStorage } from '@sourcegraph/wildcard'

import { CodeMirrorEditor } from '../../cody/components/CodeMirrorEditor'
import { isCodyEnabled } from '../../cody/isCodyEnabled'
import { useCodySidebar } from '../../cody/sidebar/Provider'
import { useFeatureFlag } from '../../featureFlags/useFeatureFlag'
import type { ExternalLinkFields, Scalars } from '../../graphql-operations'
import type { BlameHunkData } from '../blame/useBlameHunks'
import type { HoverThresholdProps } from '../RepoContainer'

import { BlameDecoration } from './BlameDecoration'
import { blobPropsFacet } from './codemirror'
import { blameData, showBlame } from './codemirror/blame-decorations'
import { codeFoldingExtension } from './codemirror/code-folding'
import { createCodeIntelExtension } from './codemirror/codeintel/extension'
import { pinnedLocation } from './codemirror/codeintel/pin'
import { syncSelection } from './codemirror/codeintel/token-selection'
import { hideEmptyLastLine } from './codemirror/eof'
import { syntaxHighlight } from './codemirror/highlight'
import { selectableLineNumbers, selectLines, type SelectedLineRange } from './codemirror/linenumbers'
import { linkify } from './codemirror/links'
import { lockFirstVisibleLine } from './codemirror/lock-line'
import { navigateToLineOnAnyClickExtension } from './codemirror/navigate-to-any-line-on-click'
import { CodeMirrorContainer } from './codemirror/react-interop'
import { scipSnapshot } from './codemirror/scip-snapshot'
import { search, type SearchPanelConfig } from './codemirror/search'
import { staticHighlights, type Range } from './codemirror/static-highlights'
import { codyWidgetExtension } from './codemirror/tooltips/CodyTooltip'
import { HovercardView } from './codemirror/tooltips/HovercardView'
import { showTemporaryTooltip, temporaryTooltip } from './codemirror/tooltips/TemporaryTooltip'
import { locationToURL, positionToOffset } from './codemirror/utils'
import { setBlobEditView } from './use-blob-store'

/**
 * The minimum number of milliseconds that must elapse before we handle a "Go to
 * definition request".  The motivation to impose a minimum latency on this
 * action is to give the user feedback that something happened if they rapidly
 * trigger "Go to definition" from the same location and the destination token
 * is already visible in the viewport.  Without this minimum latency, the user
 * gets no feedback that the destination is visible.  With this latency, the
 * source token (where the user clicks) gets briefly focused before the focus
 * moves back to the destination token. This small wiggle in the focus state
 * makes it easier to find the destination token.
 */
const MINIMUM_GO_TO_DEF_LATENCY_MILLIS = 20

// Logical grouping of props that are only used by the CodeMirror blob view
// implementation.
interface CodeMirrorBlobProps {
    overrideBrowserSearchKeybinding?: boolean
}

export interface BlobProps
    extends SettingsCascadeProps,
        PlatformContextProps,
        TelemetryProps,
        TelemetryV2Props,
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

    ocgVisibility?: TemporarySettingsSchema['openCodeGraph.annotations.visible']

    activeURL?: string
    searchPanelConfig?: SearchPanelConfig
    staticHighlightRanges?: Range[]
}

export interface BlobPropsFacet extends BlobProps {
    navigate: NavigateFunction
    location: Location
}

export interface BlobInfo extends AbsoluteRepoFile, ModeSpec {
    /** The raw content of the blob. */
    content: string

    /** LSIF syntax-highlighting data */
    lsif?: string

    /** Potential language(s) for content as determined by the backend. */
    languages: string[]

    /** If present, the file is stored in Git LFS (large file storage). */
    lfs?: { byteSize: Scalars['BigInt'] } | null

    /** External URLs for the file */
    externalURLs?: ExternalLinkFields[]

    snapshotData?: { offset: number; data: string; additional: string[] | null }[] | null
}

const staticExtensions: Extension = [
    // Log uncaught errors that happen in callbacks that we pass to
    // CodeMirror. Without this exception sink, exceptions get silently
    // ignored making it difficult to debug issues caused by uncaught
    // exceptions.
    // eslint-disable-next-line no-console
    EditorView.exceptionSink.of(exception => console.log(exception)),
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

            '&:focus': {
                boxShadow: 'none',
            },
        },
        '.highlighted-line': {
            backgroundColor: 'var(--code-selection-bg)',
        },
        '.cm-panels-top': {
            borderBottom: '1px solid var(--border-color)',
        },
    }),
    hideEmptyLastLine,
    linkify,
]

export const CodeMirrorBlob: React.FunctionComponent<BlobProps> = props => {
    const {
        className,
        wrapCode,
        ariaLabel,
        role,
        isBlameVisible,
        blameHunks,
        ocgVisibility,

        // Reference panel specific props
        navigateToLineOnAnyClick,

        overrideBrowserSearchKeybinding,
        searchPanelConfig,
        staticHighlightRanges,
        'data-testid': dataTestId,
        telemetryService,
    } = props

    const apolloClient = useApolloClient()
    const navigate = useNavigate()
    const location = useLocation()
    const isLightTheme = useIsLightTheme()

    const [enableBlobPageSwitchAreasShortcuts] = useFeatureFlag('blob-page-switch-areas-shortcuts')
    const focusCodeEditorShortcut = useKeyboardShortcut('focusCodeEditor')

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

    // Keep history and location in a ref so that we can use the latest value in
    // the onSelection callback without having to recreate it and having to
    // reconfigure the editor extensions
    const locationRef = useMutableValue(location)
    const positionRef = useMutableValue(position)

    const navigateOnClick = useMemo(
        () =>
            navigateToLineOnAnyClick
                ? (line: number) =>
                      navigate(
                          toPrettyBlobURL({
                              repoName: blobInfo.repoName,
                              filePath: blobInfo.filePath,
                              revision: blobInfo.revision,
                              commitID: blobInfo.commitID,
                              position: { line, character: 0 },
                          })
                      )
                : undefined,
        [navigateToLineOnAnyClick, navigate, blobInfo.repoName, blobInfo.filePath, blobInfo.revision, blobInfo.commitID]
    )

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
                updateBrowserHistoryIfChanged(navigate, locationRef.current, newSearchParameters)
            }
        },
        [customHistoryAction, locationRef, navigate]
    )

    // Added fallback to take care of ReferencesPanel/Simple storybook
    const { setEditorScope } = useCodySidebar()

    const editorRef = useRef<EditorView | null>(null)

    const blameDecorations = useBlameDecoration(editorRef, { visible: !!isBlameVisible, blameHunks })
    const blobProps = useCompartment(
        editorRef,
        useMemo(
            () =>
                blobPropsFacet.of({
                    ...props,
                    navigate,
                    location,
                }),
            [props, navigate, location]
        )
    )
    const wrapCodeSettings = useCompartment(
        editorRef,
        useMemo<Extension>(() => (wrapCode ? EditorView.lineWrapping : []), [wrapCode])
    )
    const codeIntelExtension = useCodeIntelExtension(
        props.platformContext,
        {
            repoName: blobInfo.repoName,
            filePath: blobInfo.filePath,
            commitID: blobInfo.commitID,
            revision: blobInfo.revision,
            languages: blobInfo.languages,
        },
        blobInfo.mode
    )
    const pinnedTooltip = useCompartment(
        editorRef,
        useMemo(() => pinnedLocation.of(hasPin ? position : null), [hasPin, position])
    )

    const openCodeGraphExtension = useOpenCodeGraphExtensionWithHardcodedConfig(
        blobInfo.filePath,
        blobInfo.content,
        Boolean(ocgVisibility)
    )

    const themeExtension = useCompartment(
        editorRef,
        useMemo(() => EditorView.darkTheme.of(!isLightTheme), [isLightTheme])
    )

    const extensions = useMemo(
        () => [
            staticExtensions,
            staticHighlights(navigate, apolloClient, staticHighlightRanges ?? []),
            selectableLineNumbers({
                onSelection,
                initialSelection: position.line !== undefined ? position : null,
                onLineClick: navigateOnClick,
            }),
            scipSnapshot(blobInfo.content, blobInfo.snapshotData),
            openCodeGraphExtension,
            codeFoldingExtension(),
            isCodyEnabled()
                ? codyWidgetExtension(
                      // TODO: replace with real telemetryRecorder
                      noOpTelemetryRecorder,
                      editorRef.current
                          ? new CodeMirrorEditor({
                                view: editorRef.current,
                                repo: props.blobInfo.repoName,
                                revision: props.blobInfo.revision,
                                filename: props.blobInfo.filePath,
                                content: props.blobInfo.content,
                            })
                          : undefined
                  )
                : [],
            pinnedTooltip,
            navigateToLineOnAnyClick ? navigateToLineOnAnyClickExtension(navigate) : codeIntelExtension,
            syntaxHighlight.of(blobInfo),
            blobProps,
            blameDecorations,
            wrapCodeSettings,
            search({
                // useFileSearch is not a dependency because the search
                // extension manages its own state. This is just the initial
                // value
                overrideBrowserFindInPageShortcut: useFileSearch,
                onOverrideBrowserFindInPageToggle: setUseFileSearch,
                initialState: searchPanelConfig,
                graphQLClient: apolloClient,
                navigate,
            }),
            themeExtension,
        ],
        // A couple of values are not dependencies (hasPin and position) because those are updated in effects
        // further below. However, they are still needed here because we need to
        // set initial values when we re-initialize the editor.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [
            onSelection,
            staticHighlightRanges,
            navigate,
            blobInfo,
            isCodyEnabled,
            openCodeGraphExtension,
            codeIntelExtension,
            editorRef.current,
            blameDecorations,
            wrapCodeSettings,
            blobProps,
            pinnedTooltip,
            themeExtension,
        ]
    )

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
            syncSelection(editor, positionRef.current)

            // Automatically focus the content DOM to enable keyboard
            // navigation. Without this automatic focus, users need to click
            // on the blob view with the mouse.
            // NOTE: this focus statment does not seem to have an effect
            // when using macOS VoiceOver.
            editor.contentDOM.focus({ preventScroll: true })
        }
    }, [blobInfo.content, extensions, navigateToLineOnAnyClick, positionRef])

    // Update selected lines when URL changes
    useEffect(() => {
        const editor = editorRef.current
        if (editor) {
            selectLines(editor, position.line ? position : null)
        }
    }, [position])

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

    // Sync editor selection/focus with the URL so that triggering
    // `history.goBack/goForward()` works similar to the "Go back"
    // command in VS Code.
    useEffect(() => {
        const view = editorRef.current
        if (view) {
            syncSelection(view, positionRef.current)
        }
    }, [position, positionRef])

    // Sync editor store with global Zustand store API
    useEffect(() => setBlobEditView(editorRef.current ?? null), [])

    const openSearch = useCallback(() => {
        if (editorRef.current) {
            openSearchPanel(editorRef.current)
        }
    }, [])

    // Sync the currently viewed document with the editor zustand store. This is used for features
    // like Cody to know what file and range a user is looking at.
    useEffect(() => {
        const view = editorRef.current
        setEditorScope(
            new CodeMirrorEditor(
                view
                    ? {
                          view,
                          repo: props.blobInfo.repoName,
                          revision: props.blobInfo.revision,
                          filename: props.blobInfo.filePath,
                          content: props.blobInfo.content,
                      }
                    : undefined
            )
        )
        return () => setEditorScope(new NoopEditor())
    }, [
        props.blobInfo.content,
        props.blobInfo.filePath,
        props.blobInfo.repoName,
        props.blobInfo.revision,
        setEditorScope,
    ])

    const logEventOnCopy = useCallback(() => {
        telemetryService.log(...codeCopiedEvent('blob-view'))
    }, [telemetryService])

    return (
        <>
            <div
                ref={containerRef}
                aria-label={ariaLabel}
                role={role}
                data-testid={dataTestId}
                className={`${className} overflow-hidden test-editor`}
                data-editor="codemirror6"
                onCopy={logEventOnCopy}
            />
            {overrideBrowserSearchKeybinding && useFileSearch && (
                <Shortcut ordered={['f']} held={['Mod']} onMatch={openSearch} ignoreInput={true} />
            )}
            {enableBlobPageSwitchAreasShortcuts &&
                focusCodeEditorShortcut?.keybindings.map((keybinding, index) => (
                    <Shortcut
                        key={index}
                        {...keybinding}
                        allowDefault={true}
                        onMatch={() => {
                            editorRef.current?.contentDOM.focus()
                        }}
                    />
                ))}
        </>
    )
}

function useCodeIntelExtension(
    context: PlatformContext,
    {
        repoName,
        filePath,
        commitID,
        revision,
        languages,
    }: { repoName: string; filePath: string; commitID: string; revision: string; languages: string[] },
    mode: string
): Extension {
    const navigate = useNavigate()
    const location = useLocation()
    const apolloClient = useApolloClient()
    const locationRef = useRef(location)
    const [api, setApi] = useState<CodeIntelAPI | null>(null)

    useEffect(() => {
        locationRef.current = location
    }, [location])

    useEffect(() => {
        let ignore = false
        void getOrCreateCodeIntelAPI(context).then(api => {
            if (!ignore) {
                setApi(api)
            }
        })
        return () => {
            ignore = true
        }
    }, [context])

    return useMemo(
        () => [
            temporaryTooltip,
            api
                ? createCodeIntelExtension({
                      api: {
                          api,
                          documentInfo: { repoName, filePath, commitID, revision, languages },
                          createTooltipView: ({ view, token, hovercardData }) =>
                              new HovercardView(view, token, hovercardData, apolloClient),
                          openImplementations(_view, documentInfo, occurrence) {
                              navigate(
                                  toPrettyBlobURL({
                                      ...documentInfo,
                                      range: occurrence.range.withIncrementedValues(),
                                      viewState: `implementations_${mode}` as BlobViewState,
                                  })
                              )
                          },
                          openReferences(_view, documentInfo, occurrence) {
                              navigate(
                                  toPrettyBlobURL({
                                      ...documentInfo,
                                      range: occurrence.range.withIncrementedValues(),
                                      viewState: 'references',
                                  })
                              )
                          },
                          goToDefinition(view, definition, options) {
                              const documentInfo = { repoName, filePath, commitID, revision }
                              const goto = options?.newWindow
                                  ? (url: string, _options?: unknown) => window.open(url, '_blank')
                                  : navigate

                              switch (definition.type) {
                                  case 'none': {
                                      const offset = positionToOffset(view.state.doc, definition.occurrence.range.start)
                                      if (offset) {
                                          showTemporaryTooltip(view, 'No definition found', offset, 2000)
                                      }
                                      break
                                  }
                                  case 'at-definition': {
                                      const offset = positionToOffset(view.state.doc, definition.occurrence.range.start)
                                      if (offset) {
                                          showTemporaryTooltip(view, 'You are at the definition', offset, 2000)
                                      }

                                      // Open reference panel
                                      goto(locationToURL(documentInfo, definition.from, 'references'), {
                                          replace: true,
                                      })
                                      break
                                  }
                                  case 'single': {
                                      interface DefinitionState {
                                          // The destination URL if we trigger `history.goBack()`.  We use this state
                                          // to avoid inserting redundant 'A->B->A->B' entries when the user triggers
                                          // "go to definition" twice in a row from the same location.
                                          previousURL?: string
                                      }

                                      const locationState = locationRef.current.state as DefinitionState
                                      const hrefFrom = locationToURL(documentInfo, definition.from)
                                      // Don't push URLs into the history if the last goto-def
                                      // action was from the same URL same as this action. This
                                      // happens when the user repeatedly triggers goto-def, which
                                      // is easy to do when the definition URL is close to
                                      // where the action got triggered.
                                      const shouldPushHistory = locationState?.previousURL !== hrefFrom
                                      // Add browser history entry for reference location. This allows users
                                      // to easily jump back to the location they triggered 'go to definition'
                                      // from. Additionally this
                                      navigate(hrefFrom, {
                                          replace: !shouldPushHistory || createPath(locationRef.current) === hrefFrom,
                                      })

                                      setTimeout(() => {
                                          goto(locationToURL(documentInfo, definition.destination), {
                                              replace: !shouldPushHistory,
                                              state: { previousURL: hrefFrom },
                                          })
                                      }, MINIMUM_GO_TO_DEF_LATENCY_MILLIS)
                                      break
                                  }
                                  case 'multiple': {
                                      // Linking to the reference panel is a temporary workaround until we
                                      // implement a component to resolve ambiguous results inside the blob
                                      // view similar to how VS Code "Peek definition" works like.
                                      goto(locationToURL(documentInfo, definition.destination, 'def'))
                                      break
                                  }
                              }
                          },
                      },
                      pin: {
                          onPin(position) {
                              const search = new URLSearchParams(locationRef.current.search)
                              search.set('popover', 'pinned')

                              updateBrowserHistoryIfChanged(
                                  navigate,
                                  locationRef.current,
                                  // It may seem strange to set start and end to the same value, but that what's the old blob view is doing as well
                                  addLineRangeQueryParameter(
                                      search,
                                      toPositionOrRangeQueryParameter({
                                          position,
                                          range: { start: position, end: position },
                                      })
                                  )
                              )
                              void navigator.clipboard.writeText(window.location.href)
                          },
                          onUnpin() {
                              const parameters = new URLSearchParams(locationRef.current.search)
                              parameters.delete('popover')

                              updateBrowserHistoryIfChanged(navigate, locationRef.current, parameters)
                          },
                      },
                      navigate,
                  })
                : [],
        ],
        [repoName, filePath, commitID, revision, mode, api, navigate, locationRef, languages, apolloClient]
    )
}

/**
 * Create and update blame decorations.
 */
function useBlameDecoration(
    editorRef: RefObject<EditorView>,
    { visible, blameHunks }: { visible: boolean; blameHunks?: BlameHunkData }
): Extension {
    const navigate = useNavigate()
    const apolloClient = useApolloClient()

    // Blame support is split into two compartments because we only want to trigger
    // `lockFirstVisibleLine` when blame is enabled, not when data is received
    // (this can cause the editor to scroll to a different line)
    const enabled = useCompartment(
        editorRef,
        useMemo(
            () =>
                visible
                    ? showBlame({
                          createBlameDecoration(container, { line, hunk, onSelect, onDeselect, externalURLs }) {
                              const root = createRoot(container)
                              root.render(
                                  <CodeMirrorContainer navigate={navigate} graphQLClient={apolloClient}>
                                      <BlameDecoration
                                          line={line ?? 0}
                                          blameHunk={hunk}
                                          onSelect={onSelect}
                                          onDeselect={onDeselect}
                                          externalURLs={externalURLs}
                                      />
                                  </CodeMirrorContainer>
                              )
                              return {
                                  destroy() {
                                      root.unmount()
                                  },
                              }
                          },
                      })
                    : [],
            [visible, navigate, apolloClient]
        ),
        lockFirstVisibleLine
    )

    const data = useCompartment(
        editorRef,
        useMemo(() => blameData(blameHunks), [blameHunks])
    )
    return useMemo(() => [enabled, data], [enabled, data])
}

function useOpenCodeGraphExtensionWithHardcodedConfig(
    filePath: string,
    content: string,
    visibility: boolean
): Extension {
    const client = useMemo(
        () =>
            createClient({
                configuration: () =>
                    Promise.resolve({
                        enable: true,
                        providers: { [`${window.location.origin}/.api/opencodegraph`]: true },
                    }),
                authInfo: async () => Promise.resolve(null),
                makeRange: r => r,
            }),
        []
    )

    const [annotations, setAnnotations] = useState<Annotation[]>()
    useEffect(() => {
        setAnnotations(undefined)
        if (!content || !visibility) {
            return
        }
        const subscription = client.annotations({ file: `sourcegraph:///${filePath}`, content }).subscribe({
            next: setAnnotations,
            error: (error: any) => {
                // eslint-disable-next-line no-console
                console.error('Error getting OpenCodeGraph annotations:', error)
            },
        })
        return () => subscription.unsubscribe()
    }, [content, visibility, filePath, client])

    const openCodeGraphExtension = useOpenCodeGraphExtension({ visibility, annotations })

    const theme = useMemo(
        () =>
            EditorView.baseTheme({
                '.ocg-chip': {
                    fontSize: '94% !important',
                },
            }),
        []
    )

    return useMemo(() => [openCodeGraphExtension, theme], [openCodeGraphExtension, theme])
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

function useMutableValue<T>(value: T): Readonly<MutableRefObject<T>> {
    const valueRef = useRef(value)

    useEffect(() => {
        valueRef.current = value
    }, [value])

    return valueRef
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
