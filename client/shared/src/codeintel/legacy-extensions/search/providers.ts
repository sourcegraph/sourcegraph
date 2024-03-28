/* eslint-disable jsdoc/check-param-names */
import { flatten, sortBy } from 'lodash'
import { firstValueFrom, from, isObservable } from 'rxjs'

import * as sourcegraph from '../api'
import type { FilterDefinitions, LanguageSpec } from '../language-specs/language-spec'
import type { Providers } from '../providers'
import { cache } from '../util'
import { API } from '../util/api'
import { asArray, isDefined } from '../util/helpers'
import { asyncGeneratorFromPromise } from '../util/ix'
import { raceWithDelayOffset } from '../util/promise'
import { parseGitURI } from '../util/uri'

import { getConfig } from './config'
import { type Result, resultToLocation, searchResultToResults } from './conversion'
import { findDocstring } from './docstrings'
import { wrapIndentationInCodeBlocks } from './markdown'
import { definitionQuery, referencesQuery } from './queries'
import { mkSquirrel } from './squirrel'
import { findSearchToken } from './tokens'

/** The number of files whose content can be cached at once. */
const FILE_CONTENT_CACHE_CAPACITY = 20

/**
 * Creates providers powered by search-based code intelligence.
 *
 * @param spec The language spec.
 * @param wrappedProviders A reference to the currently active top-level providers.
 * @param api The GraphQL API instance.
 */
export function createProviders(
    {
        languageID,
        fileExts: fileExtensions = [],
        commentStyles,
        identCharPattern,
        filterDefinitions = results => results,
    }: LanguageSpec,
    wrappedProviders: { definition?: sourcegraph.DefinitionProvider },
    api: API = new API()
): Providers {
    /* A small (randomly evicting) cache from URIs to file contents. */
    const cachedFileContents = new Map<string, Promise<string | undefined>>()

    /** Retrieves the text of the current text document. */
    const getFileContent = (uri: string): Promise<string | undefined> => {
        const cachedFileContent = cachedFileContents.get(uri)
        if (cachedFileContent !== undefined) {
            return cachedFileContent
        }

        if (cachedFileContents.size > FILE_CONTENT_CACHE_CAPACITY) {
            // Remove a random entry from map. This saves us from having to
            // keep track of frequency or recency information and is likely
            // to be a decent heuristic (with rare back-to-back evictions).
            const index = Math.floor(Math.random() * cachedFileContents.size)
            cachedFileContents.delete(Array.from(cachedFileContents.keys())[index])
        }

        const { repo, commit, path } = parseGitURI(uri)
        const fileContent = api.getFileContent(repo, commit, path)
        cachedFileContents.set(uri, fileContent)
        return fileContent
    }

    /**
     * Return the text document content and the search token found under the
     * current hover position. Returns undefined if either piece of data could
     * not be determined.
     *
     * @param textDocument The current text document.
     * @param position The current hover position.
     */
    const getContentAndToken = async (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ): Promise<{ text: string; searchToken: string } | undefined> => {
        const text = await getFileContent(textDocument.uri)
        if (!text) {
            return undefined
        }

        const tokenResult = findSearchToken({
            text,
            position,
            lineRegexes: commentStyles.map(style => style.lineRegex).filter(isDefined),
            blockCommentStyles: commentStyles.map(style => style.block).filter(isDefined),
            identCharPattern,
        })
        if (!tokenResult || tokenResult.isString || tokenResult.isComment) {
            return undefined
        }

        return { text, searchToken: tokenResult.searchToken }
    }

    const squirrel = mkSquirrel(api)

    /**
     * Retrieve a definition for the current hover position.
     *
     * @param textDocument The current text document.
     * @param position The current hover position.
     */
    const definition = async (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ): Promise<sourcegraph.Definition> => {
        const squirrelDefinition = await squirrel.definition(textDocument, position)
        if (squirrelDefinition) {
            return squirrelDefinition
        }

        const contentAndToken = await getContentAndToken(textDocument, position)
        if (!contentAndToken) {
            return null
        }
        const { text, searchToken } = contentAndToken
        const { repo, commit, path } = parseGitURI(textDocument.uri)
        const { isFork, isArchived } = await api.resolveRepo(repo)

        // Construct base definition query without scoping terms
        const queryTerms = definitionQuery({ searchToken, doc: textDocument, fileExts: fileExtensions })
        const queryArguments = {
            doc: textDocument,
            repo,
            isFork,
            isArchived,
            commit,
            path,
            text,
            filterDefinitions,
            queryTerms,
        }

        const doSearch = (negateRepoFilter: boolean): Promise<sourcegraph.Location[]> =>
            searchWithFallback(args => searchAndFilterDefinitions(api, args), queryArguments, negateRepoFilter)

        // Perform a search in the current git tree
        const sameRepoDefinitions = doSearch(false)

        // Return any local location definitions first
        const results = await sameRepoDefinitions
        if (results.length > 0) {
            return results
        }

        // Fallback to definitions found in any other repository. This performs
        // an indexed search over all repositories.
        return !getConfig('basicCodeIntel.globalSearchesEnabled', true) ? Promise.resolve([]) : doSearch(true)
    }

    /**
     * Retrieve references for the current hover position.
     *
     * @param textDocument The current text document.
     * @param position The current hover position.
     */
    const references = async (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ): Promise<sourcegraph.Location[]> => {
        const squirrelReferences = (await squirrel.references(textDocument, position)) ?? []
        if (squirrelReferences.length > 0) {
            return squirrelReferences
        }

        const contentAndToken = await getContentAndToken(textDocument, position)
        if (!contentAndToken) {
            return []
        }
        const { searchToken } = contentAndToken
        const { repo, commit } = parseGitURI(textDocument.uri)
        const { isFork, isArchived } = await api.resolveRepo(repo)

        // Construct base references query without scoping terms
        const queryTerms = referencesQuery({ searchToken, doc: textDocument, fileExts: fileExtensions })
        const queryArguments = {
            repo,
            isFork,
            isArchived,
            commit,
            queryTerms,
        }

        const doSearch = (negateRepoFilter: boolean): Promise<sourcegraph.Location[]> =>
            searchWithFallback(args => searchReferences(api, args), queryArguments, negateRepoFilter)

        // Perform a search in the current git tree, suppressing references within the current file when
        // we have squirrel results
        const sameRepoReferences = doSearch(false)

        // Perform an indexed search over all _other_ repositories.
        const remoteRepoReferences = !getConfig('basicCodeIntel.globalSearchesEnabled', true)
            ? Promise.resolve([])
            : doSearch(true)

        // Resolve then merge all references and sort them by proximity
        // to the current text document path.
        const referenceChunk = [sameRepoReferences, remoteRepoReferences]
        const mergedReferences = flatten(await Promise.all(referenceChunk))
        return sortByProximity(mergedReferences, new URL(textDocument.uri))
    }

    /**
     * Retrieve hover text for the current hover position.
     *
     * @param textDocument The current text document.
     * @param position The current hover position.
     */
    const hover = async (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ): Promise<sourcegraph.Hover | null> => {
        const squirrelHover = await squirrel.hover(textDocument, position)
        if (squirrelHover) {
            return squirrelHover
        }

        if (!wrappedProviders.definition) {
            return null
        }

        // Find the definition for this position. This will generally fall back
        // to our sibling search-based definition provider defined just above,
        // but may fall-"up" to the LSIF providers when we have an indexer that
        // provides definitions but not hover text. This will allow us to get
        // precise hover text (if it's extractable) if we just fall-"sideways"
        // to the search-based definition provider as we've done historically.
        const result = wrappedProviders.definition?.provideDefinition(textDocument, position)

        // The providers created by the non-noop provider wrapper in this repo
        // always returns an observable. If we have something else early-out.
        if (!result || !isObservable(result)) {
            return null
        }

        // Get the first definition and ensure it has a range
        const def = asArray(await firstValueFrom(from(result), { defaultValue: undefined }))[0]
        if (!def?.range) {
            return null
        }

        const text = await getFileContent(def.uri.href)
        if (!text) {
            return null
        }

        // Get the definition line
        const line = text.split('\n')[def.range.start.line]
        if (!line) {
            return null
        }

        // Clean up the line by removing punctuation from the right end
        const trimmedLine = line.trim().replace(/[(,:;<={]+$/, '')

        if (trimmedLine.includes('```')) {
            // Don't render the line if it breaks out of the Markdown
            // block that wraps content in the web UI.
            return null
        }

        // Render the line as syntax-highlighted Markdown
        const codeLineMarkdown = '```' + languageID + '\n' + trimmedLine + '\n```'

        const docstring = findDocstring({
            definitionLine: def.range.start.line,
            fileText: text,
            commentStyles,
        })

        const docstringMarkdown = docstring && wrapIndentationInCodeBlocks(languageID, docstring)

        return {
            contents: {
                kind: sourcegraph.MarkupKind.Markdown,
                value: [codeLineMarkdown, docstringMarkdown].filter(isDefined).join('\n\n---\n\n'),
            },
        }
    }

    const documentHighlights = async (
        textDocument: sourcegraph.TextDocument,
        position: sourcegraph.Position
    ): Promise<sourcegraph.DocumentHighlight[] | null> => {
        const squirrelDocumentHighlights = await squirrel.documentHighlights(textDocument, position)
        if (squirrelDocumentHighlights) {
            return squirrelDocumentHighlights
        }
        return null
    }

    return {
        definition: asyncGeneratorFromPromise(cache(definition, { max: 5 })),
        references: asyncGeneratorFromPromise(references),
        referencesPromise: references,
        hover: asyncGeneratorFromPromise(hover),
        documentHighlights: asyncGeneratorFromPromise(documentHighlights),
    }
}

/**
 * Perform a search query for definitions. Returns results converted to locations,
 * filtered by the language's definition filter, and sorted by proximity to the
 * current text document path.
 *
 * @param api The GraphQL API instance.
 * @param args Parameter bag.
 */
async function searchAndFilterDefinitions(
    api: API,
    {
        doc,
        repo,
        path,
        text,
        filterDefinitions,
        queryTerms,
    }: {
        /** The current text document. */
        doc: sourcegraph.TextDocument
        /** The repository containing the current text document. */
        repo: string
        /** Whether the repository containing the current text document is a fork. */
        isFork: boolean
        /** Whether the repository containing the current text document is archived. */
        isArchived: boolean
        /** The path of the current text document. */
        path: string
        /** The content of the current text document */
        text: string
        /** The function used to filter definitions. */
        filterDefinitions: FilterDefinitions
        /** The terms of the search query. */
        queryTerms: string[]
    }
): Promise<sourcegraph.Location[]> {
    // Perform search and perform pre-filtering before passing it
    // off to the language spec for the proper filtering pass.
    const searchResults = await search(api, queryTerms)
    const preFilteredResults = searchResults.filter(result => !isExternalPrivateSymbol(doc, path, result))

    // Filter results based on language spec
    const filteredResults = filterDefinitions(preFilteredResults, {
        repo,
        filePath: path,
        fileContent: text,
    })

    return sortByProximity(filteredResults.map(resultToLocation), new URL(doc.uri))
}

/**
 * Perform a search query for references. Returns results converted to locations.
 * Results are not sorted in any meaningful way as these results are meant to be
 * merged with other search query results.
 *
 * @param api The GraphQL API instance.
 * @param args Parameter bag.
 */
async function searchReferences(
    api: API,
    {
        queryTerms,
    }: {
        /** The terms of the search query. */
        queryTerms: string[]
    }
): Promise<sourcegraph.Location[]> {
    return (await search(api, queryTerms)).map(resultToLocation)
}

/** The time in ms to delay between unindexed search request and the fallback indexed search request. */
const DEFAULT_UNINDEXED_SEARCH_TIMEOUT_MS = 5000

/**
 * Invoke the given search function by modifying the query with a term that will
 * only look in the current git tree by appending a repo filter with the repo name
 * and the current commit or, if `negateRepoFilter` is set, outside of current git
 * tree.
 *
 * This is likely to timeout on large repos or organizations with monorepos if the
 * current commit is not an indexed commit. Instead of waiting for a timeout, we
 * will start a second index-only search of the HEAD commit for the same repo after
 * a short delay.
 *
 * This function returns the set of results that resolve first.
 *
 * @param search The search function.
 * @param args The arguments to the search function.
 * @param negateRepoFilter Whether to look only inside or outside the given repo.
 */
export function searchWithFallback<
    P extends {
        repo: string
        isFork: boolean
        isArchived: boolean
        commit: string
        queryTerms: string[]
    },
    R
>(search: (args: P) => Promise<R>, args: P, negateRepoFilter = false): Promise<R> {
    if (getConfig('basicCodeIntel.indexOnly', false)) {
        return searchIndexed(search, args, negateRepoFilter)
    }

    return raceWithDelayOffset(
        searchUnindexed(search, args, negateRepoFilter),
        () => searchIndexed(search, args, negateRepoFilter),
        getConfig('basicCodeIntel.unindexedSearchTimeout', DEFAULT_UNINDEXED_SEARCH_TIMEOUT_MS)
    )
}

/**
 * Invoke the given search function as an indexed-only (fast, imprecise) search.
 *
 * @param search The search function.
 * @param args The arguments to the search function.
 * @param negateRepoFilter Whether to look only inside or outside the given repo.
 */
function searchIndexed<
    P extends {
        repo: string
        isFork: boolean
        isArchived: boolean
        commit: string
        queryTerms: string[]
    },
    R
>(search: (args: P) => Promise<R>, args: P, negateRepoFilter = false): Promise<R> {
    const { repo, isFork, isArchived, queryTerms } = args

    // Create a copy of the args so that concurrent calls to other
    // search methods do not have their query terms unintentionally
    // modified.
    const queryTermsCopy = [...queryTerms]

    // Unlike unindexed search, we can't supply a commit as that particular
    // commit may not be indexed. We force index and look inside/outside
    // the repo at _whatever_ commit happens to be indexed at the time.
    queryTermsCopy.push((negateRepoFilter ? '-' : '') + `repo:${makeRepositoryPattern(repo)}`)
    queryTermsCopy.push('index:only')

    // If we're a fork, search in forks _for the same repo_. Otherwise,
    // search in forks only if it's set in the settings. This is also
    // symmetric for archived repositories.
    queryTermsCopy.push(...repositoryKindTerms(isFork && !negateRepoFilter, isArchived && !negateRepoFilter))

    return search({ ...args, queryTerms: queryTermsCopy })
}

/**
 * Invoke the given search function as an unindexed (slow, precise) search.
 *
 * @param search The search function.
 * @param args The arguments to the search function.
 * @param negateRepoFilter Whether to look only inside or outside the given repo.
 */
function searchUnindexed<
    P extends {
        repo: string
        isFork: boolean
        isArchived: boolean
        commit: string
        queryTerms: string[]
    },
    R
>(search: (args: P) => Promise<R>, args: P, negateRepoFilter = false): Promise<R> {
    const { repo, isFork, isArchived, commit, queryTerms } = args

    // Create a copy of the args so that concurrent calls to other
    // search methods do not have their query terms unintentionally
    // modified.
    const queryTermsCopy = [...queryTerms]

    if (!negateRepoFilter) {
        // Look in this commit only
        queryTermsCopy.push(`repo:${makeRepositoryPattern(repo)}@${commit}`)
    } else {
        // Look outside the repo (not outside the commit)
        queryTermsCopy.push(`-repo:${makeRepositoryPattern(repo)}`)
    }

    // If we're a fork, search in forks _for the same repo_. Otherwise,
    // search in forks only if it's set in the settings. This is also
    // symmetric for archived repositories.
    queryTermsCopy.push(...repositoryKindTerms(isFork && !negateRepoFilter, isArchived && !negateRepoFilter))

    return search({ ...args, queryTerms: queryTermsCopy })
}

/**
 * Perform a search query.
 *
 * @param api The GraphQL API instance.
 * @param queryTerms The terms of the search query.
 */
async function search(api: API, queryTerms: string[]): Promise<Result[]> {
    return (await api.search(queryTerms.join(' '), getConfig('fileLocal', false))).flatMap(searchResultToResults)
}

/**
 * Report whether the given symbol is both private and does not belong to
 * the current text document.
 *
 * @param textDocument The current text document.
 * @param path The path of the document.
 * @param result The search result.
 */
function isExternalPrivateSymbol(
    textDocument: sourcegraph.TextDocument,
    path: string,
    { fileLocal, file, symbolKind }: Result
): boolean {
    // Enum members are always public, but there's an open ctags bug that
    // doesn't let us treat that way.
    // See https://github.com/universal-ctags/ctags/issues/1844

    if (textDocument.languageId === 'java' && symbolKind === 'ENUMMEMBER') {
        return false
    }

    return !!fileLocal && file !== path
}

/**
 * Sort the locations by their uri field's similarity to the current text
 * document URI. This is done by applying a similarity coefficient to the
 * segments of each file path. Paths with more segments in common will
 * have a higher similarity coefficient.
 *
 * @param locations A list of locations to sort.
 * @param currentURI The URI of the current text document.
 */
function sortByProximity(locations: sourcegraph.Location[], currentURI: URL): sourcegraph.Location[] {
    return sortBy(
        locations,
        ({ uri }) => -jaccardIndex(new Set(uri.hash.slice(1).split('/')), new Set(currentURI.hash.slice(1).split('/')))
    )
}

/**
 * Calculate the jaccard index, or the Intersection over Union of two sets.
 *
 * @param a The first set.
 * @param b The second set.
 */
function jaccardIndex<T>(a: Set<T>, b: Set<T>): number {
    const aArray = Array.from(a)
    const bArray = Array.from(b)

    return (
        // Get the size of the intersection
        new Set(aArray.filter(value => b.has(value))).size /
        // Get the size of the union
        new Set(aArray.concat(bArray)).size
    )
}

/**
 * Returns fork and archived terms that should be supplied with the query.
 *
 * @param includeFork Whether or not the include forked repositories regardless of settings.
 * @param includeArchived Whether or not the include archived repositories regardless of settings.
 */
function repositoryKindTerms(includeFork: boolean, includeArchived: boolean): string[] {
    const additionalTerms = []
    if (includeFork || getConfig('basicCodeIntel.includeForks', false)) {
        additionalTerms.push('fork:yes')
    }

    if (includeArchived || getConfig('basicCodeIntel.includeArchives', false)) {
        additionalTerms.push('archived:yes')
    }

    return additionalTerms
}

/** Returns a regular expression matching the given repository. */
function makeRepositoryPattern(repo: string): string {
    return `^${repo.replaceAll(' ', '\\ ')}$`
}
