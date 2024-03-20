import type { Remote } from 'comlink'
import * as H from 'history'
import { isEqual, uniqWith } from 'lodash'
import {
    combineLatest,
    merge,
    type Observable,
    of,
    Subscription,
    type Unsubscribable,
    concat,
    from,
    EMPTY,
    lastValueFrom,
} from 'rxjs'
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
} from 'rxjs/operators'

import { ContributableMenu, type TextDocumentPositionParameters } from '@sourcegraph/client-api'
import { type HoveredToken, LOADER_DELAY, type MaybeLoadingResult, emitLoading } from '@sourcegraph/codeintellify'
import {
    asError,
    compatNavigate,
    type ErrorLike,
    type HistoryOrNavigate,
    isErrorLike,
    isExternalLink,
    logger,
} from '@sourcegraph/common'
import type { Location } from '@sourcegraph/extension-api-types'
import type { Context } from '@sourcegraph/template-parser'

import type { ActionItemAction } from '../actions/ActionItem'
import { wrapRemoteObservable } from '../api/client/api/common'
import type { FlatExtensionHostAPI } from '../api/contract'
import type { WorkspaceRootWithMetadata } from '../api/extension/extensionHostApi'
import { syncRemoteSubscription } from '../api/util'
import { resolveRawRepoName } from '../backend/repo'
import { languageSpecs } from '../codeintel/legacy-extensions/language-specs/languages'
import { getContributedActionItems } from '../contributions/contributions'
import type { Controller, ExtensionsControllerProps } from '../extensions/controller'
import type { PlatformContext, PlatformContextProps, URLToFileContext } from '../platform/context'
import { makeRepoGitURI, parseRepoGitURI, withWorkspaceRootInputRevision } from '../util/url'

import type { HoverContext } from './HoverOverlay'

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
    }: ExtensionsControllerProps<'extHostAPI'> & PlatformContextProps<'urlToFile' | 'requestGraphQL'>,
    hoverContext: HoveredToken & HoverContext
): Observable<ActionItemAction[]> {
    if (extensionsController === null) {
        return EMPTY
    }

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
                    switchMap(extensionHostAPI => wrapRemoteObservable(extensionHostAPI.getWorkspaceRoots()))
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
        switchMap(extensionHostAPI =>
            wrapRemoteObservable(extensionHostAPI.getContributions({ extraContext: context }))
        ),
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
    ['panel.url']: string
    hoverPosition: TextDocumentPositionParameters & URLToFileContext
    hoveredOnDefinition: boolean
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
        textDocument: { uri: makeRepoGitURI(hoverContext) },
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
        map(([definitionURLOrError, hasReferenceProvider, showFindReferences]): HoverActionsContext => {
            const fileUrl =
                definitionURLOrError !== LOADING && !isErrorLike(definitionURLOrError) && definitionURLOrError?.url
                    ? definitionURLOrError.url
                    : ''

            const hoveredFileUrl = urlToFile({ ...hoverContext, position: hoverContext }, { part: hoverContext.part })

            return {
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

                'panel.url': urlToFile(
                    { ...hoverContext, position: hoverContext, viewState: 'panelID' },
                    { part: hoverContext.part }
                ),

                // Store hoverPosition for the goToDefinition action's commandArguments to refer to.
                hoverPosition: parameters,
                hoveredOnDefinition: hoveredFileUrl === fileUrl,
            }
        }),
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
export const getDefinitionURL =
    (
        { urlToFile, requestGraphQL }: Pick<PlatformContext, 'urlToFile' | 'requestGraphQL'>,
        { getWorkspaceRoots }: { getWorkspaceRoots: () => Observable<WorkspaceRootWithMetadata[]> },
        parameters: TextDocumentPositionParameters & URLToFileContext
    ) =>
    (locations: Observable<MaybeLoadingResult<Location[]>>): Observable<MaybeLoadingResult<UIDefinitionURL | null>> =>
        combineLatest([locations, getWorkspaceRoots()]).pipe(
            switchMap(
                ([{ isLoading, result: definitions }, workspaceRoots]): Observable<
                    Partial<MaybeLoadingResult<UIDefinitionURL | null>>
                > => {
                    if (definitions.length === 0) {
                        return of({ isLoading, result: null })
                    }

                    // Get unique definitions.
                    definitions = uniqWith(definitions, isEqual)

                    if (definitions.length > 1) {
                        // Open the panel to show all definitions.
                        const uri = withWorkspaceRootInputRevision(
                            workspaceRoots || [],
                            parseRepoGitURI(parameters.textDocument.uri)
                        )
                        return of({
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
                    const defer = definitions[0]

                    // Preserve the input revision (e.g., a Git branch name instead of a Git commit SHA) if the result is
                    // inside one of the current roots. This avoids navigating the user from (e.g.) a URL with a nice Git
                    // branch name to a URL with a full Git commit SHA.
                    const uri = withWorkspaceRootInputRevision(workspaceRoots || [], parseRepoGitURI(defer.uri))
                    if (defer.range) {
                        uri.position = {
                            line: defer.range.start.line + 1,
                            character: defer.range.start.character + 1,
                        }
                    }

                    // When returning a single definition, include the repo's
                    // `rawRepoName`, to allow building URLs on the code host.
                    return concat(
                        // While we resolve the raw repo name, emit isLoading with the previous result
                        // (merged in the scan() below)
                        [{ isLoading: true }],
                        resolveRawRepoName({ ...uri, requestGraphQL }).pipe(
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
    platformContext: { urlToFile, requestGraphQL, clientApplication },
    historyOrNavigate,
    getLocation,
    locationAssign,
}: {
    extensionsController: Pick<Controller, 'extHostAPI' | 'registerCommand'>
    platformContext: Pick<PlatformContext, 'urlToFile' | 'requestGraphQL' | 'clientApplication'>
} & {
    historyOrNavigate: HistoryOrNavigate
    locationAssign: typeof globalThis.location.assign
    getLocation: () => H.Location
    /** Implementation of `window.location.assign()` used to navigate to external URLs. */
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
                        disabledTitle: 'You are at the definition',
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
                            disabledWhen: 'hoveredOnDefinition',
                        },
                        {
                            action: 'goToDefinition.preloaded',
                            when: 'goToDefinition.url',
                            disabledWhen: 'hoveredOnDefinition',
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
                        const parameters: TextDocumentPositionParameters & URLToFileContext =
                            JSON.parse(parametersString)

                        const { result } = await lastValueFrom(
                            wrapRemoteObservable(extensionHostAPI.getDefinition(parameters)).pipe(
                                getDefinitionURL(
                                    { urlToFile, requestGraphQL },
                                    {
                                        getWorkspaceRoots: () =>
                                            from(extensionsController.extHostAPI).pipe(
                                                switchMap(extensionHostAPI =>
                                                    wrapRemoteObservable(extensionHostAPI.getWorkspaceRoots())
                                                )
                                            ),
                                    },
                                    parameters
                                ),
                                first(({ isLoading, result }) => !isLoading || result !== null)
                            )
                        )

                        if (!result) {
                            throw new Error('No definition found.')
                        }
                        if (result.url === H.createPath(getLocation())) {
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
                        } else if (typeof historyOrNavigate === 'function') {
                            // Use react router to handle in-app navigation
                            historyOrNavigate(result.url)
                        } else {
                            compatNavigate(historyOrNavigate, result.url)
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
                            when: 'findReferences.url && (goToDefinition.showLoading || goToDefinition.url || goToDefinition.error)',
                            disabledWhen: 'false',
                        },
                    ],
                },
            })
            subscriptions.add(syncRemoteSubscription(referencesContributionPromise))

            let implementationsContributionPromise: Promise<unknown> = Promise.resolve()
            /**
             * Register find implementations contributions only for Sourcegraph web app.
             * Other client applications (browser extension, VSCode extension) use code-intel extensions bundles with
             * "Find implementations" action defined (see https://github.com/sourcegraph/sourcegraph/pull/49025 description).
             */
            if (clientApplication === 'sourcegraph') {
                const promise = extensionHostAPI.registerContributions({
                    actions: [
                        ...languageSpecs.map(spec => ({
                            actionItem: { label: 'Find implementations' },
                            command: 'open',
                            commandArguments: [
                                "${get(context, 'implementations_" +
                                    spec.languageID +
                                    "') && get(context, 'panel.url') && sub(get(context, 'panel.url'), 'panelID', 'implementations_" +
                                    spec.languageID +
                                    "') || 'noop'}",
                            ],
                            id: 'findImplementations_' + spec.languageID,
                            title: 'Find implementations',
                        })),
                    ],
                    menus: {
                        hover: languageSpecs.map(spec => ({
                            action: 'findImplementations_' + spec.languageID,
                            when:
                                "resource.language == '" +
                                spec.languageID +
                                // eslint-disable-next-line no-template-curly-in-string
                                "' && get(context, `implementations_${resource.language}`) && (goToDefinition.showLoading || goToDefinition.url || goToDefinition.error)",
                        })),
                    },
                })
                implementationsContributionPromise = promise
                subscriptions.add(syncRemoteSubscription(promise))
                for (const spec of languageSpecs) {
                    if (spec.textDocumentImplemenationSupport) {
                        extensionHostAPI
                            .updateContext({
                                [`implementations_${spec.languageID}`]: true,
                            })
                            .then(
                                () => {},
                                () => {}
                            )
                    }
                }
            }

            return Promise.all([
                definitionContributionsPromise,
                referencesContributionPromise,
                implementationsContributionPromise,
            ])
        })
        // Don't expose remote subscriptions, only sync subscriptions bag
        .then(() => undefined)
        .catch(() => {
            logger.error('Failed to register "Go to Definition" and "Find references" actions with extension host')
        })

    // Return promise to provide a way for callers to know when contributions have been successfully registered
    return { contributionsPromise, unsubscribe: subscriptions.unsubscribe.bind(subscriptions) }
}
