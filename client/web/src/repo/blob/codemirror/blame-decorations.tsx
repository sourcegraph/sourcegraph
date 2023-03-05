/**
 * This file contains CodeMirror extensions for rendering `git blame`-specific CodeMirror
 * decorations via the {@link showGitBlameDecorations} facet.
 */
import { Extension, Facet, RangeSet } from '@codemirror/state'
import {
    Decoration,
    DecorationSet,
    EditorView,
    gutter,
    gutterLineClass,
    GutterMarker,
    gutters,
    ViewPlugin,
    ViewUpdate,
    WidgetType,
} from '@codemirror/view'
import { isEqual } from 'lodash'
import { createRoot, Root } from 'react-dom/client'
import { NavigateFunction } from 'react-router-dom'

import { createUpdateableField } from '@sourcegraph/shared/src/components/CodeMirrorEditor'

import { BlameHunk, BlameHunkData } from '../../blame/useBlameHunks'
import { BlameDecoration } from '../BlameDecoration'

import { blobPropsFacet } from '.'

const highlightedLineDecoration = Decoration.line({ class: 'highlighted-line' })
const startOfHunkDecoration = Decoration.line({ class: 'border-top' })

const highlightedLineGutterMarker = new (class extends GutterMarker {
    public elementClass = 'highlighted-line'
})()

const [hoveredLine, setHoveredLine] = createUpdateableField<number | null>(null, field => [
    EditorView.decorations.compute([field], state => {
        const line = state.field(field, false) ?? null
        return line === null
            ? Decoration.none
            : Decoration.set(highlightedLineDecoration.range(state.doc.line(line).from))
    }),
    gutterLineClass.compute([field], state => {
        const line = state.field(field, false) ?? null
        return line === null
            ? RangeSet.empty
            : RangeSet.of(highlightedLineGutterMarker.range(state.doc.line(line).from))
    }),
])

class BlameDecorationWidget extends WidgetType {
    private container: HTMLElement | null = null
    private reactRoot: Root | null = null
    private state: { navigate: NavigateFunction }

    constructor(
        public view: EditorView,
        public readonly hunk: BlameHunk | undefined,
        public readonly line: number,
        // We can not access the light theme and first commit date from the view
        // props because we need the widget to re-render when it updates.
        public readonly isLightTheme: boolean,
        public readonly blameHunkMetadata: Omit<BlameHunkData, 'current'>
    ) {
        super()
        const blobProps = this.view.state.facet(blobPropsFacet)
        this.state = { navigate: blobProps.navigate }
    }

    /* eslint-disable-next-line id-length*/
    public eq(other: BlameDecorationWidget): boolean {
        return isEqual(this.hunk, other.hunk)
    }

    public toDOM(): HTMLElement {
        if (!this.container) {
            this.container = document.createElement('span')
            this.container.classList.add('blame-decoration')

            this.reactRoot = createRoot(this.container)
            this.reactRoot.render(
                <BlameDecoration
                    line={this.line ?? 0}
                    blameHunk={this.hunk}
                    navigate={this.state.navigate}
                    onSelect={this.selectRow}
                    onDeselect={this.deselectRow}
                    firstCommitDate={this.blameHunkMetadata.firstCommitDate}
                    externalURLs={this.blameHunkMetadata.externalURLs}
                    hideRecency={false}
                />
            )
        }
        return this.container
    }

    private selectRow = (line: number): void => {
        setHoveredLine(this.view, line)
    }

    private deselectRow = (line: number): void => {
        if (this.view.state.field(hoveredLine) === line) {
            setHoveredLine(this.view, null)
        }
    }

    public destroy(): void {
        this.container?.remove()
        // setTimeout seems necessary to prevent React from complaining that the
        // root is synchronously unmounted while rendering is in progress
        setTimeout(() => this.reactRoot?.unmount(), 0)
    }
}

/**
 * Facet to show git blame decorations.
 */
interface BlameDecorationsFacetProps {
    hunks: BlameHunk[]
    isLightTheme: boolean
    blameHunkMetadata: Omit<BlameHunkData, 'current'>
}
const showGitBlameDecorations = Facet.define<BlameDecorationsFacetProps, BlameDecorationsFacetProps>({
    combine: decorations => decorations[0],
    enables: facet => [
        hoveredLine,

        // Render blame hunks as line decorations.
        ViewPlugin.fromClass(
            class {
                public decorations: DecorationSet
                private previousHunkLength = -1
                private previousIsLightTheme = false
                private previousBlameHunkMetadata: Omit<BlameHunkData, 'current'> | undefined

                constructor(view: EditorView) {
                    this.decorations = this.computeDecorations(view, facet)
                }

                public update(update: ViewUpdate): void {
                    const facetProps = update.view.state.facet(facet)
                    const hunks = facetProps.hunks
                    const isLightMode = facetProps.isLightTheme
                    const blameHunkMetadata = facetProps.blameHunkMetadata

                    if (
                        update.docChanged ||
                        update.viewportChanged ||
                        this.previousHunkLength !== hunks.length ||
                        this.previousIsLightTheme !== isLightMode ||
                        this.previousBlameHunkMetadata !== blameHunkMetadata
                    ) {
                        this.decorations = this.computeDecorations(update.view, facet)
                        this.previousHunkLength = hunks.length
                        this.previousIsLightTheme = isLightMode
                        this.previousBlameHunkMetadata = blameHunkMetadata
                    }
                }

                private computeDecorations(
                    view: EditorView,
                    facet: Facet<BlameDecorationsFacetProps, BlameDecorationsFacetProps>
                ): DecorationSet {
                    const widgets = []
                    const facetProps = view.state.facet(facet)
                    const { hunks, isLightTheme, blameHunkMetadata } = facetProps

                    for (const { from, to } of view.visibleRanges) {
                        let nextHunkDecorationLineRenderedAt = -1
                        for (let position = from; position <= to; ) {
                            const line = view.state.doc.lineAt(position)
                            const matchingHunk = hunks.find(
                                hunk => line.number >= hunk.startLine && line.number < hunk.endLine
                            )

                            const isStartOfHunk = matchingHunk?.startLine === line.number
                            if (
                                (isStartOfHunk && line.number !== 1) ||
                                nextHunkDecorationLineRenderedAt === line.from
                            ) {
                                widgets.push(startOfHunkDecoration.range(line.from))

                                // When we found a hunk, we already know when the next one will start even if this
                                // hunk was not loaded yet.
                                //
                                // We mark this as rendered in `nextHunkDecorationLineRenderedAt` so that the next
                                // startLine can be skipped if it was rendered already
                                if (matchingHunk) {
                                    nextHunkDecorationLineRenderedAt = view.state.doc.line(matchingHunk.endLine).from
                                }
                            }

                            const decoration = Decoration.widget({
                                widget: new BlameDecorationWidget(
                                    view,
                                    matchingHunk,
                                    line.number,
                                    isLightTheme,
                                    blameHunkMetadata
                                ),
                            })
                            widgets.push(decoration.range(line.from))
                            position = line.to + 1
                        }
                    }
                    return Decoration.set(widgets)
                }
            },
            {
                decorations: ({ decorations }) => decorations,
            }
        ),
        EditorView.theme({
            '.cm-line': {
                // Position relative so that the blame-decoration inside can be
                // aligned to the start of the line
                position: 'relative',
                // Move the start of the line to after the blame decoration.
                // This is necessary because the start of the line is used for
                // aligning tab characters.
                paddingLeft: 'var(--blame-decoration-width) !important',
            },
            '.blame-decoration': {
                // Remove the blame decoration from the content flow so that
                // the tab start can be moved to the right
                position: 'absolute',
                left: '0',
                height: '100%',
                display: 'inline-block',
                background: 'var(--body-bg)',
                verticalAlign: 'bottom',
                width: 'var(--blame-decoration-width)',
            },

            '.selected-line .blame-decoration, .highlighted-line .blame-decoration': {
                background: 'inherit',
            },

            '.cm-content': {
                // Make .cm-content overflow .blame-gutter
                marginLeft: 'calc(var(--blame-decoration-width) * -1)',
                // override default .cm-gutters z-index 200
                zIndex: 201,
            },
        }),
    ],
})

const blameGutterElement = new (class extends GutterMarker {
    public toDOM(): HTMLElement {
        return document.createElement('div')
    }
})()

const showBlameGutter = Facet.define<boolean>({
    combine: value => value.flat(),
    enables: [
        // Render gutter with no content only to create a column with specified background.
        // This column is used by .cm-content shifted to the left by var(--blame-decoration-width)
        // to achieve column-like view of inline blame decorations.
        gutter({
            class: 'blame-gutter',
            lineMarker: () => blameGutterElement,
            initialSpacer: () => blameGutterElement,
        }),

        // By default, gutters are fixed, meaning they don't scroll along with the content horizontally (position: sticky).
        // We override this behavior when blame decorations are shown to make inline decorations column-like view work.
        gutters({ fixed: false }),

        EditorView.theme({
            '.blame-gutter': {
                background: 'var(--body-bg)',
                width: 'var(--blame-decoration-width)',
            },
        }),
    ],
})

function blameLineStyles({ isBlameVisible }: { isBlameVisible: boolean }): Extension {
    return EditorView.theme({
        '.cm-line': {
            lineHeight: isBlameVisible ? '1.5rem' : '1rem',
            // Avoid jumping when blame decorations are streamed in because we use a border
            borderTop: isBlameVisible ? '1px solid transparent' : 'none',
        },
    })
}

export const createBlameDecorationsExtension = (
    isBlameVisible: boolean,
    blameHunks: BlameHunkData | undefined,
    isLightTheme: boolean
): Extension => [
    blameLineStyles({ isBlameVisible }),
    isBlameVisible ? showBlameGutter.of(isBlameVisible) : [],
    blameHunks?.current
        ? showGitBlameDecorations.of({
              hunks: blameHunks.current,
              isLightTheme,
              blameHunkMetadata: {
                  firstCommitDate: blameHunks.firstCommitDate,
                  externalURLs: blameHunks.externalURLs,
              },
          })
        : [],
]
