import { isEqual } from 'lodash'
import { fromEvent, type Observable } from 'rxjs'
import { distinctUntilChanged, map } from 'rxjs/operators'

import { LineOrPositionOrRange, SourcegraphURL } from '@sourcegraph/common'
import type { Position, Selection, Range } from '@sourcegraph/extension-api-types'

function lprToRange(lpr: LineOrPositionOrRange): Range | undefined {
    if (lpr.line === undefined) {
        return undefined
    }
    return {
        start: { line: lpr.line, character: lpr.character || 0 },
        end: {
            line: lpr.endLine || lpr.line,
            character: lpr.endCharacter || lpr.character || 0,
        },
    }
}

// `lprToRange` sets character to 0 if it's undefined. Only - 1 the character if it's not 0.
const characterZeroIndexed = (character: number): number => (character === 0 ? character : character - 1)

export function lprToSelectionsZeroIndexed(lpr: LineOrPositionOrRange): Selection[] {
    const range = lprToRange(lpr)
    if (range === undefined) {
        return []
    }
    const start: Position = { line: range.start.line - 1, character: characterZeroIndexed(range.start.character) }
    const end: Position = { line: range.end.line - 1, character: characterZeroIndexed(range.end.character) }
    return [
        {
            start,
            end,
            anchor: start,
            active: end,
            isReversed: false,
        },
    ]
}

export function getSelectionsFromHash(): Selection[] {
    return lprToSelectionsZeroIndexed(SourcegraphURL.from({ hash: window.location.hash }).lineRange)
}

export function observeSelectionsFromHash(): Observable<Selection[]> {
    return fromEvent(window, 'hashchange').pipe(
        map(getSelectionsFromHash),
        distinctUntilChanged((a, b) => isEqual(a, b))
    )
}
