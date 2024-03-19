/**
 * This provides CodeMirror extension for highlighting a static set of ranges.
 */

import type { ApolloClient } from '@apollo/client'
import { type Extension, type EditorState, StateField, Facet } from '@codemirror/state'
import { Decoration, EditorView, showPanel, type Panel, type ViewUpdate } from '@codemirror/view'
import { mdiChevronLeft, mdiChevronRight } from '@mdi/js'
import classNames from 'classnames'
import { sortedIndexBy } from 'lodash'
import { createRoot, type Root } from 'react-dom/client'
import type { NavigateFunction } from 'react-router-dom'

import { pluralize } from '@sourcegraph/common'
import { Button, Icon, Text } from '@sourcegraph/wildcard'

import { createElement } from '../../../util/dom'

import { CodeMirrorContainer } from './react-interop'

import styles from './search.module.scss'

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
export function staticHighlights(
    navigate: NavigateFunction,
    graphQLClient: ApolloClient<any>,
    ranges: Range[]
): Extension {
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
            showPanel.of(view => new StaticHighlightsPanel(view, navigate, graphQLClient)),
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

interface HighlightedRange {
    from: number
    to: number
    selected: boolean
}

const staticHighlightDecoration = Decoration.mark({ class: 'cm-sg-static-highlight' })
const staticHighlightSelectedDecoration = Decoration.mark({ class: 'cm-sg-static-highlight-selected' })

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

export const STATIC_HIGHLIGHTS_CONTAINER_ID = 'static-highlights-navigation-container'

class StaticHighlightsPanel implements Panel {
    public dom: HTMLElement
    public top = true

    private root: Root | null = null

    constructor(
        private view: EditorView,
        private navigate: NavigateFunction,
        private graphQLClient: ApolloClient<any>
    ) {
        this.dom = createElement('div', {
            className: classNames('cm-sg-search-container', styles.root),
            id: STATIC_HIGHLIGHTS_CONTAINER_ID,
        })
        this.render(this.ranges)
    }

    private get ranges(): HighlightedRange[] {
        return this.view.state.field(staticHighlightState)
    }

    public update(viewUpdate: ViewUpdate): void {
        const oldSelectedIdx = findSelectedIdx(viewUpdate.startState.field(staticHighlightState))
        const newSelectedIdx = findSelectedIdx(viewUpdate.state.field(staticHighlightState))
        if (newSelectedIdx !== oldSelectedIdx) {
            this.render(viewUpdate.state.field(staticHighlightState))
        }
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

    private render(ranges: HighlightedRange[]): void {
        if (!this.root) {
            this.root = createRoot(this.dom)
        }

        const totalMatches = ranges.length
        const selectedIdx = ranges.findIndex(range => range.selected)

        this.root.render(
            <CodeMirrorContainer navigate={this.navigate} graphQLClient={this.graphQLClient}>
                {totalMatches > 1 && (
                    <div>
                        <Button
                            className={classNames(styles.bgroupLeft, 'p-1')}
                            type="button"
                            size="sm"
                            outline={true}
                            variant="secondary"
                            onClick={this.navigatePrevious.bind(this)}
                            data-testid="blob-view-static-previous"
                            aria-label="previous result"
                        >
                            <Icon svgPath={mdiChevronLeft} aria-hidden={true} />
                        </Button>

                        <Button
                            className={classNames(styles.bgroupRight, 'p-1')}
                            type="button"
                            size="sm"
                            outline={true}
                            variant="secondary"
                            onClick={this.navigateNext.bind(this)}
                            data-testid="blob-view-static-next"
                            aria-label="next result"
                        >
                            <Icon svgPath={mdiChevronRight} aria-hidden={true} />
                        </Button>
                    </div>
                )}
                <Text className="cm-search-results m-0 small">
                    {selectedIdx === -1 ? '' : `${selectedIdx + 1} of `}
                    {totalMatches} {pluralize('result', totalMatches)}
                </Text>
            </CodeMirrorContainer>
        )
    }
}
