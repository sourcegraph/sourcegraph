/**
 * This provides CodeMirror extension for highlighting a static set of ranges.
 */

import { type Extension, EditorState, StateField, Facet } from '@codemirror/state'
import { Decoration, EditorView, showPanel, type Panel, ViewUpdate } from '@codemirror/view'
import { sortedIndexBy } from 'lodash'

import StaticHighlightsPanelComponent from './StaticHighlightsPanel.svelte'

export interface Range {
    start: Location
    end: Location
}

export interface Location {
    // A zero-based line number
    line: number
    // A zero-based column number
    column: number
}

/**
 * staticHighlights is an extension that highlights a static set of ranges
 * and opens a panel that allows navigation between these highlights.
 */
export function staticHighlights(ranges: Range[]): Extension {
    if (ranges.length === 0) {
        return []
    }
    const facet = Facet.define<Range[], Range[]>({
        combine: ranges => ranges.flat(),
        enables: () => [
            staticHighlightTheme,
            staticHighlightState.init(state => {
                const codeMirrorRanges = ranges.map(range => ({
                    selected: false,
                    from: toCodeMirrorLocation(state, range.start),
                    to: toCodeMirrorLocation(state, range.end),
                }))
                return codeMirrorRanges.sort((a, b) => a.from - b.from)
            }),
            showPanel.of(view => new StaticHighlightsPanel(view)),
            scrollToFirstRange(),
        ],
    })
    return facet.of(ranges)
}

/**
 * scrollToFirstRange is a small utility extension that just scrolls to
 * the selected range on startup.
 */
function scrollToFirstRange(): Extension {
    let scrolled = false
    return EditorView.updateListener.of((update: ViewUpdate) => {
        if (scrolled) {
            return
        }
        const ranges = update.view.state.field(staticHighlightState)
        scrolled = true
        update.view.dispatch({
            selection: { anchor: ranges[0].from, head: ranges[0].to },
            effects: EditorView.scrollIntoView(ranges[0].from, {
                y: 'nearest',
                x: 'center',
                yMargin: update.view.dom.getBoundingClientRect().height / 3,
            }),
        })
    })
}

const staticHighlightDecoration = Decoration.mark({ class: 'cm-sg-static-highlight' })
const staticHighlightSelectedDecoration = Decoration.mark({ class: 'cm-sg-static-highlight-selected' })

export interface HighlightedRange {
    from: number
    to: number
    selected: boolean
}

const staticHighlightState = StateField.define<HighlightedRange[]>({
    create: () => [],
    update: (ranges, tr) => {
        // When the selection changes, update the selection
        const selectedRange = tr.selection?.main
        if (selectedRange === undefined) {
            return ranges
        }
        return ranges.map(range => ({
            from: range.from,
            to: range.to,
            selected: range.from === selectedRange.from && range.to === selectedRange.to,
        }))
    },
    provide: f =>
        EditorView.decorations.from(f, ranges =>
            Decoration.set(
                ranges.map(({ selected, from, to }) =>
                    selected
                        ? staticHighlightSelectedDecoration.range(from, to)
                        : staticHighlightDecoration.range(from, to)
                ),
                true
            )
        ),
})

const staticHighlightTheme = EditorView.theme({
    '.cm-sg-static-highlight': {
        backgroundColor: 'var(--mark-bg)',
    },
    '.cm-sg-static-highlight-selected': {
        backgroundColor: 'var(--oc-orange-3)',
    },
})

function toCodeMirrorLocation(state: EditorState, location: Location): number {
    // Codemirror expects 1-based line numbers
    return state.doc.line(location.line + 1).from + location.column
}

function findSelectedIdx(ranges: HighlightedRange[]): number | undefined {
    const selectedIdx = ranges.findIndex(range => range.selected)
    return selectedIdx === -1 ? undefined : selectedIdx
}

class StaticHighlightsPanel implements Panel {
    public dom: HTMLElement
    public top = true

    private root: StaticHighlightsPanelComponent

    constructor(private view: EditorView) {
        this.dom = document.createElement('div')
        this.root = new StaticHighlightsPanelComponent({
            target: this.dom,
            props: {
                ranges: this.ranges,
                handlePrevious: this.navigatePrevious.bind(this),
                handleNext: this.navigateNext.bind(this),
            },
        })
    }

    private get ranges(): HighlightedRange[] {
        return this.view.state.field(staticHighlightState)
    }

    public update(viewUpdate: ViewUpdate): void {
        const oldSelectedIdx = findSelectedIdx(viewUpdate.startState.field(staticHighlightState))
        const newSelectedIdx = findSelectedIdx(viewUpdate.state.field(staticHighlightState))
        if (newSelectedIdx !== oldSelectedIdx) {
            this.root.$set({
                ranges: this.ranges,
                handlePrevious: this.navigatePrevious.bind(this),
                handleNext: this.navigateNext.bind(this),
            })
        }
    }

    public destroy(): void {
        this.root.$destroy()
    }

    private navigateTo(target: { from: number; to: number }): void {
        this.view.dispatch({
            selection: { anchor: target.from, head: target.to },
            effects: [
                EditorView.scrollIntoView(target.from, {
                    y: 'nearest',
                    x: 'center',
                    yMargin: this.view.dom.getBoundingClientRect().height / 3,
                }),
            ],
        })
    }

    private navigatePrevious(): void {
        const currentSelection = this.view.state.selection.main
        // Find the index of the first element before or equal to the selection
        const idx = sortedIndexBy(
            this.ranges,
            { from: currentSelection.from, to: 0, selected: false },
            range => range.from
        )
        const previousRange = this.ranges[(this.ranges.length + idx - 1) % this.ranges.length]
        this.navigateTo(previousRange)
    }

    private navigateNext(): void {
        const currentSelection = this.view.state.selection.main
        // Find the index of the first element after the selection
        const idx = sortedIndexBy(
            this.ranges,
            { from: currentSelection.from + 1, to: 0, selected: false },
            range => range.from
        )
        const nextRange = this.ranges[idx % this.ranges.length]
        this.navigateTo(nextRange)
    }
}
