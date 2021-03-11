import { HoveredToken, LOADER_DELAY, MaybeLoadingResult, emitLoading } from '@sourcegraph/codeintellify'
import { Location } from '@sourcegraph/extension-api-types'
import { Remote } from 'comlink'
import * as H from 'history'
import { isEqual, uniqWith } from 'lodash'
import { combineLatest, merge, Observable, of, Subscription, Unsubscribable, concat, from } from 'rxjs'
import {
    catchError,
    delay,
    distinctUntilChanged,
    filter,
    first,
    map,
    share,
    switchMap,
    takeUntil,
    scan,
    mapTo,
    tap,
} from 'rxjs/operators'
import { ActionItemAction } from '../actions/ActionItem'
import { wrapRemoteObservable } from '../api/client/api/common'
import { Context } from '../api/client/context/context'
import { FlatExtensionHostAPI } from '../api/contract'
import { WorkspaceRootWithMetadata } from '../api/extension/flatExtensionApi'
import { ContributableMenu, TextDocumentPositionParameters } from '../api/protocol'
import { syncRemoteSubscription } from '../api/util'
import { isPrivateRepoPublicSourcegraphComErrorLike } from '../backend/errors'
import { resolveRawRepoName } from '../backend/repo'
import { getContributedActionItems } from '../contributions/contributions'
import { Controller, ExtensionsControllerProps } from '../extensions/controller'
import { PlatformContext, PlatformContextProps, URLToFileContext } from '../platform/context'
import { asError, ErrorLike, isErrorLike } from '../util/errors'
import { makeRepoURI, parseRepoURI, withWorkspaceRootInputRevision, isExternalLink } from '../util/url'
import { HoverContext } from './HoverOverlay'

const LOADING = 'loading' as const

/**
 * This function is passed to {@link module:@sourcegraph/codeintellify.createHoverifier}, which uses it to fetch
 * the list of buttons to display on the hover tooltip. This function in turn determines that by looking at all
 * action contributions for the "hover" menu. It also defines two builtin hover actions: "Go to definition" and
 * "Find references".
 */
export function getHoverActions(
    {
        extensionsController,
        platformContext,
    }: ExtensionsControllerProps & PlatformContextProps<'urlToFile' | 'requestGraphQL'>,
    hoverContext: HoveredToken & HoverContext
): Observable<ActionItemAction[]> {
    return getHoverActionsContext(
        {
            platformContext,
            getDefinition: parameters =>
                from(extensionsController.extHostAPI).pipe(
                    switchMap(extensionHostAPI => wrapRemoteObservable(extensionHostAPI.getDefinition(parameters)))
                ),
            hasReferenceProvidersForDocument: parameters =>
                from(extensionsController.extHostAPI).pipe(
                    switchMap(extensionHostAPI =>
                        wrapRemoteObservable(extensionHostAPI.hasReferenceProvidersForDocument(parameters))
                    )
                ),
            getWorkspaceRoots: () =>
                from(extensionsController.extHostAPI).pipe(
                    switchMap(extensionHostAPI => extensionHostAPI.getWorkspaceRoots())
                ),
        },
        hoverContext
    ).pipe(switchMap(context => getHoverActionItems(context, extensionsController.extHostAPI)))
}

/**
 * Gets active hover action items for the given context
 */
export const getHoverActionItems = (
    context: Context<TextDocumentPositionParameters>,
    extensionHostAPI: Promise<Remote<FlatExtensionHostAPI>>
): Observable<ActionItemAction[]> =>
    from(extensionHostAPI).pipe(
        switchMap(extensionHostAPI => wrapRemoteObservable(extensionHostAPI.getContributions(undefined, context))),
        first(),
        map(contributions => getContributedActionItems(contributions, ContributableMenu.Hover))
    )

/**
 * The scoped context properties for the hover.
 *
 * @internal
 */
export interface HoverActionsContext extends Context<TextDocumentPositionParameters> {
    ['goToDefinition.showLoading']: boolean
    ['goToDefinition.url']: string | null
    ['goToDefinition.notFound']: boolean
    ['goToDefinition.error']: boolean
    ['findReferences.url']: string | null
    hoverPosition: TextDocumentPositionParameters & URLToFileContext
}

/**
 * Returns an observable that emits the scoped context for the hover upon subscription and whenever it changes.
 *
 * @internal
 */
export function getHoverActionsContext(
    {
        getDefinition,
        hasReferenceProvidersForDocument,
        getWorkspaceRoots,
        platformContext: { urlToFile, requestGraphQL },
    }: {
        getDefinition: (parameters: TextDocumentPositionParameters) => Observable<MaybeLoadingResult<Location[]>>
        hasReferenceProvidersForDocument: (parameters: TextDocumentPositionParameters) => Observable<boolean>
        getWorkspaceRoots: () => Observable<WorkspaceRootWithMetadata[]>
        platformContext: Pick<PlatformContext, 'urlToFile' | 'requestGraphQL'>
    },
    hoverContext: HoveredToken & HoverContext
): Observable<Context<TextDocumentPositionParameters>> {
    const parameters: TextDocumentPositionParameters & URLToFileContext = {
        textDocument: { uri: makeRepoURI(hoverContext) },
        position: { line: hoverContext.line - 1, character: hoverContext.character - 1 },
        part: hoverContext.part,
    }
    const definitionURLOrError = getDefinition(parameters).pipe(
        getDefinitionURL({ urlToFile, requestGraphQL }, { getWorkspaceRoots }, parameters),
        catchError((error): [MaybeLoadingResult<ErrorLike>] => [{ isLoading: false, result: asError(error) }]),
        share()
    )

    return combineLatest([
        // definitionURLOrError:
        definitionURLOrError.pipe(emitLoading<UIDefinitionURL | ErrorLike, null>(LOADER_DELAY, null)),

        // hasReferenceProvider:
        // Only show "Find references" if a reference provider is registered. Unlike definitions, references are
        // not preloaded and here just involve statically constructing a URL, so no need to indicate loading.
        hasReferenceProvidersForDocument(parameters),

        // showFindReferences:
        // If there is no definition, delay showing "Find references" because it is likely that the token is
        // punctuation or something else that has no meaningful references. This reduces UI jitter when it can be
        // quickly determined that there is no definition. TODO(sqs): Allow reference providers to register
        // "trigger characters" or have a "hasReferences" method to opt-out of being called for certain tokens.
        merge(
            [false],
            of(true).pipe(
                delay(LOADER_DELAY),
                takeUntil(definitionURLOrError.pipe(filter(({ result }) => result !== null)))
            ),
            definitionURLOrError.pipe(
                filter(({ result }) => result !== null),
                mapTo(true)
            )
        ),
    ]).pipe(
        map(
            ([definitionURLOrError, hasReferenceProvider, showFindReferences]): HoverActionsContext => ({
                'goToDefinition.showLoading': definitionURLOrError === LOADING,
                'goToDefinition.url':
                    (definitionURLOrError !== LOADING &&
                        !isErrorLike(definitionURLOrError) &&
                        definitionURLOrError?.url) ||
                    null,
                'goToDefinition.notFound':
                    definitionURLOrError !== LOADING &&
                    !isErrorLike(definitionURLOrError) &&
                    definitionURLOrError === null,
                'goToDefinition.error': isErrorLike(definitionURLOrError) && (definitionURLOrError as any).stack,

                'findReferences.url':
                    hasReferenceProvider && showFindReferences
                        ? urlToFile(
                              { ...hoverContext, position: hoverContext, viewState: 'references' },
                              { part: hoverContext.part }
                          )
                        : null,

                // Store hoverPosition for the goToDefinition action's commandArguments to refer to.
                hoverPosition: parameters,
            })
        ),
        distinctUntilChanged((a, b) => isEqual(a, b))
    )
}

export interface UIDefinitionURL {
    /**
     * The target browser URL to navigate to when go to definition is invoked.
     */
    url: string

    /**
     * Whether the URL refers to a definition panel that shows multiple definitions.
     */
    multiple: boolean
}

/**
 * Returns an observable that emits null if no definitions are found, {url, multiple: false} if exactly 1
 * definition is found, {url: defPanelURL, multiple: true} if multiple definitions are found, or an error.
 *
 * @internal
 */
export const getDefinitionURL = (
    { urlToFile, requestGraphQL }: Pick<PlatformContext, 'urlToFile' | 'requestGraphQL'>,
    { getWorkspaceRoots }: { getWorkspaceRoots: () => Observable<WorkspaceRootWithMetadata[]> },
    parameters: TextDocumentPositionParameters & URLToFileContext
) => (locations: Observable<MaybeLoadingResult<Location[]>>): Observable<MaybeLoadingResult<UIDefinitionURL | null>> =>
    combineLatest([locations, getWorkspaceRoots()]).pipe(
        switchMap(
            ([{ isLoading, result: definitions }, workspaceRoots]): Observable<
                Partial<MaybeLoadingResult<UIDefinitionURL | null>>
            > => {
                if (definitions.length === 0) {
                    return of<MaybeLoadingResult<UIDefinitionURL | null>>({ isLoading, result: null })
                }

                // Get unique definitions.
                definitions = uniqWith(definitions, isEqual)

                if (definitions.length > 1) {
                    // Open the panel to show all definitions.
                    const uri = withWorkspaceRootInputRevision(
                        workspaceRoots || [],
                        parseRepoURI(parameters.textDocument.uri)
                    )
                    return of<MaybeLoadingResult<UIDefinitionURL | null>>({
                        isLoading,
                        result: {
                            url: urlToFile(
                                {
                                    ...uri,
                                    revision: uri.revision || '',
                                    filePath: uri.filePath || '',
                                    position: {
                                        line: parameters.position.line + 1,
                                        character: parameters.position.character + 1,
                                    },
                                    viewState: 'def',
                                },
                                { part: parameters.part }
                            ),
                            multiple: true,
                        },
                    })
                }
                const def = definitions[0]

                // Preserve the input revision (e.g., a Git branch name instead of a Git commit SHA) if the result is
                // inside one of the current roots. This avoids navigating the user from (e.g.) a URL with a nice Git
                // branch name to a URL with a full Git commit SHA.
                const uri = withWorkspaceRootInputRevision(workspaceRoots || [], parseRepoURI(def.uri))

                if (def.range) {
                    uri.position = {
                        line: def.range.start.line + 1,
                        character: def.range.start.character + 1,
                    }
                }

                // When returning a single definition, include the repo's
                // `rawRepoName`, to allow building URLs on the code host.
                return concat(
                    // While we resolve the raw repo name, emit isLoading with the previous result
                    // (merged in the scan() below)
                    [{ isLoading: true }],
                    resolveRawRepoName({ ...uri, requestGraphQL }).pipe(
                        // When encountering an ERPRIVATEREPOPUBLICSOURCEGRAPHCOM, we can assume that
                        // we're executing in a browser extension pointed to the public sourcegraph.com,
                        // in which case repoName === rawRepoName.
                        catchError(error => {
                            if (isPrivateRepoPublicSourcegraphComErrorLike(error)) {
                                return [uri.repoName]
                            }
                            throw error
                        }),
                        map(rawRepoName => ({
                            url: urlToFile(
                                { ...uri, revision: uri.revision || '', filePath: uri.filePath || '', rawRepoName },
                                { part: parameters.part }
                            ),
                            multiple: false,
                        })),
                        map(result => ({ isLoading, result }))
                    )
                )
            }
        ),
        // Merge partial updates
        scan(
            (previous, current) => ({ ...previous, ...current }),
            ((): MaybeLoadingResult<UIDefinitionURL | null> => ({ isLoading: true, result: null }))()
        )
    )

/**
 * Registers contributions for hover-related functionality.
 */
export function registerHoverContributions({
    extensionsController,
    platformContext: { urlToFile, requestGraphQL },
    history,
    locationAssign,
}: {
    extensionsController: Pick<Controller, 'extHostAPI' | 'registerCommand'>
    platformContext: Pick<PlatformContext, 'urlToFile' | 'requestGraphQL'>
} & {
    history: H.History
    /** Implementation of `window.location.assign()` used to navigate to external URLs. */
    locationAssign: typeof location.assign
}): { contributionsPromise: Promise<void> } & Unsubscribable {
    const subscriptions = new Subscription()

    const contributionsPromise = extensionsController.extHostAPI
        .then(extensionHostAPI => {
            // Registers the "Go to definition" action shown in the hover tooltip. When clicked, the action finds the
            // definition of the token using the registered definition providers and navigates the user there.
            //
            // When the user hovers over a token (even before they click "Go to definition"), it attempts to preload the
            // definition. If preloading succeeds and at least 1 definition is found, the "Go to definition" action becomes
            // a normal link (<a href>) pointing to the definition's URL. Using a normal link here is good for a11y and UX
            // (e.g., open-in-new-tab works and the browser status bar shows the URL).
            //
            // Otherwise (if preloading fails, or if preloading has not yet finished), clicking "Go to definition" executes
            // the goToDefinition command. A loading indicator is displayed, and any errors that occur during execution are
            // shown to the user.
            //
            // Future improvements:
            //
            // TODO(sqs): If the user middle-clicked or Cmd/Ctrl-clicked the button, it would be nice if when the
            // definition was found, a new browser tab was opened to the destination. This is not easy because browsers
            // usually block new tabs opened by JavaScript not directly triggered by a user mouse/keyboard interaction.
            //
            // TODO(sqs): Pin hover after an action has been clicked and before it has completed.
            const definitionContributions = {
                actions: [
                    {
                        id: 'goToDefinition',
                        title: 'Go to definition',
                        command: 'goToDefinition',
                        commandArguments: [
                            /* eslint-disable no-template-curly-in-string */
                            '${json(hoverPosition)}',
                            /* eslint-enable no-template-curly-in-string */
                        ],
                    },
                    {
                        // This action is used when preloading the definition succeeded and at least 1
                        // definition was found.
                        id: 'goToDefinition.preloaded',
                        title: 'Go to definition',
                        command: 'open',
                        // eslint-disable-next-line no-template-curly-in-string
                        commandArguments: ['${goToDefinition.url}'],
                    },
                ],
                menus: {
                    hover: [
                        // Do not show any actions if no definition provider is registered. (In that case,
                        // goToDefinition.{error, loading, url} will all be falsey.)
                        {
                            action: 'goToDefinition',
                            when: 'goToDefinition.error || goToDefinition.showLoading',
                        },
                        {
                            action: 'goToDefinition.preloaded',
                            when: 'goToDefinition.url',
                        },
                    ],
                },
            }

            const definitionContributionsPromise = extensionHostAPI.registerContributions(definitionContributions)
            subscriptions.add(syncRemoteSubscription(definitionContributionsPromise))

            subscriptions.add(
                extensionsController.registerCommand({
                    command: 'goToDefinition',
                    run: async (parametersString: string) => {
                        const parameters: TextDocumentPositionParameters & URLToFileContext = JSON.parse(
                            parametersString
                        )

                        const { result } = await wrapRemoteObservable(extensionHostAPI.getDefinition(parameters))
                            .pipe(
                                getDefinitionURL(
                                    { urlToFile, requestGraphQL },
                                    {
                                        getWorkspaceRoots: () =>
                                            from(extensionsController.extHostAPI).pipe(
                                                switchMap(extensionHostAPI => extensionHostAPI.getWorkspaceRoots())
                                            ),
                                    },
                                    parameters
                                ),
                                first(({ isLoading, result }) => !isLoading || result !== null)
                            )
                            .toPromise()

                        if (!result) {
                            throw new Error('No definition found.')
                        }
                        if (result.url === H.createPath(history.location)) {
                            // The user might be confused if they click "Go to definition" and don't go anywhere, which
                            // occurs if they are *already* on the definition. Give a helpful tip if they do this.
                            //
                            // Note that these tips won't show up if the definition URL is already known by the time they
                            // click "Go to definition", because then it's a normal link and not a button that executes
                            // this command. TODO: It would be nice if they also showed up in that case.
                            if (result.multiple) {
                                // The user may not have noticed the panel at the bottom of the screen, so tell them
                                // explicitly.
                                throw new Error('Multiple definitions shown in panel below.')
                            }
                            throw new Error('Already at the definition.')
                        }
                        if (isExternalLink(result.url)) {
                            // External links must be navigated to through the browser
                            locationAssign(result.url)
                        } else {
                            // Use history library to handle in-app navigation
                            history.push(result.url)
                        }
                    },
                })
            )

            // Register the "Find references" action shown in the hover tooltip. This is simpler than "Go to definition"
            // because it just needs a URL that can be statically constructed from the current URL (it does not need to
            // query any providers).
            const referencesContributionPromise = extensionHostAPI.registerContributions({
                actions: [
                    {
                        id: 'findReferences',
                        // title: parseTemplate('Find references'),
                        title: 'Find references',
                        command: 'open',
                        // eslint-disable-next-line no-template-curly-in-string
                        commandArguments: ['${findReferences.url}'],
                    },
                ],
                menus: {
                    hover: [
                        // To reduce UI jitter, even though "Find references" can be shown immediately (because
                        // the URL can be statically constructed), don't show it until either (1) "Go to
                        // definition" is showing or (2) the LOADER_DELAY has elapsed. The part (2) of this
                        // logic is implemented in the observable pipe that sets findReferences.url above.
                        {
                            action: 'findReferences',
                            when:
                                'findReferences.url && (goToDefinition.showLoading || goToDefinition.url || goToDefinition.error)',
                        },
                    ],
                },
            })
            subscriptions.add(syncRemoteSubscription(referencesContributionPromise))

            return Promise.all([definitionContributionsPromise, referencesContributionPromise])
        })
        // Don't expose remote subscriptions, only sync subscriptions bag
        .then(() => undefined)
        .catch(() => {
            console.error('Failed to register "Go to Definition" and "Find references" actions with extension host')
        })

    // Return promise to provide a way for callers to know when contributions have been successfully registered
    return { contributionsPromise, unsubscribe: subscriptions.unsubscribe.bind(subscriptions) }
}
