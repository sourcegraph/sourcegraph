import { Range } from '@sourcegraph/extension-api-types'
import { isEqual } from 'lodash'
import { EMPTY, NEVER, Observable, of, Subject, Subscription } from 'rxjs'
import { delay, distinctUntilChanged, filter, first, map, takeWhile } from 'rxjs/operators'
import { TestScheduler } from 'rxjs/testing'
import { ErrorLike } from './errors'
import { isDefined, propertyIsDefined } from './helpers'
import {
    AdjustmentDirection,
    createHoverifier,
    LOADER_DELAY,
    MOUSEOVER_DELAY,
    PositionAdjuster,
    PositionJump,
    TOOLTIP_DISPLAY_DELAY,
} from './hoverifier'
import { findPositionsFromEvents, SupportedMouseEvent } from './positions'
import { CodeViewProps, DOM } from './testutils/dom'
import {
    createHoverAttachment,
    createStubActionsProvider,
    createStubHoverProvider,
    createStubDocumentHighlightProvider,
} from './testutils/fixtures'
import { dispatchMouseEventAtPositionImpure } from './testutils/mouse'
import { HoverAttachment } from './types'
import { LOADING } from './loading'

const { assert } = chai

describe('Hoverifier', () => {
    const dom = new DOM()
    after(dom.cleanup)

    let testcases: CodeViewProps[] = []
    before(() => {
        testcases = dom.createCodeViews()
    })

    let subscriptions = new Subscription()

    afterEach(() => {
        subscriptions.unsubscribe()
        subscriptions = new Subscription()
    })

    it('highlights token when hover is fetched (not before)', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

            const delayTime = 100
            const hoverRange = { start: { line: 1, character: 2 }, end: { line: 3, character: 4 } }
            const hoverRange1Indexed = { start: { line: 2, character: 3 }, end: { line: 4, character: 5 } }

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    closeButtonClicks: NEVER,
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider({ range: hoverRange }, LOADER_DELAY + delayTime),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                    pinningEnabled: true,
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

    it('pins the overlay without it disappearing temporarily on mouseover then click', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

            const hover = {}
            const delayTime = 10

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    closeButtonClicks: NEVER,
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover, delayTime),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: createStubActionsProvider(['foo', 'bar'], delayTime),
                    pinningEnabled: true,
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
                    map(hoverState => !!hoverState.hoverOverlayProps),
                    distinctUntilChanged(isEqual)
                )

                // If you need to debug this test, the following might help. Append this to the `outputDiagram`
                // string below:
                //
                //   ` ${delayAfterMouseover - 1}ms c ${delayTime - 1}ms d`
                //
                // Also, add these properties to `outputValues`:
                //
                //   c: true, // the most important instant, right after the click to pin (must be true, meaning it doesn't disappear)
                //   d: true,
                //
                // There should be no emissions at "c" or "d", so this will cause the test to fail. But those are
                // the most likely instants where there would be an emission if pinning is causing a temporary
                // disappearance of the overlay.
                const delayAfterMouseover = 100
                const outputDiagram = `${MOUSEOVER_DELAY}ms a ${delayTime - 1}ms b`
                const outputValues: {
                    [key: string]: boolean
                } = {
                    a: false,
                    b: true,
                }

                // Mouseover https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold('a').subscribe(() =>
                    dispatchMouseEventAtPositionImpure('mouseover', codeView, {
                        line: 24,
                        character: 6,
                    })
                )

                // Click (to pin) https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold(`${MOUSEOVER_DELAY + delayTime + delayAfterMouseover}ms c`).subscribe(() =>
                    dispatchMouseEventAtPositionImpure('click', codeView, {
                        line: 24,
                        character: 6,
                    })
                )

                // Mouseover something else and ensure it remains pinned.
                cold(`${MOUSEOVER_DELAY + delayTime + delayAfterMouseover + 100}ms d`).subscribe(() =>
                    dispatchMouseEventAtPositionImpure('mouseover', codeView, {
                        line: 25,
                        character: 3,
                    })
                )

                expectObservable(hoverAndDefinitionUpdates).toBe(outputDiagram, outputValues)
            })
        }
    })

    it('does not pin the overlay on click when pinningEnabled is false', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

            const hover = {}
            const delayTime = 10

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    closeButtonClicks: NEVER,
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover, delayTime),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: createStubActionsProvider(['foo', 'bar'], delayTime),
                    pinningEnabled: false,
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
                    map(hoverState => !!hoverState.hoverOverlayProps),
                    distinctUntilChanged(isEqual)
                )

                const delayAfterMouseover = 100
                const outputDiagram = `${MOUSEOVER_DELAY}ms a ${delayTime - 1}ms b ${
                    MOUSEOVER_DELAY + delayAfterMouseover + 100 - 1
                }ms c ${delayTime - 1}ms d`
                const outputValues: {
                    [key: string]: boolean
                } = {
                    a: false,
                    b: true,
                    c: false,
                    d: true,
                }

                // Mouseover https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold('a').subscribe(() =>
                    dispatchMouseEventAtPositionImpure('mouseover', codeView, {
                        line: 24,
                        character: 6,
                    })
                )

                // Click (should not get pinned) https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold(`${MOUSEOVER_DELAY + delayTime + delayAfterMouseover}ms c`).subscribe(() =>
                    dispatchMouseEventAtPositionImpure('click', codeView, {
                        line: 24,
                        character: 6,
                    })
                )

                // Mouseover something else and ensure it doesn't get pinned.
                cold(`${MOUSEOVER_DELAY + delayTime + delayAfterMouseover + 100}ms d`).subscribe(() =>
                    dispatchMouseEventAtPositionImpure('mouseover', codeView, {
                        line: 25,
                        character: 3,
                    })
                )

                expectObservable(hoverAndDefinitionUpdates).toBe(outputDiagram, outputValues)
            })
        }
    })

    it('emits the currently hovered token', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

            const hover = {}
            const delayTime = 10

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    closeButtonClicks: NEVER,
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover, delayTime),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: createStubActionsProvider(['foo', 'bar'], delayTime),
                    pinningEnabled: false,
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
                    map(hoverState => hoverState.hoveredTokenElement && hoverState.hoveredTokenElement.textContent),
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
                closeButtonClicks: NEVER,
                hoverOverlayElements: of(null),
                hoverOverlayRerenders: EMPTY,
                getHover: createStubHoverProvider(),
                getDocumentHighlights: createStubDocumentHighlightProvider([
                    { range: { start: { line: 24, character: 9 }, end: { line: 4, character: 15 } } },
                    { range: { start: { line: 45, character: 5 }, end: { line: 45, character: 11 } } },
                    { range: { start: { line: 120, character: 9 }, end: { line: 120, character: 15 } } },
                ]),
                getActions: () => of(null),
                pinningEnabled: true,
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

            await hoverifier.hoverStateUpdates
                .pipe(
                    filter(state => !!state.hoverOverlayProps),
                    first()
                )
                .toPromise()

            await of(null).pipe(delay(200)).toPromise()

            const selected = codeViewProps.codeView.querySelectorAll('.test-highlight')
            assert.equal(selected.length, 3)
            for (const e of selected) {
                assert.equal(e.textContent, 'Router')
            }
        }
    })

    it('hides the hover overlay when the hovered token intersects with a scrollBoundary', async () => {
        const gitHubCodeView = testcases[1]
        const hoverifier = createHoverifier({
            closeButtonClicks: NEVER,
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
            pinningEnabled: true,
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
        const hasHoveredToken = hoverifier.hoverStateUpdates
            .pipe(takeWhile(({ hoveredTokenElement }) => !isDefined(hoveredTokenElement)))
            .toPromise()
        dispatchMouseEventAtPositionImpure('click', gitHubCodeView, {
            line: 5,
            character: 9,
        })
        await hasHoveredToken

        // Scroll down: the hover overlay should get hidden.
        const hoverIsHidden = hoverifier.hoverStateUpdates
            .pipe(takeWhile(({ hoverOverlayProps }) => isDefined(hoverOverlayProps)))
            .toPromise()
        gitHubCodeView.getCodeElementFromLineNumber(gitHubCodeView.codeView, 2)!.scrollIntoView({ behavior: 'smooth' })
        await hoverIsHidden
    })

    describe('pinning', () => {
        it('unpins upon clicking on a different position', () => {
            for (const codeView of testcases) {
                const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

                const delayTime = 10

                scheduler.run(({ cold, expectObservable }) => {
                    const hoverifier = createHoverifier({
                        closeButtonClicks: NEVER,
                        hoverOverlayElements: of(null),
                        hoverOverlayRerenders: EMPTY,
                        // Only show on line 24, not line 25 (which is the 2nd click event below).
                        getHover: position =>
                            position.line === 24
                                ? createStubHoverProvider({}, delayTime)(position)
                                : of({ isLoading: false, result: null }),
                        getDocumentHighlights: createStubDocumentHighlightProvider(),
                        getActions: position =>
                            position.line === 24
                                ? createStubActionsProvider(['foo', 'bar'], delayTime)(position)
                                : of(null),
                        pinningEnabled: true,
                    })

                    const positionJumps = new Subject<PositionJump>()

                    const positionEvents = of(codeView.codeView).pipe(
                        findPositionsFromEvents({ domFunctions: codeView })
                    )

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

                    const isPinned = hoverifier.hoverStateUpdates.pipe(
                        map(
                            hoverState =>
                                !!hoverState.hoverOverlayProps && !!hoverState.hoverOverlayProps.showCloseButton
                        ),
                        distinctUntilChanged()
                    )

                    const outputDiagram = `${delayTime}ms a-c`
                    const outputValues: {
                        [key: string]: boolean
                    } = {
                        a: true,
                        c: false,
                    }

                    // Click (to pin) https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                    cold('a').subscribe(() => {
                        dispatchMouseEventAtPositionImpure('click', codeView, {
                            line: 24,
                            character: 6,
                        })
                    })

                    // Click to another position and ensure the hover is no longer pinned.
                    cold(`${delayTime}ms --c`).subscribe(() =>
                        positionJumps.next({
                            codeView: codeView.codeView,
                            scrollElement: codeView.container,
                            position: { line: 1, character: 1 },
                        })
                    )

                    expectObservable(isPinned).toBe(outputDiagram, outputValues)
                })
            }
        })

        it('unpins upon navigation to an invalid or undefined position (such as a file with no particular position)', () => {
            for (const codeView of testcases) {
                const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

                scheduler.run(({ cold, expectObservable }) => {
                    const hoverifier = createHoverifier({
                        closeButtonClicks: NEVER,
                        hoverOverlayElements: of(null),
                        hoverOverlayRerenders: EMPTY,
                        // Only show on line 24, not line 25 (which is the 2nd click event below).
                        getHover: position =>
                            position.line === 24
                                ? createStubHoverProvider({})(position)
                                : of({ isLoading: false, result: null }),
                        getDocumentHighlights: createStubDocumentHighlightProvider(),
                        getActions: position =>
                            position.line === 24 ? createStubActionsProvider(['foo', 'bar'])(position) : of(null),
                        pinningEnabled: true,
                    })

                    const positionJumps = new Subject<PositionJump>()

                    const positionEvents = of(codeView.codeView).pipe(
                        findPositionsFromEvents({ domFunctions: codeView })
                    )

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

                    const isPinned = hoverifier.hoverStateUpdates.pipe(
                        map(hoverState => {
                            if (!hoverState.hoverOverlayProps) {
                                return 'hidden'
                            }
                            if (hoverState.hoverOverlayProps.showCloseButton) {
                                return 'pinned'
                            }
                            return 'visible'
                        }),
                        distinctUntilChanged()
                    )

                    const outputDiagram = 'ab'
                    const outputValues: {
                        [key: string]: 'hidden' | 'pinned' | 'visible'
                    } = {
                        a: 'pinned',
                        b: 'hidden',
                    }

                    // Click (to pin) https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                    cold('a').subscribe(() =>
                        dispatchMouseEventAtPositionImpure('click', codeView, {
                            line: 24,
                            character: 6,
                        })
                    )

                    // Click to another position and ensure the hover is no longer pinned.
                    cold('-b').subscribe(() =>
                        positionJumps.next({
                            codeView: codeView.codeView,
                            scrollElement: codeView.container,
                            position: { line: undefined, character: undefined },
                        })
                    )

                    expectObservable(isPinned).toBe(outputDiagram, outputValues)
                })
            }
        })
    })

    it('emits loading and then state on click events', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

            const hoverDelayTime = 100
            const actionsDelayTime = 150
            const hover = {}
            const actions = ['foo', 'bar']

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    closeButtonClicks: new Observable<MouseEvent>(),
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover, LOADER_DELAY + hoverDelayTime),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: createStubActionsProvider(actions, LOADER_DELAY + actionsDelayTime),
                    pinningEnabled: true,
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

                const hoverAndActionsUpdates = hoverifier.hoverStateUpdates.pipe(
                    filter(propertyIsDefined('hoverOverlayProps')),
                    map(({ hoverOverlayProps: { actionsOrError, hoverOrError } }) => ({
                        actionsOrError,
                        hoverOrError,
                    })),
                    distinctUntilChanged((a, b) => isEqual(a, b))
                )

                const inputDiagram = 'a'

                // Subtract 1ms before "b" because "a" takes up 1ms.
                const outputDiagram = `${LOADER_DELAY}ms ${hoverDelayTime}ms a ${
                    actionsDelayTime - hoverDelayTime - 1
                }ms b`

                const outputValues: {
                    [key: string]: {
                        hoverOrError: typeof LOADING | HoverAttachment | null | ErrorLike
                        actionsOrError: typeof LOADING | string[] | null | ErrorLike
                    }
                } = {
                    // No hover is shown if it would just consist of LOADING.
                    a: { hoverOrError: createHoverAttachment(hover), actionsOrError: LOADING },
                    b: { hoverOrError: createHoverAttachment(hover), actionsOrError: actions },
                }

                // Click https://sourcegraph.sgdev.org/github.com/gorilla/mux@cb4698366aa625048f3b815af6a0dea8aef9280a/-/blob/mux.go#L24:6
                cold(inputDiagram).subscribe(() =>
                    dispatchMouseEventAtPositionImpure('click', codeView, {
                        line: 24,
                        character: 6,
                    })
                )

                expectObservable(hoverAndActionsUpdates).toBe(outputDiagram, outputValues)
            })
        }
    })

    it('debounces mousemove events before showing overlay', () => {
        for (const codeView of testcases) {
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

            const hover = {}

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    closeButtonClicks: new Observable<MouseEvent>(),
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                    pinningEnabled: true,
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
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

            const hover = {}

            scheduler.run(({ cold, expectObservable }) => {
                const hoverOverlayElement = document.createElement('div')

                const hoverifier = createHoverifier({
                    closeButtonClicks: new Observable<MouseEvent>(),
                    hoverOverlayElements: of(hoverOverlayElement),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                    pinningEnabled: true,
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
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

            const hover = {}

            scheduler.run(({ cold, expectObservable }) => {
                const hoverifier = createHoverifier({
                    closeButtonClicks: new Observable<MouseEvent>(),
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(hover),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                    pinningEnabled: true,
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
            const scheduler = new TestScheduler((a, b) => chai.assert.deepEqual(a, b))

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
                    closeButtonClicks: new Observable<MouseEvent>(),
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover,
                    getDocumentHighlights,
                    getActions,
                    pinningEnabled: true,
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
                    closeButtonClicks: NEVER,
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    // It's important that getHover() and getActions() emit something
                    getHover: createStubHoverProvider({}),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of([{}]).pipe(delay(50)),
                    pinningEnabled: true,
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

                await hoverifier.hoverStateUpdates
                    .pipe(
                        filter(state => !!state.hoverOverlayProps),
                        first()
                    )
                    .toPromise()

                codeViewSubscription.unsubscribe()

                assert.strictEqual(hoverifier.hoverState.hoverOverlayProps, undefined)
                await of(null).pipe(delay(200)).toPromise()
                assert.strictEqual(hoverifier.hoverState.hoverOverlayProps, undefined)
            }
        })
        it('does not hide the hover overlay when a different code view is unhoverified', async () => {
            for (const codeViewProps of testcases) {
                const hoverifier = createHoverifier({
                    closeButtonClicks: NEVER,
                    hoverOverlayElements: of(null),
                    hoverOverlayRerenders: EMPTY,
                    getHover: createStubHoverProvider(),
                    getDocumentHighlights: createStubDocumentHighlightProvider(),
                    getActions: () => of(null),
                    pinningEnabled: true,
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

                await hoverifier.hoverStateUpdates
                    .pipe(
                        filter(state => !!state.hoverOverlayProps),
                        first()
                    )
                    .toPromise()

                codeViewSubscription.unsubscribe()

                assert.isDefined(hoverifier.hoverState.hoverOverlayProps)
                await of(null).pipe(delay(200)).toPromise()
                assert.isDefined(hoverifier.hoverState.hoverOverlayProps)
            }
        })
    })
})
