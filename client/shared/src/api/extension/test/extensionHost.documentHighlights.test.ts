import { type ProxyMarked, proxyMarker, type Remote } from 'comlink'
import { BehaviorSubject, type Observer } from 'rxjs'
import type { DocumentHighlight } from 'sourcegraph'
import { describe, expect, it } from 'vitest'

import { Range } from '@sourcegraph/extension-api-classes'

import type { SettingsCascade } from '../../../settings/settings'
import type { ClientAPI } from '../../client/api/api'
import { pretendRemote } from '../../util'
import { proxySubscribable } from '../api/common'

import { initializeExtensionHostTest } from './test-helpers'

describe('getDocumentHighlights from ExtensionHost API, it aims to have more e2e feel', () => {
    // integration(ish) tests for scenarios not covered by providers tests
    const noopMain = pretendRemote<ClientAPI>({
        getEnabledExtensions: () => proxySubscribable(new BehaviorSubject([])),
    })
    const initialSettings: SettingsCascade<object> = {
        subjects: [],
        final: {},
    }

    const observe = <T>(onValue: (value: T) => void): Remote<Observer<T> & ProxyMarked> =>
        pretendRemote({
            next: onValue,
            error: (error: any) => {
                throw error
            },
            complete: () => {},
            [proxyMarker]: Promise.resolve(true as const),
        })

    const documentHighlight = (value: number): DocumentHighlight => ({
        range: new Range(value, value, value, value),
    })

    it('restarts document highlights call if a provider was added or removed', () => {
        const typescriptFileUri = 'git://repo#src/f.ts'

        const { extensionHostAPI, extensionAPI } = initializeExtensionHostTest(
            { initialSettings, clientApplication: 'sourcegraph', sourcegraphURL: 'https://example.com/' },
            noopMain
        )

        extensionHostAPI.addTextDocumentIfNotExists({
            languageId: 'ts',
            text: 'body',
            uri: typescriptFileUri,
        })

        let counter = 0
        extensionAPI.languages.registerDocumentHighlightProvider([{ pattern: '*.ts' }], {
            provideDocumentHighlights: () => [documentHighlight(++counter)],
        })

        let results: DocumentHighlight[][] = []
        extensionHostAPI
            .getDocumentHighlights({
                position: { line: 1, character: 2 },
                textDocument: { uri: typescriptFileUri },
            })
            // @ts-expect-error - Unclear how to consolidate the different versions of the DocumentHighlight types
            .subscribe(observe(value => results.push(value)))

        // first provider results
        expect(results).toEqual([[], [documentHighlight(1)]])
        results = []

        const subscription = extensionAPI.languages.registerDocumentHighlightProvider([{ pattern: '*.ts' }], {
            provideDocumentHighlights: () => [documentHighlight(0)],
        })

        // second and first
        expect(results).toEqual([[], [2, 0].map(value => documentHighlight(value))])
        results = []

        subscription.unsubscribe()

        // just first was queried for the third time
        expect(results).toEqual([[], [documentHighlight(3)]])
    })
})
