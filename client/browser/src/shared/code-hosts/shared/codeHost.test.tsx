import { nextTick } from 'process'
import { promisify } from 'util'

import type { RenderResult } from '@testing-library/react'
import type { Remote } from 'comlink'
import { uniqueId, noop, pick } from 'lodash'
import { BehaviorSubject, firstValueFrom, lastValueFrom, NEVER, of, Subscription } from 'rxjs'
import { take } from 'rxjs/operators'
import { TestScheduler } from 'rxjs/testing'
import * as sinon from 'sinon'
import type * as sourcegraph from 'sourcegraph'
import { afterEach, beforeAll, beforeEach, vi, describe, expect, it, test } from 'vitest'

import { resetAllMemoizationCaches, subtypeOf } from '@sourcegraph/common'
import type { SuccessGraphQLResult } from '@sourcegraph/http-client'
import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import type { FlatExtensionHostAPI } from '@sourcegraph/shared/src/api/contract'
import type { ExtensionCodeEditor } from '@sourcegraph/shared/src/api/extension/api/codeEditor'
import type { Controller } from '@sourcegraph/shared/src/extensions/controller'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { MockIntersectionObserver } from '@sourcegraph/shared/src/testing/MockIntersectionObserver'
import { integrationTestContext } from '@sourcegraph/shared/src/testing/testHelpers'
import { toPrettyBlobURL } from '@sourcegraph/shared/src/util/url'

import type { ResolveRepoResult } from '../../../graphql-operations'
import { DEFAULT_SOURCEGRAPH_URL } from '../../util/context'
import type { MutationRecordLike } from '../../util/dom'

import {
    type CodeIntelligenceProps,
    getExistingOrCreateOverlayMount,
    handleCodeHost,
    observeHoverOverlayMountLocation,
    type HandleCodeHostOptions,
    type DiffOrBlobInfo,
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

vi.mock('uuid', () => ({
    v4: () => 'uuid',
}))

const createMockController = (extensionHostAPI: Remote<FlatExtensionHostAPI>): Controller => ({
    executeCommand: () => Promise.resolve(),
    registerCommand: () => new Subscription(),
    unsubscribe: noop,
    extHostAPI: Promise.resolve(extensionHostAPI),
})

const createMockPlatformContext = (
    partialMocks?: Partial<CodeIntelligenceProps['platformContext']>
): CodeIntelligenceProps['platformContext'] => ({
    urlToFile: toPrettyBlobURL,
    requestGraphQL: mockRequestGraphQL(),
    settings: NEVER,
    refreshSettings: () => Promise.resolve(),
    sourcegraphURL: '',
    clientApplication: 'other',
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
            const { extensionHostAPI } = await integrationTestContext()
            subscriptions.add(
                await handleCodeHost({
                    ...commonArguments(),
                    codeHost: {
                        type: 'github',
                        name: 'GitHub',
                        check: () => true,
                        codeViewResolvers: [],
                    },
                    extensionsController: createMockController(extensionHostAPI),
                })
            )
            const overlayMount = document.body.querySelector('.hover-overlay-mount')
            expect(overlayMount).toBeDefined()
            expect(overlayMount!.className).toBe('hover-overlay-mount hover-overlay-mount__github')
            const renderedOverlay = elementRenderedAtMount(overlayMount!)
            expect(renderedOverlay).not.toBeUndefined()
        })

        test('detects code views based on selectors', async () => {
            const { extensionHostAPI, extensionAPI } = await integrationTestContext(undefined, {
                roots: [],
                viewers: [],
            })
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
                    extensionsController: createMockController(extensionHostAPI),
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
            await firstValueFrom(wrapRemoteObservable(extensionHostAPI.viewerUpdates()))

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
                    extensionsController: createMockController(extensionHostAPI),
                    platformContext: createMockPlatformContext(),
                })
            )
            await lastValueFrom(wrapRemoteObservable(extensionHostAPI.viewerUpdates()).pipe(take(2)), {
                defaultValue: null,
            })

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
            setTimeout(() => mutations.next([{ addedNodes: [], removedNodes: [codeView1] }]))
            // One editor should have been removed, model should still exist
            await firstValueFrom(wrapRemoteObservable(extensionHostAPI.viewerUpdates()), { defaultValue: null })

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
            setTimeout(() => mutations.next([{ addedNodes: [], removedNodes: [codeView2] }]))
            // // Second editor and model should have been removed
            await firstValueFrom(wrapRemoteObservable(extensionHostAPI.viewerUpdates()), { defaultValue: null })
            expect(getEditors(extensionAPI)).toEqual([])
        })

        test('Hoverifies a view', async () => {
            const { extensionHostAPI, extensionAPI } = await integrationTestContext(undefined, {
                roots: [],
                viewers: [],
            })
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
                    extensionsController: createMockController(extensionHostAPI),
                })
            )
            await firstValueFrom(wrapRemoteObservable(extensionHostAPI.viewerUpdates()), { defaultValue: null })
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
