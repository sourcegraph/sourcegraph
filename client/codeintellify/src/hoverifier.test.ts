import { isEqual } from 'lodash'
import { EMPTY, firstValueFrom, lastValueFrom, NEVER, of, Subject, Subscription } from 'rxjs'
import { delay, distinctUntilChanged, filter, map, takeWhile } from 'rxjs/operators'
import { TestScheduler } from 'rxjs/testing'
import { afterEach, beforeEach, describe, it, expect } from 'vitest'

import { isDefined } from '@sourcegraph/common'
import type { Range } from '@sourcegraph/extension-api-types'

import { propertyIsDefined } from './helpers'
import {
    AdjustmentDirection,
    createHoverifier,
    LOADER_DELAY,
    MOUSEOVER_DELAY,
    type PositionAdjuster,
    type PositionJump,
    TOOLTIP_DISPLAY_DELAY,
} from './hoverifier'
import { findPositionsFromEvents, type SupportedMouseEvent } from './positions'
import { type CodeViewProps, DOM } from './testutils/dom'
import {
    createStubActionsProvider,
    createStubHoverProvider,
    createStubDocumentHighlightProvider,
} from './testutils/fixtures'
import { dispatchMouseEventAtPositionImpure } from './testutils/mouse'

describe('Hoverifier', () => {
    let dom: DOM
    let testcases: CodeViewProps[] = []

    beforeEach(() => {
        dom = new DOM()
        testcases = dom.createCodeViews()
    })

    let subscriptions = new Subscription()

    afterEach(() => {
        dom.cleanup()
        subscriptions.unsubscribe()
        subscriptions = new Subscription()
    })

    it('highlights token when hover is fetched (not before)', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => expect(a).toEqual(b))

            const delayTime = 100
            const hoverRange = { start: { line: 1, character: 2 }, end: { line: 3, character: 4 } }
            const hoverRange1Indexed = { start: { line: 2, character: 3 }, end: { line: 4, character: 5 } }

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider({ range: hoverRange }, LOADER_DELAY + delayTime),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                })

                const positionJumps = new Subject<PositionJump>()

                const positionEvents = of(codeView.codeView).pipe(findPositionsFromEvents({ domFunctions: codeView }))

                const subscriptions = new Subscription()

                subscriptions.add(hoverifier)
                subscriptions.add(
                    hoverifier.hoverify({
                        dom: codeView,
                        positionEvents,
                        positionJumps,
                        resolveContext: () => codeView.revSpec,
                    })
                )

                const highlightedRangeUpdates = hoverifier.hoverStateUpdates.pipe(
                    map(hoverOverlayProps => (hoverOverlayProps ? hoverOverlayProps.highlightedRange : null)),
                    distinctUntilChanged((a, b) => isEqual(a, b))
                )

                const inputDiagram = 'a'

                const outputDiagram = `${MOUSEOVER_DELAY}ms a ${LOADER_DELAY + delayTime - 1}ms b`

                const outputValues: {
                    [key: string]: Range | undefined
                } = {
                    a: undefined, // highlightedRange is undefined when the hover is loading
                    b: hoverRange1Indexed,
                }

                // Hover over https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold(inputDiagram).subscribe(() =>
                    dispatchMouseEventAtPositionImpure('mouseover', codeView, {
                        line: 24,
                        character: 6,
                    })
                )

                expectObservable(highlightedRangeUpdates).toBe(outputDiagram, outputValues)
            })
        }
    })

    it('emits the currently hovered token', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => expect(a).toEqual(b))

            const hover = {}
            const delayTime = 10

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover, delayTime),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: createStubActionsProvider(['foo', 'bar'], delayTime),
                })

                const positionJumps = new Subject<PositionJump>()

                const positionEvents = of(codeView.codeView).pipe(findPositionsFromEvents({ domFunctions: codeView }))

                const subscriptions = new Subscription()

                subscriptions.add(hoverifier)
                subscriptions.add(
                    hoverifier.hoverify({
                        dom: codeView,
                        positionEvents,
                        positionJumps,
                        resolveContext: () => codeView.revSpec,
                    })
                )

                const hoverAndDefinitionUpdates = hoverifier.hoverStateUpdates.pipe(
                    map(hoverState => hoverState.hoveredTokenElement?.textContent),
                    distinctUntilChanged(isEqual)
                )

                const outputDiagram = `${MOUSEOVER_DELAY}ms a ${delayTime - 1}ms b`
                const outputValues: {
                    [key: string]: string | undefined
                } = {
                    a: undefined,
                    b: 'Router',
                }

                // Mouseover https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold('a').subscribe(() =>
                    dispatchMouseEventAtPositionImpure('mouseover', codeView, {
                        line: 48,
                        character: 10,
                    })
                )

                expectObservable(hoverAndDefinitionUpdates).toBe(outputDiagram, outputValues)
            })
        }
    })

    it('highlights document highlights', async () => {
        for (const codeViewProps of testcases) {
            const hoverifier = createHoverifier({
                hoverOverlayElements: of(null),
                hoverOverlayRerenders: EMPTY,
                getHover: createStubHoverProvider(),
                getDocumentHighlights: createStubDocumentHighlightProvider([
                    { range: { start: { line: 24, character: 9 }, end: { line: 4, character: 15 } } },
                    { range: { start: { line: 45, character: 5 }, end: { line: 45, character: 11 } } },
                    { range: { start: { line: 120, character: 9 }, end: { line: 120, character: 15 } } },
                ]),
                getActions: () => of(null),
                documentHighlightClassName: 'test-highlight',
            })
            const positionJumps = new Subject<PositionJump>()
            const positionEvents = of(codeViewProps.codeView).pipe(
                findPositionsFromEvents({ domFunctions: codeViewProps })
            )

            hoverifier.hoverify({
                dom: codeViewProps,
                positionEvents,
                positionJumps,
                resolveContext: () => codeViewProps.revSpec,
            })

            dispatchMouseEventAtPositionImpure('mouseover', codeViewProps, {
                line: 24,
                character: 6,
            })

            await firstValueFrom(hoverifier.hoverStateUpdates.pipe(filter(state => !!state.hoverOverlayProps)))

            await new Promise(resolve => setTimeout(resolve, 200))

            const selected = codeViewProps.codeView.querySelectorAll('.test-highlight')
            expect(selected.length).toEqual(3)
            for (const element of selected) {
                expect(element).toHaveTextContent('Router')
            }
        }
    })

    // jsdom does not support `scrollIntoView()` because it doesn't do layout.
    // https://github.com/jsdom/jsdom/issues/1695
    it.skip('hides the hover overlay when the hovered token intersects with a scrollBoundary', async () => {
        const gitHubCodeView = testcases[1]
        const hoverifier = createHoverifier({
            hoverOverlayElements: of(null),
            hoverOverlayRerenders: EMPTY,
            getHover: createStubHoverProvider({
                range: {
                    start: { line: 4, character: 9 },
                    end: { line: 4, character: 9 },
                },
            }),
            getDocumentHighlights: createStubDocumentHighlightProvider(),
            getActions: createStubActionsProvider(['foo', 'bar']),
        })
        subscriptions.add(hoverifier)
        subscriptions.add(
            hoverifier.hoverify({
                dom: gitHubCodeView,
                positionEvents: of(gitHubCodeView.codeView).pipe(
                    findPositionsFromEvents({ domFunctions: gitHubCodeView })
                ),
                positionJumps: new Subject<PositionJump>(),
                resolveContext: () => gitHubCodeView.revSpec,
                scrollBoundaries: [gitHubCodeView.codeView.querySelector<HTMLElement>('.sticky-file-header')!],
            })
        )

        gitHubCodeView.codeView.scrollIntoView()

        // Click https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L5:9
        // and wait for the hovered token to be defined.
        const hasHoveredToken = lastValueFrom(
            hoverifier.hoverStateUpdates.pipe(takeWhile(({ hoveredTokenElement }) => !isDefined(hoveredTokenElement))),
            { defaultValue: null }
        )
        dispatchMouseEventAtPositionImpure('click', gitHubCodeView, {
            line: 5,
            character: 9,
        })
        await hasHoveredToken

        // Scroll down: the hover overlay should get hidden.
        const hoverIsHidden = lastValueFrom(
            hoverifier.hoverStateUpdates.pipe(takeWhile(({ hoverOverlayProps }) => isDefined(hoverOverlayProps))),
            { defaultValue: null }
        )
        gitHubCodeView.getCodeElementFromLineNumber(gitHubCodeView.codeView, 2)!.scrollIntoView({ behavior: 'smooth' })
        await hoverIsHidden
    })

    it('debounces mousemove events before showing overlay', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => expect(a).toEqual(b))

            const hover = {}

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                })

                const positionJumps = new Subject<PositionJump>()

                const positionEvents = of(codeView.codeView).pipe(findPositionsFromEvents({ domFunctions: codeView }))

                const subscriptions = new Subscription()

                subscriptions.add(hoverifier)
                subscriptions.add(
                    hoverifier.hoverify({
                        dom: codeView,
                        positionEvents,
                        positionJumps,
                        resolveContext: () => codeView.revSpec,
                    })
                )

                const hoverAndDefinitionUpdates = hoverifier.hoverStateUpdates.pipe(
                    filter(propertyIsDefined('hoverOverlayProps')),
                    map(({ hoverOverlayProps }) => !!hoverOverlayProps),
                    distinctUntilChanged(isEqual)
                )

                const mousemoveDelay = 25
                const outputDiagram = `${TOOLTIP_DISPLAY_DELAY + mousemoveDelay}ms a`

                const outputValues: { [key: string]: boolean } = {
                    a: true,
                }

                // Mousemove on https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold(`a b ${mousemoveDelay - 2}ms c ${TOOLTIP_DISPLAY_DELAY - 1}ms`, {
                    a: 'mouseover',
                    b: 'mousemove',
                    c: 'mousemove',
                } as Record<string, SupportedMouseEvent>).subscribe(eventType =>
                    dispatchMouseEventAtPositionImpure(eventType, codeView, {
                        line: 24,
                        character: 6,
                    })
                )

                expectObservable(hoverAndDefinitionUpdates).toBe(outputDiagram, outputValues)
            })
        }
    })

    it('keeps the overlay open when the mouse briefly moves over another token on the way to the overlay', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => expect(a).toEqual(b))

            const hover = {}

            scheduler.run(({ cold, expectObservable }) => {
                const hoverOverlayElement = document.createElement('div')

                const hoverifier = createHoverifier({
                    hoverOverlayElements: of(hoverOverlayElement),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                })

                const positionJumps = new Subject<PositionJump>()

                const positionEvents = of(codeView.codeView).pipe(findPositionsFromEvents({ domFunctions: codeView }))

                const subscriptions = new Subscription()

                subscriptions.add(hoverifier)
                subscriptions.add(
                    hoverifier.hoverify({
                        dom: codeView,
                        positionEvents,
                        positionJumps,
                        resolveContext: () => codeView.revSpec,
                    })
                )

                const hoverAndDefinitionUpdates = hoverifier.hoverStateUpdates.pipe(
                    filter(propertyIsDefined('hoverOverlayProps')),
                    map(({ hoverOverlayProps }) => hoverOverlayProps.hoveredToken?.character),
                    distinctUntilChanged(isEqual)
                )

                const outputDiagram = `${TOOLTIP_DISPLAY_DELAY + MOUSEOVER_DELAY + 1}ms a`

                const outputValues: { [key: string]: number } = {
                    a: 6,
                }

                cold(`a b ${TOOLTIP_DISPLAY_DELAY}ms c d 1ms e`, {
                    a: ['mouseover', 6],
                    b: ['mousemove', 6],
                    c: ['mouseover', 19],
                    d: ['mousemove', 19],
                    e: ['mouseover', 'overlay'],
                } as Record<string, [SupportedMouseEvent, number | 'overlay']>).subscribe(([eventType, value]) => {
                    if (value === 'overlay') {
                        hoverOverlayElement.dispatchEvent(
                            new MouseEvent(eventType, {
                                bubbles: true, // Must be true so that React can see it.
                            })
                        )
                    } else {
                        dispatchMouseEventAtPositionImpure(eventType, codeView, {
                            line: 24,
                            character: value,
                        })
                    }
                })

                expectObservable(hoverAndDefinitionUpdates).toBe(outputDiagram, outputValues)
            })
            break
        }
    })

    it('dedupes mouseover and mousemove event on same token', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => expect(a).toEqual(b))

            const hover = {}

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                })

                const positionJumps = new Subject<PositionJump>()

                const positionEvents = of(codeView.codeView).pipe(findPositionsFromEvents({ domFunctions: codeView }))

                const subscriptions = new Subscription()

                subscriptions.add(hoverifier)
                subscriptions.add(
                    hoverifier.hoverify({
                        dom: codeView,
                        positionEvents,
                        positionJumps,
                        resolveContext: () => codeView.revSpec,
                    })
                )

                const hoverAndDefinitionUpdates = hoverifier.hoverStateUpdates.pipe(
                    filter(propertyIsDefined('hoverOverlayProps')),
                    map(({ hoverOverlayProps }) => !!hoverOverlayProps),
                    distinctUntilChanged(isEqual)
                )

                // Add 2 for 1 tick each for "c" and "d" below.
                const outputDiagram = `${TOOLTIP_DISPLAY_DELAY + MOUSEOVER_DELAY + 2}ms a`

                const outputValues: { [key: string]: boolean } = {
                    a: true,
                }

                // Mouse on https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold(
                    `a b ${MOUSEOVER_DELAY - 2}ms c d e`,
                    ((): Record<string, SupportedMouseEvent> => ({
                        a: 'mouseover',
                        b: 'mousemove',
                        // Now perform repeated mousemove/mouseover events on the same token.
                        c: 'mousemove',
                        d: 'mouseover',
                        e: 'mousemove',
                    }))()
                ).subscribe(eventType =>
                    dispatchMouseEventAtPositionImpure(eventType, codeView, {
                        line: 24,
                        character: 6,
                    })
                )

                expectObservable(hoverAndDefinitionUpdates).toBe(outputDiagram, outputValues)
            })
        }
    })

    /**
     * This test ensures that the adjustPosition options is being called in the ways we expect. This test is actually not the best way to ensure the feature
     * works as expected. This is a good example of a bad side effect of how the main `hoverifier.ts` file is too tightly integrated with itself. Ideally, I'd be able to assert
     * that the effected positions have actually been adjusted as intended but this is impossible with the current implementation. We can assert that the `HoverProvider` and `ActionsProvider`s
     * have the adjusted positions (AdjustmentDirection.CodeViewToActual). However, we cannot reliably assert that the code "highlighting" the token has the position adjusted (AdjustmentDirection.ActualToCodeView).
     */
    /**
     * This test is skipped because its flakey. I'm unsure how to reliably test this feature in hoverifiers current state.
     */
    it.skip('PositionAdjuster gets called when expected', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => expect(a).toEqual(b))

            scheduler.run(({ cold, expectObservable }) => {
                const adjustmentDirections = new Subject<AdjustmentDirection>()

                const getHover = createStubHoverProvider({})
                const getDocumentHighlights = createStubDocumentHighlightProvider()
                const getActions = createStubActionsProvider(['foo', 'bar'])

                const adjustPosition: PositionAdjuster<{}> = ({ direction, position }) => {
                    adjustmentDirections.next(direction)

                    return of(position)
                }

                const hoverifier = createHoverifier({
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover,
                    getDocumentHighlights,
                    getActions,
                })

                const positionJumps = new Subject<PositionJump>()

                const positionEvents = of(codeView.codeView).pipe(findPositionsFromEvents({ domFunctions: codeView }))

                const subscriptions = new Subscription()

                subscriptions.add(hoverifier)
                subscriptions.add(
                    hoverifier.hoverify({
                        dom: codeView,
                        positionEvents,
                        positionJumps,
                        adjustPosition,
                        resolveContext: () => codeView.revSpec,
                    })
                )

                const inputDiagram = 'ab'
                // There is probably a bug in code that is unrelated to this feature that is causing the
                // PositionAdjuster to be called an extra time. It should look like '-(ba)'. That is, we adjust the
                // position from CodeViewToActual for the fetches and then back from CodeViewToActual for
                // highlighting the token in the DOM.
                const outputDiagram = 'a(ba)'

                const outputValues: {
                    [key: string]: AdjustmentDirection
                } = {
                    a: AdjustmentDirection.ActualToCodeView,
                    b: AdjustmentDirection.CodeViewToActual,
                }

                cold(inputDiagram).subscribe(() =>
                    dispatchMouseEventAtPositionImpure('click', codeView, {
                        line: 1,
                        character: 1,
                    })
                )

                expectObservable(adjustmentDirections).toBe(outputDiagram, outputValues)
            })
        }
    })

    describe('unhoverify', () => {
        it('hides the hover overlay when the code view is unhoverified', async () => {
            for (const codeView of testcases) {
                const hoverifier = createHoverifier({
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    // It's important that getHover() and getActions() emit something
                    getHover: createStubHoverProvider({}),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of([{}]).pipe(delay(50)),
                })
                const positionJumps = new Subject<PositionJump>()
                const positionEvents = of(codeView.codeView).pipe(findPositionsFromEvents({ domFunctions: codeView }))

                const codeViewSubscription = hoverifier.hoverify({
                    dom: codeView,
                    positionEvents,
                    positionJumps,
                    resolveContext: () => codeView.revSpec,
                })

                dispatchMouseEventAtPositionImpure('mouseover', codeView, {
                    line: 24,
                    character: 6,
                })

                await firstValueFrom(hoverifier.hoverStateUpdates.pipe(filter(state => !!state.hoverOverlayProps)))

                codeViewSubscription.unsubscribe()

                expect(hoverifier.hoverState.hoverOverlayProps).toEqual(undefined)
                await new Promise(resolve => setTimeout(resolve, 200))
                expect(hoverifier.hoverState.hoverOverlayProps).toEqual(undefined)
            }
        })
        it('does not hide the hover overlay when a different code view is unhoverified', async () => {
            for (const codeViewProps of testcases) {
                const hoverifier = createHoverifier({
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                })
                const positionJumps = new Subject<PositionJump>()
                const positionEvents = of(codeViewProps.codeView).pipe(
                    findPositionsFromEvents({ domFunctions: codeViewProps })
                )

                const codeViewSubscription = hoverifier.hoverify({
                    dom: codeViewProps,
                    positionEvents: NEVER,
                    positionJumps: NEVER,
                    resolveContext: () => {
                        throw new Error('not called')
                    },
                })
                hoverifier.hoverify({
                    dom: codeViewProps,
                    positionEvents,
                    positionJumps,
                    resolveContext: () => codeViewProps.revSpec,
                })

                dispatchMouseEventAtPositionImpure('mouseover', codeViewProps, {
                    line: 24,
                    character: 6,
                })

                await firstValueFrom(hoverifier.hoverStateUpdates.pipe(filter(state => !!state.hoverOverlayProps)))

                codeViewSubscription.unsubscribe()

                expect(hoverifier.hoverState.hoverOverlayProps).toBeDefined()
                await new Promise(resolve => setTimeout(resolve, 200))
                expect(hoverifier.hoverState.hoverOverlayProps).toBeDefined()
            }
        })
    })
})
