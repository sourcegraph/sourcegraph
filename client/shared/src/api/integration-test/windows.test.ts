import { pick } from 'lodash'
import { lastValueFrom, of } from 'rxjs'
import { switchMap, take, toArray } from 'rxjs/operators'
import type { ViewComponent, Window } from 'sourcegraph'
import { describe, expect, test } from 'vitest'

import { fromSubscribable } from '@sourcegraph/common'

import { assertToJSON, integrationTestContext } from '../../testing/testHelpers'
import type { TextDocumentData } from '../viewerTypes'

describe('Windows (integration)', () => {
    describe('app.activeWindow', () => {
        test('returns the active window', async () => {
            const { extensionAPI } = await integrationTestContext()
            const viewComponent: Pick<ViewComponent, 'type'> & {
                document: TextDocumentData
            } = {
                type: 'CodeEditor' as const,
                document: { uri: 'file:///f', languageId: 'l', text: 't' },
            }
            assertToJSON(pick(extensionAPI.app.activeWindow!, 'visibleViewComponents', 'activeViewComponent'), {
                visibleViewComponents: [viewComponent],
                activeViewComponent: viewComponent,
            })
        })
    })

    describe('app.activeWindowChanges', () => {
        // Skipped, as sourcegraph.app.activeWindow is always defined.
        test.skip('reflects changes to the active window', async () => {
            const { extensionAPI, extensionHostAPI } = await integrationTestContext(undefined, {
                roots: [],
                viewers: [],
            })
            expect(extensionAPI.app.activeWindow).toBeUndefined()
            await extensionHostAPI.addTextDocumentIfNotExists({
                uri: 'u',
                languageId: 'l',
                text: 't',
            })
            await extensionHostAPI.addViewerIfNotExists({
                type: 'CodeEditor',
                resource: 'u',
                selections: [],
                isActive: true,
            })

            expect(extensionAPI.app.activeWindow).toBeTruthy()
        })
    })

    describe('app.windows', () => {
        test('lists windows', async () => {
            const { extensionAPI } = await integrationTestContext()
            const viewComponent: Pick<ViewComponent, 'type'> & {
                document: TextDocumentData
            } = {
                type: 'CodeEditor' as const,
                document: { uri: 'file:///f', languageId: 'l', text: 't' },
            }
            assertToJSON(
                extensionAPI.app.windows.map(window => pick(window, 'visibleViewComponents', 'activeViewComponent')),
                [
                    {
                        visibleViewComponents: [viewComponent],
                        activeViewComponent: viewComponent,
                    },
                ] as Window[]
            )
        })

        test('adds new text documents', async () => {
            const { extensionAPI, extensionHostAPI } = await integrationTestContext(undefined, {
                viewers: [],
                roots: [],
            })

            await extensionHostAPI.addTextDocumentIfNotExists({ uri: 'file:///f2', languageId: 'l2', text: 't2' })
            await extensionHostAPI.addViewerIfNotExists({
                type: 'CodeEditor',
                resource: 'file:///f2',
                selections: [],
                isActive: true,
            })

            const viewComponent: Pick<ViewComponent, 'type'> & {
                document: TextDocumentData
            } = {
                type: 'CodeEditor' as const,
                document: { uri: 'file:///f2', languageId: 'l2', text: 't2' },
            }
            assertToJSON(
                extensionAPI.app.windows.map(window => pick(window, 'visibleViewComponents', 'activeViewComponent')),
                [
                    {
                        visibleViewComponents: [viewComponent],
                        activeViewComponent: viewComponent,
                    },
                ] as Window[]
            )
        })
    })

    describe('Window', () => {
        test('Window#visibleViewComponents', async () => {
            const { extensionAPI, extensionHostAPI } = await integrationTestContext()

            await extensionHostAPI.addTextDocumentIfNotExists({
                uri: 'u2',
                languageId: 'l2',
                text: 't2',
            })
            await extensionHostAPI.addViewerIfNotExists({
                type: 'CodeEditor',
                resource: 'u2',
                selections: [],
                isActive: true,
            })

            assertToJSON(extensionAPI.app.windows[0].visibleViewComponents, [
                {
                    type: 'CodeEditor' as const,
                    document: { uri: 'file:///f', languageId: 'l', text: 't' },
                },
                {
                    type: 'CodeEditor' as const,
                    document: { uri: 'u2', languageId: 'l2', text: 't2' },
                },
            ] as ViewComponent[])
        })

        describe('Window#activeViewComponent', () => {
            test('ignores inactive components', async () => {
                const { extensionAPI, extensionHostAPI } = await integrationTestContext()

                await extensionHostAPI.addTextDocumentIfNotExists({
                    uri: 'file:///inactive',
                    languageId: 'inactive',
                    text: 'inactive',
                })
                await extensionHostAPI.addViewerIfNotExists({
                    type: 'CodeEditor',
                    resource: 'file:///inactive',
                    selections: [],
                    isActive: false,
                })

                assertToJSON(extensionAPI.app.windows[0].activeViewComponent, {
                    type: 'CodeEditor' as const,
                    document: { uri: 'file:///f', languageId: 'l', text: 't' },
                })
            })
        })

        describe('Window#activeViewComponentChanges', () => {
            // Skipped, as sourcegraph.app.activeWindow is always defined.
            test.skip('reflects changes to the active window', async () => {
                const { extensionAPI, extensionHostAPI } = await integrationTestContext(undefined, {
                    roots: [],
                    viewers: [],
                })

                const viewers = lastValueFrom(
                    fromSubscribable(extensionAPI.app.activeWindowChanges).pipe(
                        switchMap(activeWindow =>
                            activeWindow ? fromSubscribable(activeWindow.activeViewComponentChanges) : of(null)
                        ),
                        take(4),
                        toArray()
                    )
                )

                await extensionHostAPI.addTextDocumentIfNotExists({ uri: 'foo', languageId: 'l1', text: 't1' })
                await extensionHostAPI.addTextDocumentIfNotExists({ uri: 'bar', languageId: 'l2', text: 't2' })
                const viewerId = await extensionHostAPI.addViewerIfNotExists({
                    type: 'CodeEditor',
                    resource: 'foo',
                    selections: [],
                    isActive: true,
                })
                await extensionHostAPI.removeViewer(viewerId)
                await extensionHostAPI.addViewerIfNotExists({
                    type: 'CodeEditor',
                    resource: 'bar',
                    selections: [],
                    isActive: true,
                })
                assertToJSON(
                    (await viewers).map(viewer =>
                        viewer && viewer.type === 'CodeEditor' ? viewer.document.uri : null
                    ),
                    [null, 'foo', null, 'bar']
                )
            })
        })
    })
})
