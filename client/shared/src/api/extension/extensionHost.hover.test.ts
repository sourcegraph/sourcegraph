import { MarkupKind } from '@sourcegraph/extension-api-classes'
import { Hover } from 'sourcegraph'
import { HoverMerged } from '../client/types/hover'
import { initNewExtensionAPI } from './flatExtensionApi'
import { pretendRemote } from '../util'
import { MainThreadAPI } from '../contract'
import { SettingsCascade } from '../../settings/settings'
import { Observer } from 'rxjs'
import { ProxyMarked, proxyMarker, Remote } from 'comlink'
import { MaybeLoadingResult } from '@sourcegraph/codeintellify'

describe('getHover from ExtensionHost API, it aims to have more e2e feel', () => {
    // integration(ish) tests for scenarios not covered by providers tests
    const noopMain = pretendRemote<MainThreadAPI>({})
    const emptySettings: SettingsCascade<object> = {
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

    const textHover = (value: string): Hover => ({
        contents: { value, kind: MarkupKind.PlainText },
    })

    it('restarts hover call if a provider was added or removed', () => {
        const typescriptFileUri = 'file:///f.ts'

        const { exposedToMain, languages } = initNewExtensionAPI(noopMain, {
            initialSettings: emptySettings,
            clientApplication: 'sourcegraph',
        })

        exposedToMain.addTextDocumentIfNotExists({
            languageId: 'ts',
            text: 'body',
            uri: typescriptFileUri,
        })

        let counter = 0
        languages.registerHoverProvider([{ pattern: '*.ts' }], {
            provideHover: () => textHover(`a${++counter}`),
        })

        let results: any[] = []
        exposedToMain
            .getHover({
                position: { line: 1, character: 2 },
                textDocument: { uri: typescriptFileUri },
            })
            .subscribe(observe(value => results.push(value)))

        // first provider results
        expect(results).toEqual<MaybeLoadingResult<HoverMerged | null>[]>([
            { isLoading: true, result: null },
            {
                isLoading: false,
                result: { contents: [textHover('a1').contents], alerts: [] },
            },
        ])
        results = []

        const subscription = languages.registerHoverProvider([{ pattern: '*.ts' }], {
            provideHover: () => textHover('b'),
        })

        // second and first
        expect(results).toEqual<MaybeLoadingResult<HoverMerged | null>[]>([
            {
                isLoading: true,
                result: { contents: [textHover('a2').contents], alerts: [] },
            },
            {
                isLoading: false,
                result: {
                    contents: ['a2', 'b'].map(value => textHover(value).contents),
                    alerts: [],
                },
            },
        ])
        results = []

        subscription.unsubscribe()

        // just first was queried for the third time
        expect(results).toEqual<MaybeLoadingResult<HoverMerged | null>[]>([
            { isLoading: true, result: null },
            {
                isLoading: false,
                result: { contents: [textHover('a3').contents], alerts: [] },
            },
        ])
    })
})
