import { from, fromEvent, merge, type Observable } from 'rxjs'
import { filter, map, switchMap, tap } from 'rxjs/operators'

import { convertCodeElementIdempotent, type DOMFunctions, type HoveredToken, locateTarget } from './tokenPosition'
import { isPosition } from './types'

export type SupportedMouseEvent = 'click' | 'mousemove' | 'mouseover'

export interface PositionEvent {
    /**
     * The event object
     */
    event: MouseEvent
    /**
     * The type of mouse event that caused this to emit.
     */
    eventType: SupportedMouseEvent
    /**
     * The position of the token that the event occured at.
     * or undefined when a target was hovered/clicked that does not correspond to a position (e.g. after the end of the line)
     */
    position: HoveredToken | undefined
    /**
     * The current code view.
     */
    codeView: HTMLElement
}

export const findPositionsFromEvents =
    ({ domFunctions, tokenize = true }: { domFunctions: DOMFunctions; tokenize?: boolean }) =>
    (elements: Observable<HTMLElement>): Observable<PositionEvent> =>
        merge(
            from(elements).pipe(
                switchMap(element =>
                    merge(fromEvent<MouseEvent>(element, 'mouseover'), fromEvent<MouseEvent>(element, 'mousemove'))
                ),
                map(event => ({ event, eventType: event.type as 'mouseover' | 'mousemove' })),
                filter(({ event }) => event.currentTarget !== null),
                map(({ event, ...rest }) => ({
                    event,
                    target: event.target as HTMLElement,
                    codeView: event.currentTarget as HTMLElement,
                    ...rest,
                })),
                // SIDE EFFECT (but idempotent)
                // If not done for this cell, wrap the tokens in this cell to enable finding the precise positioning.
                // This may be possible in other ways (looking at mouse position and rendering characters), but it works
                tap(({ target }) => {
                    if (!tokenize) {
                        return
                    }
                    const code = domFunctions.getCodeElementFromTarget(target)
                    if (code) {
                        convertCodeElementIdempotent(code)
                    }
                })
            )
        ).pipe(
            // Find out the position that was hovered over
            map(({ target, codeView, ...rest }) => {
                const hoveredToken = locateTarget(target, domFunctions)
                const position = isPosition(hoveredToken) ? hoveredToken : undefined
                return { position, codeView, ...rest }
            })
        )
