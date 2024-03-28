import { castArray } from 'lodash'
import { from, of, lastValueFrom } from 'rxjs'
import { defaultIfEmpty, map } from 'rxjs/operators'

import {
    fromHoverMerged,
    type HoverMerged,
    type TextDocumentIdentifier,
    type TextDocumentPositionParameters,
} from '@sourcegraph/client-api'
import type { MaybeLoadingResult } from '@sourcegraph/codeintellify'
// eslint-disable-next-line no-restricted-imports
import { isDefined } from '@sourcegraph/common/src/types'
import type * as clientType from '@sourcegraph/extension-api-types'

import { match } from '../api/client/types/textDocument'
import type { CodeIntelExtensionHostAPI, FlatExtensionHostAPI, ScipParameters } from '../api/contract'
import { proxySubscribable } from '../api/extension/api/common'
import { toPosition } from '../api/extension/api/types'
import { getModeFromPath } from '../languages'
import { parseRepoGitURI } from '../util/url'

import type { DocumentSelector, TextDocument, DocumentHighlight } from './legacy-extensions/api'
import * as sourcegraph from './legacy-extensions/api'
import type { LanguageSpec } from './legacy-extensions/language-specs/language-spec'
import { languageSpecs } from './legacy-extensions/language-specs/languages'
import { RedactingLogger } from './legacy-extensions/logging'
import { createProviders, emptySourcegraphProviders, type SourcegraphProviders } from './legacy-extensions/providers'
import type { Occurrence } from './scip'

export interface CodeIntelAPI {
    hasReferenceProvidersForDocument(textParameters: TextDocumentPositionParameters): Promise<boolean>
    getDefinition(
        textParameters: TextDocumentPositionParameters,
        scipParameters?: ScipParameters
    ): Promise<clientType.Location[]>
    getReferences(
        textParameters: TextDocumentPositionParameters,
        context: sourcegraph.ReferenceContext,
        scipParameters?: ScipParameters
    ): Promise<clientType.Location[]>
    getImplementations(parameters: TextDocumentPositionParameters): Promise<clientType.Location[]>
    getHover(textParameters: TextDocumentPositionParameters): Promise<HoverMerged | null>
    getDocumentHighlights(textParameters: TextDocumentPositionParameters): Promise<DocumentHighlight[]>
}

export function createCodeIntelAPI(context: sourcegraph.CodeIntelContext): CodeIntelAPI {
    sourcegraph.updateCodeIntelContext(context)
    return new DefaultCodeIntelAPI()
}

class DefaultCodeIntelAPI implements CodeIntelAPI {
    private locationResult(
        locations: sourcegraph.ProviderResult<sourcegraph.Definition>
    ): Promise<clientType.Location[]> {
        return lastValueFrom(
            locations.pipe(
                defaultIfEmpty(undefined),
                map(result =>
                    castArray(result)
                        .filter(isDefined)
                        .map(location => ({ ...location, uri: location.uri.toString() }))
                )
            ),
            { defaultValue: [] }
        )
    }

    public hasReferenceProvidersForDocument(textParameters: TextDocumentPositionParameters): Promise<boolean> {
        const document = toTextDocument(textParameters.textDocument)
        const providers = findLanguageMatchingDocument(document)?.providers
        return Promise.resolve(!!providers)
    }
    public async getReferences(
        textParameters: TextDocumentPositionParameters,
        context: sourcegraph.ReferenceContext,
        scipParameters?: ScipParameters
    ): Promise<clientType.Location[]> {
        const locals = scipParameters ? localReferences(scipParameters) : []
        if (locals.length > 0) {
            return locals.map(local => ({ uri: textParameters.textDocument.uri, range: local.range }))
        }
        const request = requestFor(textParameters)
        return this.locationResult(
            request.providers.references.provideReferences(request.document, request.position, context)
        )
    }
    public async getDefinition(
        textParameters: TextDocumentPositionParameters,
        scipParameters?: ScipParameters
    ): Promise<clientType.Location[]> {
        const definitions = scipParameters ? localDefinition(scipParameters) : []
        if (definitions.length > 0) {
            return definitions.map(definition => ({ uri: textParameters.textDocument.uri, range: definition.range }))
        }
        const request = requestFor(textParameters)
        return this.locationResult(request.providers.definition.provideDefinition(request.document, request.position))
    }
    public async getImplementations(textParameters: TextDocumentPositionParameters): Promise<clientType.Location[]> {
        const request = requestFor(textParameters)
        return this.locationResult(
            request.providers.implementations.provideLocations(request.document, request.position)
        )
    }
    public getHover(textParameters: TextDocumentPositionParameters): Promise<HoverMerged | null> {
        const request = requestFor(textParameters)
        return lastValueFrom(
            request.providers.hover
                .provideHover(request.document, request.position)
                // We intentionally don't use `defaultIfEmpty()` here because
                // that makes the popover load with an empty docstring.
                .pipe(map(result => fromHoverMerged([result]))),
            { defaultValue: null }
        )
    }
    public getDocumentHighlights(textParameters: TextDocumentPositionParameters): Promise<DocumentHighlight[]> {
        const request = requestFor(textParameters)
        return lastValueFrom(
            request.providers.documentHighlights.provideDocumentHighlights(request.document, request.position).pipe(
                defaultIfEmpty(undefined),
                map(result => result || [])
            ),
            { defaultValue: [] }
        )
    }
}

interface LanguageRequest {
    providers: SourcegraphProviders
    document: sourcegraph.TextDocument
    position: sourcegraph.Position
}

function requestFor(textParameters: TextDocumentPositionParameters): LanguageRequest {
    const document = toTextDocument(textParameters.textDocument)
    return {
        document,
        position: toPosition(textParameters.position),
        providers: findLanguageMatchingDocument(document)?.providers || emptySourcegraphProviders,
    }
}

function toTextDocument(textDocument: TextDocumentIdentifier): sourcegraph.TextDocument {
    return {
        uri: textDocument.uri,
        languageId: getModeFromPath(parseRepoGitURI(textDocument.uri).filePath || ''),
        text: undefined,
    }
}

export function findLanguageMatchingDocument(textDocument: TextDocumentIdentifier): Language | undefined {
    const document: Pick<TextDocument, 'uri' | 'languageId'> = toTextDocument(textDocument)
    for (const language of languages) {
        if (match(language.selector, document)) {
            return language
        }
    }
    return undefined
}

interface Language {
    spec: LanguageSpec
    selector: DocumentSelector
    providers: SourcegraphProviders
}
const hasImplementationsField = true
const languages: Language[] = languageSpecs.map(spec => ({
    spec,
    selector: selectorForSpec(spec),
    providers: createProviders(spec, hasImplementationsField, new RedactingLogger(console)),
}))

// Returns true if the provided language supports "Find implementations"
export function hasFindImplementationsSupport(language: string): boolean {
    for (const spec of languageSpecs) {
        if (spec.languageID === language) {
            return spec.textDocumentImplemenationSupport ?? false
        }
    }
    return false
}

function selectorForSpec(languageSpec: LanguageSpec): DocumentSelector {
    return [
        { language: languageSpec.languageID },
        ...(languageSpec.verbatimFilenames || []).flatMap(filename => [{ pattern: filename }]),
        ...languageSpec.fileExts.flatMap(extension => [{ pattern: `*.${extension}` }]),
    ]
}

// Replaces codeintel functions from the "old" extension/webworker extension API
// with new implementations of code that lives in this repository. The old
// implementation invoked codeintel functions via webworkers, and the codeintel
// implementation lived in a separate repository
// https://github.com/sourcegraph/code-intel-extensions Ideally, we should
// update all the usages of `comlink.Remote<FlatExtensionHostAPI>` with the new
// `CodeIntelAPI` interfaces, but that would require refactoring a lot of files.
// To minimize the risk of breaking changes caused by the deprecation of
// extensions, we monkey patch the old implementation with new implementations.
// The benefit of monkey patching is that we can optionally disable if for
// customers that choose to enable the legacy extensions.
//
// TODO(camdencheek): USE THIS to patch code intel into extensions
export function injectNewCodeintel(
    old: FlatExtensionHostAPI,
    codeintelContext: sourcegraph.CodeIntelContext
): FlatExtensionHostAPI {
    const api = createCodeIntelAPI(codeintelContext)
    const codeintelOverrides = newCodeIntelExtensionHostAPI(api)
    return { ...old, ...codeintelOverrides }
}

export function newCodeIntelExtensionHostAPI(codeintel: CodeIntelAPI): CodeIntelExtensionHostAPI {
    function thenMaybeLoadingResult<T>(result: T): MaybeLoadingResult<T> {
        return { isLoading: false, result }
    }

    return {
        hasReferenceProvidersForDocument(textParameters) {
            return proxySubscribable(from(codeintel.hasReferenceProvidersForDocument(textParameters)))
        },
        getLocations(id, parameters) {
            if (!id.startsWith('implementations_')) {
                return proxySubscribable(of({ isLoading: false, result: [] }))
            }
            return proxySubscribable(from(codeintel.getImplementations(parameters).then(thenMaybeLoadingResult)))
        },
        getDefinition(parameters) {
            return proxySubscribable(from(codeintel.getDefinition(parameters).then(thenMaybeLoadingResult)))
        },
        getReferences(parameters, context, scipParameters) {
            return proxySubscribable(
                from(codeintel.getReferences(parameters, context, scipParameters).then(thenMaybeLoadingResult))
            )
        },
        getDocumentHighlights: (textParameters: TextDocumentPositionParameters) =>
            proxySubscribable(from(codeintel.getDocumentHighlights(textParameters))),
        getHover: (textParameters: TextDocumentPositionParameters) =>
            proxySubscribable(from(codeintel.getHover(textParameters).then(thenMaybeLoadingResult))),
    }
}

export function localReferences(params: ScipParameters): Occurrence[] {
    if (params.referenceOccurrence.symbol?.startsWith('local ')) {
        return params.documentOccurrences.filter(occurrence => occurrence.symbol === params.referenceOccurrence.symbol)
    }
    return []
}

function localDefinition(params: ScipParameters): Occurrence[] {
    if (!params.referenceOccurrence.symbol) {
        return []
    }
    return params.documentOccurrences.filter(
        definitionOccurrence =>
            definitionOccurrence.symbol === params.referenceOccurrence.symbol &&
            definitionOccurrence.symbolRoles &&
            (definitionOccurrence.symbolRoles & 1) === 1
    )
}
