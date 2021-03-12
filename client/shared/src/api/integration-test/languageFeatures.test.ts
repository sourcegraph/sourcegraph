import { MarkupKind } from '@sourcegraph/extension-api-classes'
import { Location } from '@sourcegraph/extension-api-types'
import { asyncScheduler, Observable, of } from 'rxjs'
import { observeOn, take, toArray, map, first } from 'rxjs/operators'
import * as sourcegraph from 'sourcegraph'
import { assertToJSON, createBarrier, integrationTestContext } from './testHelpers'
import { MaybeLoadingResult } from '@sourcegraph/codeintellify'
import { Remote } from 'comlink'
import { FlatExtensionHostAPI } from '../contract'
import { wrapRemoteObservable } from '../client/api/common'

describe('LanguageFeatures (integration)', () => {
    testLocationProvider<sourcegraph.HoverProvider>({
        name: 'registerHoverProvider',
        registerProvider: extensionAPI => (selector, provider) =>
            extensionAPI.languages.registerHoverProvider(selector, provider),
        labeledProvider: label => ({
            provideHover: (textDocument: sourcegraph.TextDocument, position: sourcegraph.Position) =>
                of({
                    contents: { value: label, kind: MarkupKind.PlainText },
                }).pipe(observeOn(asyncScheduler)),
        }),
        labeledProviderResults: labels => ({
            contents: labels.map(label => ({ value: label, kind: MarkupKind.PlainText })),
            alerts: [],
        }),
        providerWithImplementation: run => ({ provideHover: run } as sourcegraph.HoverProvider),
        getResult: (uri, extensionHostAPI) =>
            wrapRemoteObservable(
                extensionHostAPI.getHover({
                    textDocument: { uri },
                    position: { line: 1, character: 2 },
                })
            ),
        emptyResultValue: null,
    })
    testLocationProvider<sourcegraph.DefinitionProvider>({
        name: 'registerDefinitionProvider',
        registerProvider: extensionAPI => extensionAPI.languages.registerDefinitionProvider,
        labeledProvider: label => ({
            provideDefinition: (textDocument: sourcegraph.TextDocument, position: sourcegraph.Position) =>
                of([{ uri: new URL(`file:///${label}`) }]).pipe(observeOn(asyncScheduler)),
        }),
        labeledProviderResults: labeledDefinitionResults,
        providerWithImplementation: run => ({ provideDefinition: run } as sourcegraph.DefinitionProvider),
        getResult: (uri, extensionHostAPI) =>
            wrapRemoteObservable(
                extensionHostAPI.getDefinition({
                    textDocument: { uri },
                    position: { line: 1, character: 2 },
                })
            ),
        emptyResultValue: [],
    })
    testLocationProvider<sourcegraph.ReferenceProvider>({
        name: 'registerReferenceProvider',
        registerProvider: extensionAPI => extensionAPI.languages.registerReferenceProvider,
        labeledProvider: label => ({
            provideReferences: (
                textDocument: sourcegraph.TextDocument,
                position: sourcegraph.Position,
                context: sourcegraph.ReferenceContext
            ) => of([{ uri: new URL(`file:///${label}`) }]).pipe(observeOn(asyncScheduler)),
        }),
        labeledProviderResults: labels => labels.map(label => ({ uri: `file:///${label}`, range: undefined })),
        providerWithImplementation: run =>
            ({
                provideReferences: (
                    textDocument: sourcegraph.TextDocument,
                    position: sourcegraph.Position,
                    _context: sourcegraph.ReferenceContext
                ) => run(textDocument, position),
            } as sourcegraph.ReferenceProvider),
        getResult: (uri, extensionHostAPI) =>
            wrapRemoteObservable(
                extensionHostAPI.getReferences(
                    {
                        textDocument: { uri },
                        position: { line: 1, character: 2 },
                    },
                    { includeDeclaration: true }
                )
            ),
        emptyResultValue: [],
    })
    testLocationProvider<sourcegraph.LocationProvider>({
        name: 'registerLocationProvider',
        registerProvider: extensionAPI => (selector, provider) =>
            extensionAPI.languages.registerLocationProvider('x', selector, provider),
        labeledProvider: label => ({
            provideLocations: (textDocument: sourcegraph.TextDocument, position: sourcegraph.Position) =>
                of([{ uri: new URL(`file:///${label}`) }]).pipe(observeOn(asyncScheduler)),
        }),
        labeledProviderResults: labels => labels.map(label => ({ uri: `file:///${label}`, range: undefined })),
        providerWithImplementation: run =>
            ({
                provideLocations: (textDocument: sourcegraph.TextDocument, position: sourcegraph.Position) =>
                    run(textDocument, position),
            } as sourcegraph.LocationProvider),
        getResult: (uri, extensionHostAPI) =>
            wrapRemoteObservable(
                extensionHostAPI.getLocations('x', {
                    textDocument: { uri },
                    position: { line: 1, character: 2 },
                })
            ),
        emptyResultValue: [],
    })
})

/**
 * Generates test cases for sourcegraph.languages.registerXyzProvider functions and their associated
 * XyzProviders, for providers that return a list of locations.
 */
function testLocationProvider<P>({
    name,
    registerProvider,
    labeledProvider,
    labeledProviderResults,
    providerWithImplementation,
    getResult,
    emptyResultValue,
}: {
    name: keyof typeof sourcegraph.languages
    registerProvider: (
        extensionAPI: typeof sourcegraph
    ) => (selector: sourcegraph.DocumentSelector, provider: P) => sourcegraph.Unsubscribable
    labeledProvider: (label: string) => P
    labeledProviderResults: (labels: string[]) => any
    providerWithImplementation: (
        run: (textDocument: sourcegraph.TextDocument, position: sourcegraph.Position) => void
    ) => P
    getResult: (uri: string, extensionHostAPI: Remote<FlatExtensionHostAPI>) => Observable<MaybeLoadingResult<unknown>>
    emptyResultValue: unknown
}): void {
    describe(`languages.${name}`, () => {
        it('registers and unregisters a single provider', async () => {
            const { extensionAPI, extensionHostAPI } = await integrationTestContext()

            // Register the provider and call it.
            const subscription = registerProvider(extensionAPI)(['*'], labeledProvider('a'))
            await extensionAPI.internal.sync()
            expect(
                await getResult('file:///f', extensionHostAPI)
                    .pipe(
                        first(({ isLoading }) => !isLoading),
                        map(({ result }) => result)
                    )
                    .toPromise()
            ).toEqual(labeledProviderResults(['a']))

            // Unregister the provider and ensure it's removed.
            subscription.unsubscribe()
            expect(
                await getResult('file:///f', extensionHostAPI)
                    .pipe(
                        first(({ isLoading }) => !isLoading),
                        map(({ result }) => result)
                    )
                    .toPromise()
            ).toEqual(emptyResultValue)
        })

        it('syncs with models', async () => {
            const { extensionHostAPI, extensionAPI } = await integrationTestContext()

            const subscription = registerProvider(extensionAPI)(['*'], labeledProvider('a'))
            await extensionAPI.internal.sync()

            extensionHostAPI.addTextDocumentIfNotExists({
                uri: 'file:///f2',
                languageId: 'l1',
                text: 't1',
            })
            extensionHostAPI.addViewerIfNotExists({
                type: 'CodeEditor',
                resource: 'file:///f2',
                selections: [],
                isActive: true,
            })

            expect(
                await getResult('file:///f2', extensionHostAPI)
                    .pipe(
                        first(({ isLoading }) => !isLoading),
                        map(({ result }) => result)
                    )
                    .toPromise()
            ).toEqual(labeledProviderResults(['a']))

            subscription.unsubscribe()
        })

        it('supplies params to the provideXyz method', async () => {
            const { extensionHostAPI, extensionAPI } = await integrationTestContext()
            const { wait, done } = createBarrier()
            registerProvider(extensionAPI)(
                ['*'],
                providerWithImplementation((textDocument, position) => {
                    assertToJSON(textDocument, { uri: 'file:///f', languageId: 'l', text: 't' })
                    assertToJSON(position, { line: 1, character: 2 })
                    done()
                })
            )
            await extensionAPI.internal.sync()
            await getResult('file:///f', extensionHostAPI)
                .pipe(
                    first(({ isLoading }) => !isLoading),
                    map(({ result }) => result)
                )
                .toPromise()
            await wait
        })

        it('supports multiple providers', async () => {
            const { extensionHostAPI, extensionAPI } = await integrationTestContext()

            // Register 2 providers with different results.
            registerProvider(extensionAPI)(['*'], labeledProvider('a'))
            registerProvider(extensionAPI)(['*'], labeledProvider('b'))
            await extensionAPI.internal.sync()

            // Expect it to emit the first provider's result first (and not block on both providers being ready).
            expect(await getResult('file:///f', extensionHostAPI).pipe(take(3), toArray()).toPromise()).toEqual([
                { isLoading: true, result: emptyResultValue },
                { isLoading: true, result: labeledProviderResults(['a']) },
                { isLoading: false, result: labeledProviderResults(['a', 'b']) },
            ])
        })
    })
}

function labeledDefinitionResults(labels: string[]): Location[] {
    return labels.map(label => ({ uri: `file:///${label}`, range: undefined }))
}
