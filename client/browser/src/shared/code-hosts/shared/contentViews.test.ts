import { MarkupKind } from '@sourcegraph/extension-api-classes'
import { uniqueId } from 'lodash'
import { concat, Observable, of, Subject, Subscription } from 'rxjs'
import { first } from 'rxjs/operators'
import { FlatExtensionHostAPI } from '../../../../../shared/src/api/contract'
import { LinkPreviewMerged } from '../../../../../shared/src/api/extension/flatExtensionApi'
import { createBarrier } from '../../../../../shared/src/api/integration-test/testHelpers'
import { pretendProxySubscribable, pretendRemote } from '../../../../../shared/src/api/util'
import { MutationRecordLike } from '../../util/dom'
import { handleContentViews } from './contentViews'

describe('contentViews', () => {
    beforeEach(() => {
        document.body.innerHTML = ''
    })

    describe('handleContentViews()', () => {
        let subscriptions = new Subscription()

        afterEach(() => {
            subscriptions.unsubscribe()
            subscriptions = new Subscription()
        })

        const createTestElement = (): HTMLElement => {
            const element = document.createElement('div')
            element.className = `test test-${uniqueId()}`
            document.body.append(element)
            return element
        }

        test('detects addition, mutation, and removal of content views (and annotates them)', async () => {
            const element = createTestElement()
            element.id = 'content-view'
            element.innerHTML = '0 <a href=#foo>foo</a> 1 <a href=#bar>bar</a> 2 <a href=#qux>qux</a> 3'

            const wait = new Subject<void>()
            const unsubscribed = new Subject<void>()
            const mutations = new Subject<MutationRecordLike[]>()

            subscriptions.add(
                handleContentViews(
                    mutations,
                    {
                        extensionsController: {
                            extHostAPI: Promise.resolve(
                                pretendRemote<FlatExtensionHostAPI>({
                                    getLinkPreviews: url => {
                                        wait.next()

                                        if (url.includes('bar')) {
                                            return pretendProxySubscribable(of(null))
                                        }

                                        return pretendProxySubscribable(
                                            concat(
                                                of<LinkPreviewMerged>({
                                                    content: [
                                                        {
                                                            kind: MarkupKind.Markdown,
                                                            value: `**${url.slice(url.lastIndexOf('#') + 1)}** x`,
                                                        },
                                                    ],
                                                    hover: [
                                                        {
                                                            kind: MarkupKind.PlainText,
                                                            value: url.slice(url.lastIndexOf('#') + 1),
                                                        },
                                                    ],
                                                }),
                                                // Support checking that the provider's observable was unsubscribed.
                                                new Observable<LinkPreviewMerged>(() => () => {
                                                    unsubscribed.next()
                                                })
                                            )
                                        )
                                    },
                                })
                            ),
                        },
                    },
                    {
                        contentViewResolvers: [{ selector: 'div', resolveView: () => ({ element }) }],
                        setElementTooltip: (element, text) =>
                            text !== null ? (element.dataset.tooltip = text) : element.removeAttribute('data-tooltip'),
                    }
                )
            )
            const inners: string[] = []
            wait.subscribe(() => {
                inners.push(element.innerHTML)
            })

            // Add content view.
            mutations.next([{ addedNodes: [document.body], removedNodes: [] }])
            await wait.pipe(first()).toPromise()
            expect(element.innerHTML).toBe(
                '0 <a href="#foo" data-tooltip="foo">foo</a><span class="sg-link-preview-content" data-tooltip="foo"><strong>foo</strong> x</span> 1 <a href="#bar">bar</a> 2 <a href="#qux" data-tooltip="qux">qux</a><span class="sg-link-preview-content" data-tooltip="qux"><strong>qux</strong> x</span> 3'
            )

            // Mutate content view.
            element.innerHTML = '4 <a href=#zip>zip</a> 5'
            await Promise.all([unsubscribed.pipe(first()).toPromise(), wait.pipe(first()).toPromise()])
            expect(element.innerHTML).toBe(
                '4 <a href="#zip" data-tooltip="zip">zip</a><span class="sg-link-preview-content" data-tooltip="zip"><strong>zip</strong> x</span> 5'
            )

            // Remove content view.
            mutations.next([{ addedNodes: [], removedNodes: [element] }])
            await unsubscribed.pipe(first()).toPromise()

            console.log({ inners })
        })

        test('handles multiple emissions', async () => {
            const element = createTestElement()
            element.id = 'content-view'
            element.innerHTML = '0 <a href=#foo>foo</a> 1 <a href=#bar>bar</a> 2'
            const originalInnerHTML = element.innerHTML
            const fooLinkPreviewValues = new Subject<LinkPreviewMerged>()
            const { wait, done } = createBarrier()
            subscriptions.add(
                handleContentViews(
                    of([{ addedNodes: [document.body], removedNodes: [] }]),
                    {
                        extensionsController: {
                            extHostAPI: Promise.resolve(
                                pretendRemote<FlatExtensionHostAPI>({
                                    getLinkPreviews: url => {
                                        done()
                                        return pretendProxySubscribable(
                                            url.includes('bar') ? of(null) : fooLinkPreviewValues
                                        )
                                    },
                                })
                            ),
                        },
                    },
                    {
                        contentViewResolvers: [{ selector: 'div', resolveView: () => ({ element }) }],
                        setElementTooltip: (element, text) =>
                            text !== null ? (element.dataset.tooltip = text) : element.removeAttribute('data-tooltip'),
                    }
                )
            )

            await wait
            expect(element.innerHTML).toBe(originalInnerHTML)

            fooLinkPreviewValues.next({
                content: [
                    {
                        kind: MarkupKind.Markdown,
                        value: '**foo**',
                    },
                ],
                hover: [
                    {
                        kind: MarkupKind.PlainText,
                        value: 'foo',
                    },
                ],
            })
            expect(element.innerHTML).toBe(
                '0 <a href="#foo" data-tooltip="foo">foo</a><span class="sg-link-preview-content" data-tooltip="foo"><strong>foo</strong></span> 1 <a href="#bar">bar</a> 2'
            )

            fooLinkPreviewValues.next({
                content: [
                    {
                        kind: MarkupKind.Markdown,
                        value: '**foo2**',
                    },
                ],
                hover: [
                    {
                        kind: MarkupKind.PlainText,
                        value: 'foo2',
                    },
                ],
            })
            expect(element.innerHTML).toBe(
                '0 <a href="#foo" data-tooltip="foo2">foo</a><span class="sg-link-preview-content" data-tooltip="foo2"><strong>foo2</strong></span> 1 <a href="#bar">bar</a> 2'
            )

            fooLinkPreviewValues.next({
                content: [],
                hover: [
                    {
                        kind: MarkupKind.PlainText,
                        value: 'foo2',
                    },
                ],
            })
            expect(element.innerHTML).toBe('0 <a href="#foo" data-tooltip="foo2">foo</a> 1 <a href="#bar">bar</a> 2')

            fooLinkPreviewValues.next({
                content: [],
                hover: [],
            })
            expect(element.innerHTML).toBe('0 <a href="#foo">foo</a> 1 <a href="#bar">bar</a> 2')
        })
    })
})
