import { nextTick } from 'process'
import { promisify } from 'util'

import { RenderResult } from '@testing-library/react'
import { uniqueId, pick } from 'lodash'
import { BehaviorSubject, NEVER, of, Subscription } from 'rxjs'
import { take, first } from 'rxjs/operators'
import { TestScheduler } from 'rxjs/testing'
import * as sinon from 'sinon'

import { resetAllMemoizationCaches, subtypeOf } from '@sourcegraph/common'
import { SuccessGraphQLResult } from '@sourcegraph/http-client'
import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockIntersectionObserver } from '@sourcegraph/shared/src/testing/MockIntersectionObserver'
import { toPrettyBlobURL } from '@sourcegraph/shared/src/util/url'

import { ResolveRepoResult } from '../../../graphql-operations'
import { DEFAULT_SOURCEGRAPH_URL } from '../../util/context'
import { MutationRecordLike } from '../../util/dom'

import {
    CodeIntelligenceProps,
    getExistingOrCreateOverlayMount,
    handleCodeHost,
    observeHoverOverlayMountLocation,
    HandleCodeHostOptions,
    DiffOrBlobInfo,
} from './codeHost'
import { toCodeViewResolver } from './codeViews'
import { DEFAULT_GRAPHQL_RESPONSES, mockRequestGraphQL } from './testHelpers'

const RENDER = sinon.spy()

const elementRenderedAtMount = (mount: Element): RenderResult | undefined => {
    const call = RENDER.args.find(call => call[1] === mount)
    return call?.[0]
}

const scheduler = (): TestScheduler => new TestScheduler((a, b) => expect(a).toEqual(b))

const createTestElement = (): HTMLElement => {
    const element = document.createElement('div')
    element.className = `test test-${uniqueId()}`
    document.body.append(element)
    return element
}

jest.mock('uuid', () => ({
    v4: () => 'uuid',
}))

const createMockPlatformContext = (
    partialMocks?: Partial<CodeIntelligenceProps['platformContext']>
): CodeIntelligenceProps['platformContext'] => ({
    urlToFile: toPrettyBlobURL,
    requestGraphQL: mockRequestGraphQL(),
    settings: NEVER,
    refreshSettings: () => Promise.resolve(),
    sourcegraphURL: '',
    ...partialMocks,
})

const commonArguments = () =>
    subtypeOf<Partial<HandleCodeHostOptions>>()({
        mutations: of([{ addedNodes: [document.body], removedNodes: [] }]),
        platformContext: createMockPlatformContext(),
        sourcegraphURL: DEFAULT_SOURCEGRAPH_URL,
        telemetryService: NOOP_TELEMETRY_SERVICE,
        render: RENDER,
        userSignedIn: true,
        minimalUI: false,
        background: {
            notifyRepoSyncError: () => Promise.resolve(),
            openOptionsPage: () => Promise.resolve(),
        },
    })

function getEditors(
    extensionAPI: typeof sourcegraph
): Pick<ExtensionCodeEditor, 'type' | 'viewerId' | 'isActive' | 'resource' | 'selections'>[] {
    return [...extensionAPI.app.activeWindow!.visibleViewComponents]
        .filter((viewer): viewer is ExtensionCodeEditor => viewer.type === 'CodeEditor')
        .map(editor => pick(editor, 'viewerId', 'isActive', 'resource', 'selections', 'type'))
}

const tick = promisify(nextTick)

describe('codeHost', () => {
    // Mock the global IntersectionObserver constructor with an implementation that
    // will immediately signal all observed elements as intersecting.
    beforeAll(() => {
        window.IntersectionObserver = MockIntersectionObserver
    })

    beforeEach(() => {
        document.body.innerHTML = ''
    })

    describe('createOverlayMount()', () => {
        it('should create the overlay mount', () => {
            getExistingOrCreateOverlayMount('some-code-host', document.body)
            const mount = document.body.querySelector('.hover-overlay-mount')
            expect(mount).toBeDefined()
            expect(mount!.className).toBe('hover-overlay-mount hover-overlay-mount__some-code-host')
        })
    })

    describe('handleCodeHost()', () => {
        let subscriptions = new Subscription()

        afterEach(() => {
            RENDER.resetHistory()
            resetAllMemoizationCaches()
            subscriptions.unsubscribe()
            subscriptions = new Subscription()
        })

        test('renders the hover overlay mount', async () => {
            subscriptions.add(
                await handleCodeHost({
                    ...commonArguments(),
                    codeHost: {
                        type: 'github',
                        name: 'GitHub',
                        check: () => true,
                        codeViewResolvers: [],
                    },
                })
            )
            const overlayMount = document.body.querySelector('.hover-overlay-mount')
            expect(overlayMount).toBeDefined()
            expect(overlayMount!.className).toBe('hover-overlay-mount hover-overlay-mount__github')
            const renderedOverlay = elementRenderedAtMount(overlayMount!)
            expect(renderedOverlay).not.toBeUndefined()
        })

        test('detects code views based on selectors', async () => {
            const codeView = createTestElement()
            codeView.id = 'code'
            const toolbarMount = document.createElement('div')
            codeView.append(toolbarMount)
            const blobInfo: DiffOrBlobInfo = {
                blob: {
                    rawRepoName: 'foo',
                    filePath: '/bar.ts',
                    commitID: '1',
                },
            }
            subscriptions.add(
                await handleCodeHost({
                    ...commonArguments(),
                    codeHost: {
                        type: 'github',
                        name: 'GitHub',
                        check: () => true,
                        codeViewResolvers: [
                            toCodeViewResolver('#code', {
                                dom: {
                                    getCodeElementFromTarget: sinon.spy(),
                                    getCodeElementFromLineNumber: sinon.spy(),
                                    getLineElementFromLineNumber: sinon.spy(),
                                    getLineNumberFromCodeElement: sinon.spy(),
                                },
                                resolveFileInfo: () => of(blobInfo),
                                getToolbarMount: () => toolbarMount,
                            }),
                        ],
                    },
                    platformContext: createMockPlatformContext({
                        // Simulate an instance with repositoryPathPattern
                        requestGraphQL: mockRequestGraphQL({
                            ...DEFAULT_GRAPHQL_RESPONSES,
                            ResolveRepo: variables =>
                                // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
                                of({
                                    data: {
                                        repository: {
                                            name: `github/${variables.rawRepoName as string}`,
                                        },
                                    },
                                    errors: undefined,
                                } as SuccessGraphQLResult<ResolveRepoResult>),
                        }),
                    }),
                })
            )
            await wrapRemoteObservable(extensionHostAPI.viewerUpdates()).pipe(first()).toPromise()

            expect(getEditors(extensionAPI)).toEqual([
                {
                    viewerId: 'viewer#0',
                    isActive: true,
                    // The repo name exposed to extensions is affected by repositoryPathPattern
                    resource: 'git://github/foo?1#/bar.ts',
                    selections: [],
                    type: 'CodeEditor',
                },
            ])

            await tick()
            expect(codeView).toHaveClass('sg-mounted')
            const toolbar = elementRenderedAtMount(toolbarMount)
            expect(toolbar).not.toBeUndefined()
        })

        test('removes code views and models', async () => {
            const { extensionAPI, extensionHostAPI } = await integrationTestContext(undefined, {
                roots: [],
                viewers: [],
            })
            const codeView1 = createTestElement()
            codeView1.className = 'code'
            const codeView2 = createTestElement()
            codeView2.className = 'code'
            const blobInfo: DiffOrBlobInfo = {
                blob: {
                    rawRepoName: 'foo',
                    filePath: '/bar.ts',
                    commitID: '1',
                },
            }
            const mutations = new BehaviorSubject<MutationRecordLike[]>([
                { addedNodes: [document.body], removedNodes: [] },
            ])
            subscriptions.add(
                await handleCodeHost({
                    ...commonArguments(),
                    mutations,
                    codeHost: {
                        type: 'github',
                        name: 'GitHub',
                        check: () => true,
                        codeViewResolvers: [
                            toCodeViewResolver('.code', {
                                dom: {
                                    getCodeElementFromTarget: sinon.spy(),
                                    getCodeElementFromLineNumber: sinon.spy(),
                                    getLineElementFromLineNumber: sinon.spy(),
                                    getLineNumberFromCodeElement: sinon.spy(),
                                },
                                resolveFileInfo: codeView =>
                                    codeView === codeView1
                                        ? of(blobInfo)
                                        : of({
                                              blob: {
                                                  ...blobInfo.blob,
                                                  filePath: '/bar2.ts',
                                              },
                                          }),
                            }),
                        ],
                    },
                    platformContext: createMockPlatformContext(),
                })
            )
            await wrapRemoteObservable(extensionHostAPI.viewerUpdates()).pipe(take(2)).toPromise()

            expect(getEditors(extensionAPI)).toEqual([
                {
                    viewerId: 'viewer#0',
                    isActive: true,
                    resource: 'git://foo?1#/bar.ts',
                    selections: [],
                    type: 'CodeEditor',
                },
                {
                    viewerId: 'viewer#1',
                    isActive: true,
                    resource: 'git://foo?1#/bar2.ts',
                    selections: [],
                    type: 'CodeEditor',
                },
            ])

            // // Simulate codeView1 removal
            mutations.next([{ addedNodes: [], removedNodes: [codeView1] }])
            // One editor should have been removed, model should still exist
            await wrapRemoteObservable(extensionHostAPI.viewerUpdates()).pipe(first()).toPromise()

            expect(getEditors(extensionAPI)).toEqual([
                {
                    viewerId: 'viewer#1',
                    isActive: true,
                    resource: 'git://foo?1#/bar2.ts',
                    selections: [],
                    type: 'CodeEditor',
                },
            ])
            // // Simulate codeView2 removal
            mutations.next([{ addedNodes: [], removedNodes: [codeView2] }])
            // // Second editor and model should have been removed
            await wrapRemoteObservable(extensionHostAPI.viewerUpdates()).pipe(first()).toPromise()
            expect(getEditors(extensionAPI)).toEqual([])
        })

        test('Hoverifies a view', async () => {
            const codeView = createTestElement()
            codeView.id = 'code'
            const codeElement = document.createElement('span')
            codeElement.textContent = 'alert(1)'
            codeView.append(codeElement)
            const dom = {
                getCodeElementFromTarget: sinon.spy(() => codeElement),
                getCodeElementFromLineNumber: sinon.spy(() => codeElement),
                getLineElementFromLineNumber: sinon.spy(() => codeElement),
                getLineNumberFromCodeElement: sinon.spy(() => 1),
            }
            subscriptions.add(
                await handleCodeHost({
                    ...commonArguments(),
                    codeHost: {
                        type: 'github',
                        name: 'GitHub',
                        check: () => true,
                        codeViewResolvers: [
                            toCodeViewResolver('#code', {
                                dom,
                                resolveFileInfo: () =>
                                    of({
                                        blob: {
                                            rawRepoName: 'foo',
                                            filePath: '/bar.ts',
                                            commitID: '1',
                                        },
                                    }),
                            }),
                        ],
                    },
                })
            )
            await wrapRemoteObservable(extensionHostAPI.viewerUpdates()).pipe(first()).toPromise()
            expect(getEditors(extensionAPI).length).toEqual(1)
            await tick()
            codeView.dispatchEvent(new MouseEvent('mouseover'))
            sinon.assert.called(dom.getCodeElementFromTarget)
        })
    })

    describe('observeHoverOverlayMountLocation()', () => {
        test('emits document.body if the getMountLocationSelector() returns null', () => {
            scheduler().run(({ cold, expectObservable }) => {
                expectObservable(
                    observeHoverOverlayMountLocation(
                        () => null,
                        cold<MutationRecordLike[]>('a', {
                            a: [
                                {
                                    addedNodes: [document.body],
                                    removedNodes: [],
                                },
                            ],
                        })
                    )
                ).toBe('a', {
                    a: document.body,
                })
            })
        })

        test('emits a custom mount location if a node matching the selector is in addedNodes()', () => {
            const element = createTestElement()
            scheduler().run(({ cold, expectObservable }) => {
                expectObservable(
                    observeHoverOverlayMountLocation(
                        () => '.test',
                        cold<MutationRecordLike[]>('-b', {
                            b: [
                                {
                                    addedNodes: [element],
                                    removedNodes: [],
                                },
                            ],
                        })
                    )
                ).toBe('ab', {
                    a: document.body,
                    b: element,
                })
            })
        })

        test('emits a custom mount location if a node matching the selector is nested in an addedNode', () => {
            const element = createTestElement()
            const nested = document.createElement('div')
            nested.classList.add('nested')
            element.append(nested)
            scheduler().run(({ cold, expectObservable }) => {
                expectObservable(
                    observeHoverOverlayMountLocation(
                        () => '.nested',
                        cold<MutationRecordLike[]>('-b', {
                            b: [
                                {
                                    addedNodes: [element],
                                    removedNodes: [],
                                },
                            ],
                        })
                    )
                ).toBe('ab', {
                    a: document.body,
                    b: nested,
                })
            })
        })

        test('emits document.body if a node matching the selector is removed', () => {
            const element = createTestElement()
            scheduler().run(({ cold, expectObservable }) => {
                expectObservable(
                    observeHoverOverlayMountLocation(
                        () => '.test',
                        cold<MutationRecordLike[]>('-bc', {
                            b: [
                                {
                                    addedNodes: [element],
                                    removedNodes: [],
                                },
                            ],
                            c: [
                                {
                                    addedNodes: [],
                                    removedNodes: [element],
                                },
                            ],
                        })
                    )
                ).toBe('abc', {
                    a: document.body,
                    b: element,
                    c: document.body,
                })
            })
        })
    })
})
