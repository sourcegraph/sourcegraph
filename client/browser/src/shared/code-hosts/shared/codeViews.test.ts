import { lastValueFrom, of } from 'rxjs'
import { toArray } from 'rxjs/operators'
import * as sinon from 'sinon'
import type { Omit } from 'utility-types'
import { beforeEach, describe, expect, it } from 'vitest'

import type { DiffOrBlobInfo } from './codeHost'
import { type CodeView, toCodeViewResolver, trackCodeViews } from './codeViews'

describe('codeViews', () => {
    beforeEach(() => {
        document.body.innerHTML = ''
    })
    describe('trackCodeViews()', () => {
        const fileInfo: DiffOrBlobInfo = {
            blob: {
                rawRepoName: 'foo',
                filePath: '/bar.ts',
                commitID: '1',
            },
        }
        const codeViewSpec: Omit<CodeView, 'element'> = {
            dom: {
                getCodeElementFromTarget: () => null,
                getCodeElementFromLineNumber: () => null,
                getLineElementFromLineNumber: () => null,
                getLineNumberFromCodeElement: () => 1,
            },
            resolveFileInfo: () => of(fileInfo),
        }
        it('should detect added code views from specs', async () => {
            const element = document.createElement('div')
            element.className = 'test-code-view'
            document.body.append(element)
            const selector = '.test-code-view'
            const detected = await lastValueFrom(
                of([{ addedNodes: [document.body], removedNodes: [] }]).pipe(
                    trackCodeViews({
                        codeViewResolvers: [toCodeViewResolver(selector, codeViewSpec)],
                    }),
                    toArray()
                )
            )
            expect(detected.map(({ subscriptions, ...rest }) => rest)).toEqual([{ ...codeViewSpec, element }])
        })
        it('should detect added code views from resolver', async () => {
            const element = document.createElement('div')
            element.className = 'test-code-view'
            document.body.append(element)
            const selector = '.test-code-view'
            const resolveView = sinon.spy((element: HTMLElement) => ({ element, ...codeViewSpec }))
            const detected = await lastValueFrom(
                of([{ addedNodes: [document.body], removedNodes: [] }]).pipe(
                    trackCodeViews({
                        codeViewResolvers: [{ selector, resolveView }],
                    }),
                    toArray()
                )
            )
            expect(detected.map(({ subscriptions, ...rest }) => rest)).toEqual([{ ...codeViewSpec, element }])
            sinon.assert.calledOnce(resolveView)
            sinon.assert.calledWith(resolveView, element)
        })
    })
})
