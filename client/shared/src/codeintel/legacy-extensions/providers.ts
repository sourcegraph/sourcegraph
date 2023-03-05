/* eslint-disable jsdoc/check-param-names */
import { Observable, of } from 'rxjs'

import * as sourcegraph from './api'
import * as indicators from './indicators'
import { LanguageSpec } from './language-specs/language-spec'
import { Logger, NoopLogger } from './logging'
import { createProviders as createLSIFProviders } from './lsif/providers'
import { createProviders as createSearchProviders } from './search/providers'
import { TelemetryEmitter } from './telemetry'
import { API } from './util/api'
import { asArray, mapArrayish, nonEmpty } from './util/helpers'
import { noopAsyncGenerator, observableFromAsyncIterator } from './util/ix'
import { parseGitURI } from './util/uri'

export interface Providers {
    definition: DefinitionProvider
    references: ReferencesProvider
    referencesPromise: ReferencesPromiseProvider
    hover: HoverProvider
    documentHighlights: DocumentHighlightProvider
}

export interface CombinedProviders {
    definitionAndHover: DefinitionAndHoverProvider
    references: ReferencesProvider
    implementations: LocationsProvider
    documentHighlights: DocumentHighlightProvider
}

export interface SourcegraphProviders {
    definition: sourcegraph.DefinitionProvider
    references: sourcegraph.ReferenceProvider
    implementations: sourcegraph.LocationProvider
    hover: sourcegraph.HoverProvider
    documentHighlights: sourcegraph.DocumentHighlightProvider
}

export const emptySourcegraphProviders: SourcegraphProviders = {
    definition: {
        provideDefinition() {
            return of(null)
        },
    },
    references: {
        provideReferences() {
            return of(null)
        },
    },
    implementations: {
        provideLocations() {
            return of(null)
        },
    },
    hover: {
        provideHover() {
            return of(null)
        },
    },
    documentHighlights: {
        provideDocumentHighlights() {
            return of(null)
        },
    },
}

export interface PromiseProviders {
    definition: (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ) => Promise<sourcegraph.Definition>
    references: (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ) => Promise<sourcegraph.Location[] | null>
    implementations: (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ) => Promise<sourcegraph.Location[] | null>
    hover: (textDocument: sourcegraph.TextDocument, position: sourcegraph.Position) => Promise<sourcegraph.Hover | null>
    documentHighlights: (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ) => Promise<sourcegraph.DocumentHighlight[] | null>
}

export interface DefinitionAndHover {
    definition: sourcegraph.Definition | null
    hover: sourcegraph.Hover | null
}

export type DefinitionAndHoverProvider = (
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position
) => Promise<DefinitionAndHover | null>

export type DefinitionProvider = (
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position
) => AsyncGenerator<sourcegraph.Definition, void, undefined>

export type ReferencesProvider = (
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position,
    context: sourcegraph.ReferenceContext
) => AsyncGenerator<sourcegraph.Location[] | null, void, undefined>

export type ReferencesPromiseProvider = (
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position,
    context: sourcegraph.ReferenceContext
) => Promise<sourcegraph.Location[]>

export type LocationsProvider = (
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position
) => AsyncGenerator<sourcegraph.Location[] | null, void, undefined>

export type HoverProvider = (
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position
) => AsyncGenerator<sourcegraph.Hover | null, void, undefined>

export type DocumentHighlightProvider = (
    textDocument: sourcegraph.TextDocument,
    position: sourcegraph.Position
) => AsyncGenerator<sourcegraph.DocumentHighlight[] | null, void, undefined>

export const noopProviders = {
    definitionAndHover: (): Promise<DefinitionAndHover | null> => Promise.resolve(null),
    definition: noopAsyncGenerator,
    references: noopAsyncGenerator,
    implementations: noopAsyncGenerator,
    hover: noopAsyncGenerator,
    documentHighlights: noopAsyncGenerator,
}

/**
 * Creates a SourcegraphProviders with LSIF and search-based behaviors.
 *
 * @param languageSpec The language spec used to provide search-based code intelligence.
 */
export function createProviders(
    languageSpec: LanguageSpec,
    hasImplementationsField: boolean,
    logger: Logger
): SourcegraphProviders {
    const wrapped: { definition?: sourcegraph.DefinitionProvider } = {}
    const api = new API()
    const lsifProviders = createLSIFProviders(hasImplementationsField, logger)
    const searchProviders = createSearchProviders(languageSpec, wrapped, api)

    const providerLogger =
        sourcegraph.getSetting<boolean>('codeIntel.traceExtension') ?? false ? console : new NoopLogger()

    // Register visible definition provider that does not
    // have any active telemetry. This is to reduce the double
    // count of definitions, which are triggered for search-based
    // hover text.
    wrapped.definition = createDefinitionProvider(
        lsifProviders.definitionAndHover,
        searchProviders.definition,
        providerLogger,
        languageSpec.languageID,
        true,
        api
    )

    return {
        // Return the provider with telemetry to use from the root
        definition: createDefinitionProvider(
            lsifProviders.definitionAndHover,
            searchProviders.definition,
            providerLogger,
            languageSpec.languageID,
            false,
            api
        ),

        references: createReferencesProvider(
            lsifProviders.references,
            searchProviders.references,
            providerLogger,
            languageSpec.languageID,
            api
        ),

        implementations: createImplementationsProvider(
            lsifProviders.implementations,
            providerLogger,
            languageSpec.languageID,
            api
        ),

        hover: createHoverProvider(
            lsifProviders.definitionAndHover,
            searchProviders.definition,
            searchProviders.hover,
            providerLogger,
            languageSpec.languageID,
            api
        ),

        documentHighlights: createDocumentHighlightProvider(
            lsifProviders.documentHighlights,
            searchProviders.documentHighlights,
            logger,
            languageSpec.languageID,
            api
        ),
    }
}

/**
 * Creates a definition provider.
 *
 * @param lsifProvider The LSIF-based definition and hover provider.
 * @param searchProvider The search-based definition provider.
 * @param logger The logger instance.
 * @param languageID The language the extension recognizes.
 * @param quiet Disable telemetry from this provider. Used for recursive calls from the hover provider when a definition location is required.
 */
export function createDefinitionProvider(
    lsifProvider: DefinitionAndHoverProvider,
    searchProvider: DefinitionProvider,
    logger?: Logger,
    languageID: string = '',
    quiet = false,
    api = new API()
): sourcegraph.DefinitionProvider {
    return {
        provideDefinition: wrapProvider(async function* (
            textDocument: sourcegraph.TextDocument,
            position: sourcegraph.Position
        ): AsyncGenerator<sourcegraph.Definition | undefined, void, undefined> {
            const { repo } = parseGitURI(new URL(textDocument.uri))
            const repoId = (await api.resolveRepo(repo)).id
            const emitter = new TelemetryEmitter(languageID, repoId, !quiet)
            const commonFields = { provider: 'definition', repo, textDocument, position, emitter, logger }
            const lsifWrapper = await lsifProvider(textDocument, position)
            let hasPreciseResult = false

            for await (const rawResults of asArray(lsifWrapper?.definition || [])) {
                // Mark new results as precise
                const aggregableBadges = [indicators.preciseBadge]
                const results = { ...rawResults, aggregableBadges }
                logLocationResults({ ...commonFields, action: 'lsifDefinitions', results })
                yield results
                hasPreciseResult = true
            }
            if (hasPreciseResult) {
                // Found the best precise definition we'll get. Stop.
                return
            }

            // No results so far, fall back to search if not disabled.
            if (sourcegraph.getSetting<boolean>('codeIntel.disableSearchBased') ?? false) {
                return
            }

            for await (const rawResult of searchProvider(textDocument, position)) {
                if (!nonEmpty(rawResult)) {
                    continue
                }

                // Mark new results as imprecise
                const badge = indicators.impreciseBadge
                const aggregableBadges = [indicators.searchBasedBadge]
                const results = mapArrayish(rawResult, location => ({ ...location, badge, aggregableBadges }))
                logLocationResults({ ...commonFields, action: 'searchDefinitions', results })
                yield results
            }
        }),
    }
}

/** Gets an opaque value that is the same for all locations within a file but different from other files. */
const file = (location: sourcegraph.Location): string => location.uri

/**
 * A cache of precise results from the previous references provider request. This is used to quick-start
 * the request that occurs after re-registering the references provider when the user config setting
 * mixPreciseAndSearchBasedReferences changes value.
 */
let lsifReferenceResultCache:
    | {
          textDocumentUri: string
          position: sourcegraph.Position
          lsifResults: sourcegraph.Location[]
      }
    | undefined

/**
 * Unsets lsifReferenceResultCache. Exported for testing.
 */
export function clearReferenceResultCache(): void {
    lsifReferenceResultCache = undefined
}

/**
 * Creates a reference provider.
 *
 * @param lsifProvider The LSIF-based references provider.
 * @param searchProvider The search-based references provider.
 * @param logger The logger instance.
 * @param languageID The language the extension recognizes.
 * @param api The Sourcegraph API instance.
 * @param shouldMixPreciseAndSearchBasedReferences is a function that returns whether
 * or not search-based results should be displayed when precise results are available.
 */
export function createReferencesProvider(
    lsifProvider: ReferencesProvider,
    searchProvider: ReferencesProvider,
    logger?: Logger,
    languageID: string = '',
    api = new API(),
    shouldMixPreciseAndSearchBasedReferences = (): boolean =>
        Boolean(sourcegraph.getSetting<boolean>('codeIntel.mixPreciseAndSearchBasedReferences') ?? false)
): sourcegraph.ReferenceProvider {
    return {
        provideReferences: wrapProvider(async function* (
            textDocument: sourcegraph.TextDocument,
            position: sourcegraph.Position,
            context: sourcegraph.ReferenceContext
        ): AsyncGenerator<sourcegraph.Location[] | null, void, undefined> {
            const { repo } = parseGitURI(new URL(textDocument.uri))
            const repoId = (await api.resolveRepo(repo)).id
            const emitter = new TelemetryEmitter(languageID, repoId)
            const commonFields = { repo, textDocument, position, emitter, logger, provider: 'references' }

            let cached = false
            let lsifResults: sourcegraph.Location[] = []
            if (
                lsifReferenceResultCache?.textDocumentUri === textDocument.uri &&
                lsifReferenceResultCache?.position.isEqual(position)
            ) {
                // If we just made this request we can re-use the results from the previous invocation
                // These results have already been badged and don't need to be logged twice. We'll just
                // set the local so that the results are used as if they were freshly generated by the
                // fallback logic.
                //
                // We're not going to emit these results eagerly. It feels like a better UX to batch the
                // lsif results to the first set of search results as they come back. In doing so, we need
                // to be sure that we actually do re-yield the lsifResults otherwise we may cache but never
                // actually display the precise results.
                cached = true
                lsifResults = lsifReferenceResultCache.lsifResults
            } else {
                for await (const rawResult of lsifProvider(textDocument, position, context)) {
                    if (!nonEmpty(rawResult)) {
                        continue
                    }

                    // Mark results as precise
                    const aggregableBadges = [indicators.preciseBadge]
                    lsifResults = asArray(rawResult).map(location => ({ ...location, aggregableBadges }))
                    logLocationResults({ ...commonFields, action: 'lsifReferences', results: lsifResults })
                    yield lsifResults
                }

                // Cache new precise results
                lsifReferenceResultCache = { textDocumentUri: textDocument.uri, position, lsifResults }
            }

            // If search-based is disabled or we have precise results and
            // mixPreciseAndSearchBasedReferences is disabled, do not fall back to display any additional
            // search-based results, regardless if it's from a file with no precise results.
            if (
                (sourcegraph.getSetting<boolean>('codeIntel.disableSearchBased') ?? false) ||
                (lsifResults.length > 0 && !shouldMixPreciseAndSearchBasedReferences())
            ) {
                if (cached) {
                    // If we haven't emitted lsif results yet, do so now
                    yield lsifResults
                }

                return
            }

            const lsifFiles = new Set(lsifResults.map(file))
            let hasSearchResults = false

            for await (const rawResults of searchProvider(textDocument, position, context)) {
                // Filter out any search results that occur in the same file as LSIF results. These
                // results are definitely incorrect and will pollute the ordering of precise and fuzzy
                // results in the references pane.
                const searchResults = asArray(rawResults).filter(location => !lsifFiles.has(file(location)))
                if (searchResults.length === 0) {
                    continue
                }

                hasSearchResults = true

                // Mark new results as imprecise, then append them to the previous result set. We need
                // to append here so that we do not overwrite what was emitted previously.
                const badge = indicators.impreciseBadge
                const aggregableBadges = [indicators.searchBasedBadge]
                const results = lsifResults.concat(
                    searchResults.map(location => ({ ...location, badge, aggregableBadges }))
                )
                logLocationResults({ ...commonFields, action: 'searchReferences', results })
                yield results
            }

            if (cached && !hasSearchResults) {
                // if lsifResults came from the cache and we never had any results come from
                // the search provider above, then we need to emit our cached precise results.
                yield lsifResults
            }
        }),
    }
}

/**
 * Creates an implementation provider.
 *
 * @param lsifProvider The LSIF-based implementations provider.
 * @param logger The logger instance.
 * @param languageID The language the extension recognizes.
 * @param api The Sourcegraph API instance.
 * or not search-based results should be displayed when precise results are available.
 */
export function createImplementationsProvider(
    lsifProvider: LocationsProvider,
    logger?: Logger,
    languageID: string = '',
    api = new API()
): sourcegraph.LocationProvider {
    return {
        provideLocations: wrapProvider(async function* (
            textDocument: sourcegraph.TextDocument,
            position: sourcegraph.Position
        ): AsyncGenerator<sourcegraph.Location[] | null, void, undefined> {
            const { repo } = parseGitURI(new URL(textDocument.uri))
            const repoId = (await api.resolveRepo(repo)).id
            const emitter = new TelemetryEmitter(languageID, repoId)
            const commonFields = { repo, textDocument, position, emitter, logger, provider: 'implementations' }

            let lsifResults: sourcegraph.Location[] = []
            for await (const rawResult of lsifProvider(textDocument, position)) {
                if (!nonEmpty(rawResult)) {
                    continue
                }

                // Mark results as precise
                const aggregableBadges = [indicators.preciseBadge]
                lsifResults = asArray(rawResult).map(location => ({ ...location, aggregableBadges }))
                logLocationResults({ ...commonFields, action: 'lsifImplementations', results: lsifResults })
                yield lsifResults
            }
        }),
    }
}

/** logLocationResults emits telemetry events and emits location counts to the debug logger. */
function logLocationResults<T extends sourcegraph.Badged<sourcegraph.Location>, R extends T | T[] | null>({
    provider,
    action,
    repo,
    textDocument,
    position,
    results,
    emitter,
    logger,
}: {
    provider: string
    action: string
    repo: string
    textDocument: sourcegraph.TextDocument
    position: sourcegraph.Position
    results: R
    emitter?: TelemetryEmitter
    logger?: Logger
}): void {
    emitter?.emitOnce(action)

    // Emit xrepo event if we contain a result from another repository
    if (asArray(results).some(location => parseGitURI(new URL(location.uri)).repo !== repo)) {
        emitter?.emitOnce(action + '.xrepo')
    }

    if (logger) {
        let arrayResults = asArray(results)
        const totalCount = arrayResults.length
        const searchCount = arrayResults.reduce(
            (count, result) => count + (result.aggregableBadges?.some(badge => badge.text === 'search-based') ? 1 : 0),
            0
        )

        if (arrayResults.length > 500) {
            arrayResults = arrayResults.slice(0, 500)
        }

        const { path } = parseGitURI(new URL(textDocument.uri))
        const { line, character } = position

        logger.log({
            provider,
            path,
            line,
            character,
            preciseCount: totalCount - searchCount,
            searchCount,
            results: arrayResults.map(result => ({
                uri: result.uri.toString(),
                badges: result.aggregableBadges?.map(badge => badge.text),
                ...result.range,
            })),
        })
    }
}

/**
 * Creates a hover provider.
 *
 * @param lsifProvider The LSIF-based definition and hover provider.
 * @param searchDefinitionProvider The search-based definition provider.
 * @param searchHoverProvider The search-based hover provider.
 * @param logger The logger instance.
 * @param languageID The language the extension recognizes.
 */
export function createHoverProvider(
    lsifProvider: DefinitionAndHoverProvider,
    searchDefinitionProvider: DefinitionProvider,
    searchHoverProvider: HoverProvider,
    logger?: Logger,
    languageID: string = '',
    api = new API()
): sourcegraph.HoverProvider {
    return {
        provideHover: wrapProvider(async function* (
            textDocument: sourcegraph.TextDocument,
            position: sourcegraph.Position
        ): AsyncGenerator<sourcegraph.Badged<sourcegraph.Hover> | null | undefined, void, undefined> {
            const { repo, path } = parseGitURI(new URL(textDocument.uri))
            const repoId = (await api.resolveRepo(repo)).id
            const emitter = new TelemetryEmitter(languageID, repoId)
            const commonLogFields = { path, line: position.line, character: position.character }
            const lsifWrapper = await lsifProvider(textDocument, position)

            if (lsifWrapper?.hover) {
                // We have a precise hover, but we might not have a precise definition.
                // We want to tell the difference here, otherwise the definition may take
                // the user to an unrelated location without making it obvious that it's
                // not a precise result.

                let hasSearchBasedDefinition = false
                if (!nonEmpty(lsifWrapper.definition)) {
                    for await (const searchResult of searchDefinitionProvider(textDocument, position)) {
                        if (nonEmpty(searchResult)) {
                            hasSearchBasedDefinition = true
                            break
                        }
                    }
                }

                emitter.emitOnce('lsifHover')
                logger?.log({ provider: 'hover', precise: true, ...commonLogFields })
                yield badgeHoverResult(
                    lsifWrapper.hover,
                    // Display a partial data badge when there is a precise hover text but a
                    // search definition. This can happen if we haven't indexed the target
                    // repository, but the dependent repository still has hover information for
                    // externally defined symbols.
                    [!hasSearchBasedDefinition ? indicators.preciseBadge : indicators.partialHoverNoDefinitionBadge]
                )

                // Found the best precise hover text we'll get. Stop.
                return
            }

            const hasPreciseDefinition = nonEmpty(lsifWrapper?.definition)

            // No results so far, fall back to search if not disabled.
            if (sourcegraph.getSetting<boolean>('codeIntel.disableSearchBased') ?? false) {
                return
            }

            for await (const searchResult of searchHoverProvider(textDocument, position)) {
                if (!searchResult) {
                    continue
                }

                emitter.emitOnce('searchHover')
                logger?.log({ provider: 'hover', precise: false, ...commonLogFields })

                if (hasPreciseDefinition) {
                    // We have a precise definition but imprecise hover text
                    const aggregableBadges = [indicators.partialDefinitionNoHoverBadge]
                    yield badgeHoverResult(searchResult, aggregableBadges)
                    continue
                }

                // Only search results for this token
                const aggregableBadges = [indicators.searchBasedBadge]
                yield badgeHoverResult(searchResult, aggregableBadges)
            }
        }),
    }
}

function badgeHoverResult(
    hover: sourcegraph.Hover,
    aggregableBadges?: sourcegraph.AggregableBadge[]
): sourcegraph.Badged<sourcegraph.Hover> {
    return { ...hover, ...(aggregableBadges ? { aggregableBadges } : {}) }
}

/**
 * Creates a document highlight provider.
 *
 * @param lsifProvider The LSIF-based document highlight provider.
 * @param logger The logger instance.
 * @param languageID The language the extension recognizes.
 */
export function createDocumentHighlightProvider(
    lsifProvider: DocumentHighlightProvider,
    searchProvider: DocumentHighlightProvider,
    logger?: Logger,
    languageID: string = '',
    api = new API()
): sourcegraph.DocumentHighlightProvider {
    return {
        provideDocumentHighlights: wrapProvider(async function* (
            textDocument: sourcegraph.TextDocument,
            position: sourcegraph.Position
        ): AsyncGenerator<sourcegraph.DocumentHighlight[] | null | undefined, void, undefined> {
            const { repo } = parseGitURI(new URL(textDocument.uri))
            const repoId = (await api.resolveRepo(repo)).id
            const emitter = new TelemetryEmitter(languageID, repoId)

            let hasLsifResults = false

            for await (const lsifResult of lsifProvider(textDocument, position)) {
                if (lsifResult) {
                    emitter.emitOnce('lsifDocumentHighlight')
                    yield lsifResult
                    hasLsifResults = true
                }
            }

            if (!hasLsifResults) {
                emitter.emitOnce('searchDocumentHighlight')
                for await (const searchResult of searchProvider(textDocument, position)) {
                    if (searchResult) {
                        yield searchResult
                    }
                }
            }
        }),
    }
}

/**
 * Converts an async generator provider into an observable provider.
 *
 * @param func A function that returns a the provider.
 */
function wrapProvider<P extends unknown[], R>(
    func: (...args: P) => AsyncGenerator<R, void, void>
): (...args: P) => Observable<R> {
    return (...args) => observableFromAsyncIterator(() => func(...args))
}
