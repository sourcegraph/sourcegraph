import * as React from 'react'

import classNames from 'classnames'
import * as H from 'history'
import { isEqual } from 'lodash'
import type { Renderer } from 'react-dom'
import { createRoot } from 'react-dom/client'
import {
    asyncScheduler,
    combineLatest,
    EMPTY,
    from,
    type Observable,
    of,
    Subject,
    Subscription,
    type Unsubscribable,
    concat,
    BehaviorSubject,
    fromEvent,
    lastValueFrom,
    throwError,
} from 'rxjs'
import {
    catchError,
    concatAll,
    concatMap,
    filter,
    map,
    mergeMap,
    observeOn,
    switchMap,
    withLatestFrom,
    tap,
    startWith,
    distinctUntilChanged,
    retry,
    take,
} from 'rxjs/operators'

import type { HoverMerged } from '@sourcegraph/client-api'
import {
    type ContextResolver,
    createHoverifier,
    findPositionsFromEvents,
    type Hoverifier,
    type HoverState,
    type MaybeLoadingResult,
} from '@sourcegraph/codeintellify'
import {
    asError,
    asObservable,
    isDefined,
    isInstanceOf,
    property,
    registerHighlightContributions,
    isExternalLink,
    type LineOrPositionOrRange,
} from '@sourcegraph/common'
import type { WorkspaceRoot } from '@sourcegraph/extension-api-types'
import { gql, isHTTPAuthError } from '@sourcegraph/http-client'
import type { ActionItemAction } from '@sourcegraph/shared/src/actions/ActionItem'
import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import type { CodeEditorData, CodeEditorWithPartialModel } from '@sourcegraph/shared/src/api/viewerTypes'
import { isRepoNotFoundErrorLike } from '@sourcegraph/shared/src/backend/errors'
import type { Controller } from '@sourcegraph/shared/src/extensions/controller'
import { getHoverActions, registerHoverContributions } from '@sourcegraph/shared/src/hover/actions'
import {
    type HoverContext,
    HoverOverlay,
    type HoverOverlayClassProps,
} from '@sourcegraph/shared/src/hover/HoverOverlay'
import { getModeFromPath } from '@sourcegraph/shared/src/languages'
import type { PlatformContext, URLToFileContext } from '@sourcegraph/shared/src/platform/context'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { createURLWithUTM } from '@sourcegraph/shared/src/tracking/utm'
import {
    type FileSpec,
    type UIPositionSpec,
    type RawRepoSpec,
    type RepoSpec,
    type ResolvedRevisionSpec,
    type RevisionSpec,
    type ViewStateSpec,
    makeRepoGitURI,
} from '@sourcegraph/shared/src/util/url'

import { background } from '../../../browser-extension/web-extension-api/runtime'
import { observeStorageKey } from '../../../browser-extension/web-extension-api/storage'
import type { BackgroundPageApi } from '../../../browser-extension/web-extension-api/types'
import type { UserSettingsURLResult } from '../../../graphql-operations'
import { toTextDocumentPositionParameters } from '../../backend/extension-api-conversion'
import { CodeViewToolbar, type CodeViewToolbarClassProps } from '../../components/CodeViewToolbar'
import { TrackAnchorClick } from '../../components/TrackAnchorClick'
import { WildcardThemeProvider } from '../../components/WildcardThemeProvider'
import { isExtension, isInPage } from '../../context'
import type { SourcegraphIntegrationURLs, BrowserPlatformContext } from '../../platform/context'
import { resolveRevision, retryWhenCloneInProgressError, resolvePrivateRepo } from '../../repo/backend'
import { ConditionalTelemetryRecorderProvider } from '../../telemetry'
import { ConditionalTelemetryService, EventLogger } from '../../tracking/eventLogger'
import { DEFAULT_SOURCEGRAPH_URL, getPlatformName, isDefaultSourcegraphUrl } from '../../util/context'
import { type MutationRecordLike, querySelectorOrSelf } from '../../util/dom'
import { observeSendTelemetry } from '../../util/optionFlags'
import { bitbucketCloudCodeHost } from '../bitbucket-cloud/codeHost'
import { bitbucketServerCodeHost } from '../bitbucket/codeHost'
import { gerritCodeHost } from '../gerrit/codeHost'
import { type GithubCodeHost, githubCodeHost, isGithubCodeHost } from '../github/codeHost'
import { gitlabCodeHost } from '../gitlab/codeHost'
import { phabricatorCodeHost } from '../phabricator/codeHost'

import { type CodeView, trackCodeViews, fetchFileContentForDiffOrFileInfo } from './codeViews'
import { NotAuthenticatedError, RepoURLParseError } from './errors'
import { initializeExtensions } from './extensions'
import { SignInButton } from './SignInButton'
import { resolveRepoNamesForDiffOrFileInfo, defaultRevisionToCommitID } from './util/fileInfo'
import { lprToSelectionsZeroIndexed } from './util/selections'
import {
    type ViewOnSourcegraphButtonClassProps,
    ViewOnSourcegraphButton,
    ConfigureSourcegraphButton,
} from './ViewOnSourcegraphButton'
import { delayUntilIntersecting, trackViews, type ViewResolver } from './views'

import styles from './codeHost.module.scss'

registerHighlightContributions()

export type OverlayPosition = { left: number } & ({ top: number } | { bottom: number })

export type ObserveMutations = (
    target: Node,
    options?: MutationObserverInit,
    paused?: Subject<boolean>
) => Observable<MutationRecordLike[]>

/**
 * A function that gets the mount location for elements being mounted to the DOM.
 *
 * - If the mount doesn't belong into the container, it must return `null`.
 * - If the mount already exists in the container, it must return the existing mount.
 * - If the mount does not exist yet in the container, it must create and return it.
 *
 * Caveats:
 * - The passed element might be the mount itself
 * - The passed element might be an element _within_ the mount
 */
export type MountGetter = (container: HTMLElement) => HTMLElement | null

/**
 * The context the code host is in on the current page.
 */
export type CodeHostContext = RawRepoSpec & Partial<RevisionSpec> & { privateRepository: boolean }

export type CodeHostType = 'github' | 'phabricator' | 'bitbucket-server' | 'bitbucket-cloud' | 'gitlab' | 'gerrit'

/** Information for adding code navigation to code views on arbitrary code hosts. */
export interface CodeHost {
    /**
     * The type of the code host. This will be added as a className to the overlay mount.
     * Use {@link CodeHost#name} if you need a human-readable name for the code host to display in the UI.
     */
    type: CodeHostType

    /**
     * A human-readable name for the code host, to be displayed in the UI.
     */
    name: string

    /**
     * Basic contextual information for the current code host.
     */
    getContext?: () => Promise<CodeHostContext>

    /**
     * An Observable for whether the code host is in light theme (vs dark theme).
     * Defaults to always light theme.
     */
    isLightTheme?: Observable<boolean>

    /**
     * An Observable to indicate when client-side route has been changed.
     */
    routeChange?: (mutations: Observable<MutationRecordLike[]>) => Observable<unknown>

    /**
     * Mount getter for the repository "View on Sourcegraph" button.
     *
     * If undefined, the "View on Sourcegraph" button won't be rendered on the code host.
     */
    getViewContextOnSourcegraphMount?: MountGetter

    /**
     * Optional class name for the contextual link to Sourcegraph.
     */
    viewOnSourcegraphButtonClassProps?: ViewOnSourcegraphButtonClassProps

    /**
     * Checks to see if the current context the code is running in is within
     * the given code host.
     */
    check: () => boolean

    /**
     * CSS classes for ActionItem buttons in the hover overlay to customize styling
     */
    hoverOverlayClassProps?: HoverOverlayClassProps

    /**
     * Resolve {@link CodeView}s from the DOM.
     */
    codeViewResolvers: ViewResolver<CodeView>[]

    /**
     * Override of `observeMutations`, used where a MutationObserve is not viable, such as in the shadow DOMs in Gerrit.
     */
    observeMutations?: ObserveMutations

    // Extensions related input

    /**
     * Returns a selector used to determine the mount location of the hover overlay in the DOM.
     *
     * If undefined, or when null is returned, the hover overlay container will be mounted to <body>.
     */
    getHoverOverlayMountLocation?: () => string | null

    /**
     * Construct the URL to the specified file.
     *
     * @param sourcegraphURL The URL of the Sourcegraph instance.
     * @param target The target to build a URL for.
     * @param context Context information about this invocation.
     */
    urlToFile?: (
        sourcegraphURL: string,
        target: RepoSpec & RawRepoSpec & RevisionSpec & FileSpec & Partial<UIPositionSpec> & Partial<ViewStateSpec>,
        context: URLToFileContext
    ) => string

    observeLineSelection?: Observable<LineOrPositionOrRange>

    /**
     * CSS classes for the code view toolbar to customize styling
     */
    codeViewToolbarClassProps?: CodeViewToolbarClassProps

    /**
     * Whether or not code views need to be tokenized. Defaults to false.
     */
    codeViewsRequireTokenization?: boolean

    /**
     * Called before injecting the code intelligence to the code host.
     */
    prepareCodeHost?: (requestGraphQL: BrowserPlatformContext['requestGraphQL']) => Promise<boolean>
}

/**
 * A blob (single file `FileInfo`) or a diff (with a head `FileInfo` and/or base `FileInfo`)
 */
export type DiffOrBlobInfo<T extends FileInfo = FileInfo> = BlobInfo<T> | DiffInfo<T>
export interface BlobInfo<T extends FileInfo = FileInfo> {
    blob: T
}
export type DiffInfo<T extends FileInfo = FileInfo> =
    // `base?: undefined` avoids making `{ head: T; base: T }` assignable to this type
    { head: T; base?: undefined } | { base: T; head?: undefined } | { head: T; base: T }

export interface FileInfo {
    /**
     * The path for the repo the file belongs to.
     */
    rawRepoName: string
    /**
     * The path for the file path for a given `codeView`.
     */
    filePath: string
    /**
     * The commit that the code view is at.
     */
    commitID: string
    /**
     * The revision the code view is at.
     */
    revision?: string
}

export interface FileInfoWithRepoName extends FileInfo, RepoSpec {}
export interface FileInfoWithContent extends FileInfoWithRepoName {
    content?: string
}

export interface CodeIntelligenceProps extends TelemetryProps, TelemetryV2Props {
    platformContext: Pick<
        BrowserPlatformContext,
        'urlToFile' | 'requestGraphQL' | 'settings' | 'refreshSettings' | 'sourcegraphURL' | 'clientApplication'
    >
    codeHost: CodeHost
    extensionsController: Controller
}

export const getExistingOrCreateOverlayMount = (codeHostName: string, container: HTMLElement): HTMLElement => {
    let mount = container.querySelector<HTMLDivElement>(`.hover-overlay-mount.hover-overlay-mount__${codeHostName}`)

    if (!mount) {
        mount = document.createElement('div')
        mount.classList.add('hover-overlay-mount', `hover-overlay-mount__${codeHostName}`)
        container.append(mount)
    }

    return mount
}

/**
 * Prepares the page for code navigation. It creates the hoverifier, injects
 * and mounts the hover overlay and then returns the hoverifier.
 */
function initCodeIntelligence({
    mutations,
    codeHost,
    platformContext,
    extensionsController,
    render,
    telemetryService,
    telemetryRecorder,
    repoSyncErrors,
}: Pick<
    CodeIntelligenceProps,
    'codeHost' | 'platformContext' | 'extensionsController' | 'telemetryService' | 'telemetryRecorder'
> & {
    render: Renderer
    mutations: Observable<MutationRecordLike[]>
    repoSyncErrors: Observable<boolean>
}): {
    hoverifier: Hoverifier<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec, HoverMerged, ActionItemAction>
    subscription: Unsubscribable
} {
    const subscription = new Subscription()

    /** Emits whenever the ref callback for the hover element is called */
    const hoverOverlayElements = new Subject<HTMLElement | null>()

    const containerComponentUpdates = new Subject<void>()

    const history = H.createBrowserHistory()

    subscription.add(
        registerHoverContributions({
            extensionsController,
            platformContext,
            historyOrNavigate: history,
            getLocation: () => history.location,
            locationAssign: location.assign.bind(location),
        })
    )

    // Code views come and go, but there is always a single hoverifier on the page
    const hoverifier = createHoverifier<
        RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec,
        HoverMerged,
        ActionItemAction
    >({
        hoverOverlayElements,
        hoverOverlayRerenders: containerComponentUpdates.pipe(
            withLatestFrom(hoverOverlayElements),
            map(([, hoverOverlayElement]) => ({ hoverOverlayElement })),
            filter(property('hoverOverlayElement', isDefined))
        ),
        getHover: ({ line, character, part, ...rest }) =>
            concat(
                [{ isLoading: true, result: null }],
                from(extensionsController.extHostAPI)
                    .pipe(
                        withLatestFrom(repoSyncErrors),
                        switchMap(([extensionHost, hasRepoSyncError]) =>
                            // Prevent GraphQL requests that we know will result in error/null when the repo is private (and not added to Cloud)
                            hasRepoSyncError
                                ? of({ isLoading: true, result: null })
                                : wrapRemoteObservable(
                                      extensionHost.getHover(
                                          toTextDocumentPositionParameters({ ...rest, position: { line, character } })
                                      )
                                  )
                        )
                    )
                    .pipe(
                        map(
                            ({ isLoading, result: hoverMerged }): MaybeLoadingResult<HoverMerged | null> => ({
                                isLoading,
                                result: hoverMerged || null,
                            })
                        )
                    )
            ),
        getDocumentHighlights: ({ line, character, part, ...rest }) =>
            from(extensionsController.extHostAPI).pipe(
                withLatestFrom(repoSyncErrors),
                switchMap(([extensionHost, hasRepoSyncError]) =>
                    // Prevent GraphQL requests that we know will result in error/null when the repo is private (and not added to Cloud)
                    hasRepoSyncError
                        ? of([])
                        : wrapRemoteObservable(
                              extensionHost.getDocumentHighlights(
                                  toTextDocumentPositionParameters({ ...rest, position: { line, character } })
                              )
                          )
                )
            ),
        getActions: context =>
            // Prevent GraphQL requests that we know will result in error/null when the repo is private (and not added to Cloud)
            repoSyncErrors.pipe(
                take(1),
                switchMap(hasRepoSyncError =>
                    hasRepoSyncError ? of([]) : getHoverActions({ extensionsController, platformContext }, context)
                )
            ),
        tokenize: codeHost.codeViewsRequireTokenization,
    })

    class HoverOverlayContainer extends React.Component<{}, HoverState<HoverContext, HoverMerged, ActionItemAction>> {
        private subscription = new Subscription()
        private nextOverlayElement = hoverOverlayElements.next.bind(hoverOverlayElements)

        constructor(props: {}) {
            super(props)
            this.state = {
                ...hoverifier.hoverState,
            }
        }
        public componentDidMount(): void {
            this.subscription.add(
                hoverifier.hoverStateUpdates.subscribe(update => {
                    this.setState(update)
                })
            )
            containerComponentUpdates.next()
        }
        public componentWillUnmount(): void {
            this.subscription.unsubscribe()
        }
        public componentDidUpdate(): void {
            containerComponentUpdates.next()
        }
        /**
         * Intercept all link clicks and append UTM markers to any external URLs
         */
        private handleHoverLinkClick = (event: React.MouseEvent): void => {
            const element = event.target as HTMLAnchorElement
            if (!isExternalLink(element.href)) {
                return
            }

            const newHref = createURLWithUTM(new URL(element.href), {
                utm_source: getPlatformName(),
                utm_campaign: 'hover',
            }).href

            if (element.getAttribute('target') === '_blank') {
                window.open(newHref, '_blank', element.getAttribute('rel') ?? undefined)
            } else {
                window.location.href = newHref
            }

            event.preventDefault()
        }
        public render(): JSX.Element | null {
            if (!this.state.hoverOverlayProps) {
                return null
            }

            return (
                <TrackAnchorClick onClick={this.handleHoverLinkClick}>
                    <HoverOverlay
                        {...this.state.hoverOverlayProps}
                        {...codeHost.hoverOverlayClassProps}
                        className={classNames(styles.hoverOverlay, codeHost.hoverOverlayClassProps?.className)}
                        telemetryService={telemetryService}
                        telemetryRecorder={telemetryRecorder}
                        hoverRef={this.nextOverlayElement}
                        extensionsController={extensionsController}
                        location={H.createLocation(window.location)}
                        useBrandedLogo={true}
                    />
                </TrackAnchorClick>
            )
        }
    }

    const { getHoverOverlayMountLocation } = codeHost
    if (!getHoverOverlayMountLocation) {
        // This renders to document.body, which we can assume is never removed,
        // so we don't need to subscribe to mutations.
        const overlayMount = getExistingOrCreateOverlayMount(codeHost.type, document.body)
        render(<HoverOverlayContainer />, overlayMount)
    } else {
        let previousMount: HTMLElement | null = null
        subscription.add(
            observeHoverOverlayMountLocation(getHoverOverlayMountLocation, mutations).subscribe(mountLocation => {
                // Remove the previous mount if it exists,
                // to avoid displaying duplicate hovers.
                if (previousMount) {
                    previousMount.remove()
                }
                const mount = getExistingOrCreateOverlayMount(codeHost.type, mountLocation)
                previousMount = mount
                render(<HoverOverlayContainer />, mount)
            })
        )
    }

    return { hoverifier, subscription }
}

/**
 * Returns an Observable that emits the element where
 * the hover overlay mount should be appended, taking account
 * mutations and {@link CodeHost#getHoverOverlayMountLocation}.
 *
 * The caller is responsible for removing the previous mount if it exists.
 *
 * This is useful to mount the hover overlay to a different container than document.body,
 * so that it is affected by the visibility changes of that container.
 *
 * Related issue: https://gitlab.com/gitlab-org/gitlab/issues/193433
 *
 * Example use case on GitLab:
 * 1. User visits https://gitlab.com/gitlab-org/gitaly/-/merge_requests/1575. `div.tab-pane.diffs` doesn't exist yet (it'll be lazy-loaded) -> Mount the  hover overlay is to `document.body`.
 * 2. User visits the 'Changes' tab -> Unmount from `document.body`, mount to `div.tab-pane.diffs`.
 * 3. User visits the 'Overview' tab again -> `div.tab-pane.diffs` is hidden, and as a result so is the hover overlay.
 * 4. User navigates away from the merge request (soft-reload), `div.tab-pane.diffs` is removed -> Mount to `document.body` again.
 */
export function observeHoverOverlayMountLocation(
    getMountLocationSelector: NonNullable<CodeHost['getHoverOverlayMountLocation']>,
    mutations: Observable<MutationRecordLike[]>
): Observable<HTMLElement> {
    return mutations.pipe(
        concatAll(),
        map(({ addedNodes, removedNodes }): HTMLElement | null => {
            // If no selector can be used to determine the mount location
            // return document.body as the mount location.
            const selector = getMountLocationSelector()
            if (selector === null) {
                return document.body
            }
            // If any of the added nodes match the selector, return it
            // as the new mount location.
            for (const addedNode of addedNodes) {
                if (!(addedNode instanceof HTMLElement)) {
                    continue
                }
                const mountLocation = querySelectorOrSelf<HTMLElement>(addedNode, selector)
                if (mountLocation) {
                    return mountLocation
                }
            }
            // If any of the removed nodes match the selector,
            // return document.body as the new mount location.
            for (const removedNode of removedNodes) {
                if (!(removedNode instanceof HTMLElement)) {
                    continue
                }
                if (querySelectorOrSelf<HTMLElement>(removedNode, selector)) {
                    return document.body
                }
            }
            // Neither added nodes nor removed nodes match the selector,
            // don't return a new mount location.
            return null
        }),
        filter(isDefined),
        startWith(document.body),
        distinctUntilChanged()
    )
}

export interface HandleCodeHostOptions extends CodeIntelligenceProps {
    mutations: Observable<MutationRecordLike[]>
    render: Renderer
    minimalUI: boolean
    hideActions?: boolean
    background: Pick<BackgroundPageApi, 'notifyRepoSyncError' | 'openOptionsPage'>
}

/**
 * Opens extension options page
 */
const onConfigureSourcegraphClick: React.MouseEventHandler<HTMLAnchorElement> = async event => {
    event.preventDefault()
    if (isExtension) {
        await background.openOptionsPage()
    }
}

const buildManageRepositoriesURL = (sourcegraphURL: string, settingsURL: string, repoName: string): string => {
    const url = new URL(`${settingsURL}/repositories/manage`, sourcegraphURL)
    url.searchParams.set('filter', repoName)
    return url.href
}

const observeUserSettingsURL = (requestGraphQL: PlatformContext['requestGraphQL']): Observable<string> =>
    requestGraphQL<UserSettingsURLResult>({
        request: gql`
            query UserSettingsURL {
                currentUser {
                    settingsURL
                }
            }
        `,
        variables: {},
        mightContainPrivateInfo: true,
    }).pipe(
        map(({ data }) => data?.currentUser?.settingsURL),
        filter(isDefined)
    )

/**
 * @returns boolean indicating whether it is safe to continue initialization
 *
 * Returns
 * - "false": if could not parse repository name or if detected repository is private is not cloned in Sourcegraph Cloud
 * - "true" in all other cases
 *
 * Side-effect:
 * - Notifies background about private cloud error
 * - Renders "Configure Sourcegraph" or "Sign In" button
 */
const isSafeToContinueCodeIntel = async ({
    sourcegraphURL,
    codeHost,
    requestGraphQL,
    render,
}: Pick<HandleCodeHostOptions, 'render' | 'codeHost'> &
    Pick<HandleCodeHostOptions['platformContext'], 'requestGraphQL' | 'sourcegraphURL'>): Promise<boolean> => {
    if (!isDefaultSourcegraphUrl(sourcegraphURL) || !codeHost.getContext) {
        return true
    }

    let rawRepoName: string | undefined

    // This is only when connected to Sourcegraph Cloud and code host either GitLab or GitHub
    try {
        const context = await codeHost.getContext()

        if (!context.privateRepository) {
            // We can auto-clone public repos
            return true
        }

        rawRepoName = context.rawRepoName

        const isRepoCloned = await lastValueFrom(
            resolvePrivateRepo({
                rawRepoName,
                requestGraphQL,
            })
        )

        return isRepoCloned
    } catch (error) {
        // Ignore non-repository pages
        if (error instanceof RepoURLParseError) {
            return false
        }

        if (isExtension) {
            // Notify to show extension alert-icon
            background.notifyRepoSyncError({ sourcegraphURL, hasRepoSyncError: true }).catch(error => {
                console.error('Error notifying background page of private cloud.', error)
            })
        }

        if (!codeHost.getViewContextOnSourcegraphMount) {
            console.warn('Repository is not cloned or you are not authenticated to Sourcegraph.', error)
            return false
        }

        if (error instanceof NotAuthenticatedError) {
            // Show "Sign In" button
            console.warn('Not authenticated to Sourcegraph.', error)
            render(
                <SignInButton {...{ ...codeHost.viewOnSourcegraphButtonClassProps }} sourcegraphURL={sourcegraphURL} />,
                codeHost.getViewContextOnSourcegraphMount(document.body)
            )
        } else {
            // Show "Configure Sourcegraph" button
            console.warn('Repository is not cloned.', error)

            const settingsURL = await lastValueFrom(observeUserSettingsURL(requestGraphQL), { defaultValue: undefined })

            if (rawRepoName && settingsURL) {
                render(
                    <ConfigureSourcegraphButton
                        {...codeHost.viewOnSourcegraphButtonClassProps}
                        className={classNames(
                            'open-on-sourcegraph',
                            codeHost.viewOnSourcegraphButtonClassProps?.className
                        )}
                        codeHostType={codeHost.type}
                        href={buildManageRepositoriesURL(
                            sourcegraphURL,
                            settingsURL,
                            rawRepoName.split('/').slice(1).join('/')
                        )}
                    />,
                    codeHost.getViewContextOnSourcegraphMount(document.body)
                )
            }
        }

        return false
    }
}

export async function handleCodeHost({
    mutations,
    codeHost,
    extensionsController,
    platformContext,
    telemetryService,
    telemetryRecorder,
    render,
    minimalUI,
    hideActions,
    background,
}: HandleCodeHostOptions): Promise<Subscription> {
    const subscriptions = new Subscription()
    const { requestGraphQL, sourcegraphURL } = platformContext

    const addedElements = mutations.pipe(
        concatAll(),
        concatMap(mutation => mutation.addedNodes),
        filter(isInstanceOf(HTMLElement))
    )

    // Handle theming
    subscriptions.add(
        (codeHost.isLightTheme ?? of(true)).subscribe(isLightTheme => {
            document.documentElement.classList.toggle('theme-light', isLightTheme)
            document.documentElement.classList.toggle('theme-dark', !isLightTheme)
        })
    )

    /**
     * A stream that emits a boolean that signifies whether any request for
     * the current repository has failed because one of the following reasons.
     * 1. It is a private repository not synced with Sourcegraph Cloud and the latter is the
     * active Sourcegraph URL.
     * 2. It is a repository not added to the Sourcegraph instance (other than Cloud).
     * If the current state is `true`, we can short circuit subsequent requests.
     * */
    const repoSyncErrors = new BehaviorSubject<boolean>(false)
    // Set by `ViewOnSourcegraphButton` (cleans up and sets to `false` whenever it is unmounted).
    const setRepoSyncError = repoSyncErrors.next.bind(repoSyncErrors)

    /**
     * Checks whether the error was caused by one of the following conditions:
     * - repository is private and not synced with Sourcegraph Cloud
     * - repository is not added to other than Cloud Sourcegraph instance.
     *
     * (no side effects, doesn't notify `repoSyncErrors`)
     * */
    const checkRepoSyncError = async (error: any): Promise<boolean> =>
        isRepoNotFoundErrorLike(error) &&
        (isDefaultSourcegraphUrl(sourcegraphURL) ? !!(await codeHost.getContext?.())?.privateRepository : true)

    if (isGithubCodeHost(codeHost)) {
        // TODO: add tests in codeHost.test.tsx
        const { searchEnhancement } = codeHost
        if (searchEnhancement) {
            subscriptions.add(initializeGithubSearchInputEnhancement(searchEnhancement, sourcegraphURL, mutations))
        }
        // TODO(#44327): Uncomment or remove this depending on the outcome of the issue.
        // if (codeHost.enhanceSearchPage) {
        //     subscriptions.add(enhanceSearchPage(sourcegraphURL))
        // }
    }

    if (!(await isSafeToContinueCodeIntel({ sourcegraphURL, requestGraphQL, codeHost, render }))) {
        // Stop initializing code navigation
        return subscriptions
    }

    const { hoverifier, subscription } = initCodeIntelligence({
        codeHost,
        extensionsController,
        platformContext,
        telemetryService,
        telemetryRecorder,
        render,
        mutations,
        repoSyncErrors,
    })
    subscriptions.add(hoverifier)
    subscriptions.add(subscription)

    const signInCloses = new Subject<void>()
    const nextSignInClose = signInCloses.next.bind(signInCloses)

    // Try to fetch settings and refresh them when a sign in tab was closed
    subscriptions.add(
        concat([null], signInCloses)
            .pipe(
                switchMap(() =>
                    from(platformContext.refreshSettings()).pipe(
                        catchError(error => {
                            console.error('Refreshing settings failed', error)
                            return []
                        })
                    )
                )
            )
            .subscribe()
    )

    /** The number of code views that were detected on the page (not necessarily initialized) */
    const codeViewCount = new BehaviorSubject<number>(0)

    // Render view on Sourcegraph button
    if (codeHost.getContext) {
        const { getContext, viewOnSourcegraphButtonClassProps } = codeHost

        /** Whether or not the repo exists on the configured Sourcegraph instance. */
        const repoExistsOrErrors = combineLatest([signInCloses.pipe(startWith(null)), from(getContext())]).pipe(
            switchMap(([, { rawRepoName, revision }]) =>
                resolveRevision({ repoName: rawRepoName, revision, requestGraphQL }).pipe(
                    retryWhenCloneInProgressError(),
                    map(() => true),
                    startWith(undefined)
                )
            ),
            catchError(error => {
                if (isRepoNotFoundErrorLike(error) || error instanceof RepoURLParseError) {
                    return [false]
                }
                return [asError(error)]
            }),
            tap(repoExistsOrErrors => {
                if (typeof repoExistsOrErrors === 'boolean') {
                    const hasRepoSyncError = repoExistsOrErrors === false
                    setRepoSyncError(hasRepoSyncError)
                    if (isExtension) {
                        background.notifyRepoSyncError({ sourcegraphURL, hasRepoSyncError }).catch(error => {
                            console.error('Error notifying background page of private cloud error:', error)
                        })
                    }
                }
            })
        )

        if (codeHost.getViewContextOnSourcegraphMount) {
            subscriptions.add(
                combineLatest([
                    repoExistsOrErrors,
                    addedElements.pipe(map(codeHost.getViewContextOnSourcegraphMount), filter(isDefined)),
                    // Only show sign in button when there is no other code view on the page that is displaying it
                    codeViewCount.pipe(
                        map(count => count === 0),
                        distinctUntilChanged()
                    ),
                    from(getContext()),
                    observeUserSettingsURL(requestGraphQL).pipe(startWith(undefined)),
                ]).subscribe(([repoExistsOrError, mount, showSignInButton, context, userSettingsURL]) => {
                    render(
                        <ViewOnSourcegraphButton
                            {...viewOnSourcegraphButtonClassProps}
                            codeHostType={codeHost.type}
                            context={context}
                            minimalUI={minimalUI}
                            sourcegraphURL={sourcegraphURL}
                            userSettingsURL={
                                userSettingsURL &&
                                buildManageRepositoriesURL(sourcegraphURL, userSettingsURL, context.rawRepoName)
                            }
                            repoExistsOrError={repoExistsOrError}
                            showSignInButton={showSignInButton}
                            // The bound function is constant
                            onSignInClose={nextSignInClose}
                            onConfigureSourcegraphClick={isInPage ? undefined : onConfigureSourcegraphClick}
                        />,
                        mount
                    )
                })
            )
        } else {
            subscriptions.add(repoExistsOrErrors.subscribe())
        }
    }

    /** A stream of added or removed code views with the resolved file info */
    const codeViews = mutations.pipe(
        trackCodeViews(codeHost),
        tap(codeViewEvent => {
            codeViewCount.next(codeViewCount.value + 1)
            codeViewEvent.subscriptions.add(() => codeViewCount.next(codeViewCount.value - 1))
        }),
        // Delay emitting code views until they are in the viewport, or within 4000 vertical
        // pixels of the viewport's top or bottom edges.
        delayUntilIntersecting({ rootMargin: '4000px 0px' }),
        mergeMap(codeViewEvent =>
            asObservable(() =>
                codeViewEvent.resolveFileInfo(codeViewEvent.element, platformContext.requestGraphQL)
            ).pipe(
                mergeMap(diffOrBlobInfo =>
                    resolveRepoNamesForDiffOrFileInfo(
                        diffOrBlobInfo,
                        checkRepoSyncError,
                        platformContext.requestGraphQL
                    )
                ),
                mergeMap(diffOrBlobInfo =>
                    fetchFileContentForDiffOrFileInfo(
                        diffOrBlobInfo,
                        checkRepoSyncError,
                        platformContext.requestGraphQL
                    ).pipe(
                        map(diffOrBlobInfo => ({
                            diffOrBlobInfo,
                            ...codeViewEvent,
                        }))
                    )
                ),
                catchError(error =>
                    // Ignore private Cloud RepoNotFound errors (don't initialize those code views)
                    from(checkRepoSyncError(error)).pipe(hasRepoSyncError => {
                        if (hasRepoSyncError) {
                            return EMPTY
                        }
                        throw error
                    })
                ),
                tap({
                    error: error => {
                        if (codeViewEvent.getToolbarMount) {
                            const mount = codeViewEvent.getToolbarMount(codeViewEvent.element)
                            render(
                                <CodeViewToolbar
                                    {...codeHost.codeViewToolbarClassProps}
                                    fileInfoOrError={error}
                                    sourcegraphURL={sourcegraphURL}
                                    telemetryService={telemetryService}
                                    telemetryRecorder={telemetryRecorder}
                                    platformContext={platformContext}
                                    extensionsController={extensionsController}
                                    buttonProps={codeViewEvent.toolbarButtonProps}
                                    // The bound function is constant
                                    onSignInClose={nextSignInClose}
                                    location={H.createLocation(window.location)}
                                />,
                                mount
                            )
                        }
                    },
                }),
                // Retry auth errors after the user closed a sign-in tab
                retry({
                    delay: error => (isHTTPAuthError(error) ? signInCloses : throwError(() => error)),
                }),
                catchError(error => {
                    // Log errors but don't break the handling of other code views
                    console.error('Could not resolve file info for code view', error)
                    return []
                })
            )
        ),
        observeOn(asyncScheduler)
    )

    /** Map from workspace URI to number of editors referencing it */
    const rootReferenceCounts = new Map<string, number>()

    /**
     * Adds root referenced by a code editor to the worskpace.
     */
    const addRootReference = async (uri: string, inputRevision: string | undefined): Promise<void> => {
        rootReferenceCounts.set(uri, (rootReferenceCounts.get(uri) || 0) + 1)
        if (rootReferenceCounts.get(uri) === 1) {
            const workspaceRoot: WorkspaceRoot = { uri, inputRevision }
            return extensionsController.extHostAPI
                .then(extensionHostAPI => extensionHostAPI.addWorkspaceRoot({ uri, inputRevision }))
                .catch(error =>
                    console.error('Sourcegraph: error adding workspace root', { error: asError(error), workspaceRoot })
                )
        }
    }

    /**
     * Deletes a reference to a workspace root from a code editor.
     */
    const deleteRootReference = async (uri: string): Promise<void> => {
        const currentReferenceCount = rootReferenceCounts.get(uri)
        if (!currentReferenceCount) {
            throw new Error(`No preexisting root refs for uri ${uri}`)
        }
        const updatedReferenceCount = currentReferenceCount - 1
        rootReferenceCounts.set(uri, updatedReferenceCount)
        if (updatedReferenceCount === 0) {
            return extensionsController.extHostAPI
                .then(extensionHostAPI => extensionHostAPI.removeWorkspaceRoot(uri))
                .catch(error =>
                    console.error('Sourcegraph: error removing workspace root', { error: asError(error), uri })
                )
        }
    }

    subscriptions.add(
        codeViews.subscribe(codeViewEvent => {
            console.log('Code view added')
            // This code view could have left the DOM between the time that
            // 1) it entered the DOM
            // 2) requests to Sourcegraph instance for repo name + file info fulfilled
            // When the code view leaves the DOM, the codeViewEvent's subscription is
            // unsubscribed, and new additions to the subscription are immediately invoked.
            // Check `wasRemoved` to prevent doing unnecessary work
            let wasRemoved = false
            codeViewEvent.subscriptions.add(() => {
                wasRemoved = true
                console.log('Code view removed')
            })

            if (wasRemoved) {
                return
            }

            ;(async () => {
                const {
                    element,
                    diffOrBlobInfo,
                    getPositionAdjuster,
                    getToolbarMount,
                    toolbarButtonProps,
                    overrideTokenize,
                } = codeViewEvent

                const initializeModelAndViewerForFileInfo = async (
                    fileInfo: FileInfoWithContent
                ): Promise<CodeEditorWithPartialModel> => {
                    const uri = makeRepoGitURI({
                        repoName: fileInfo.repoName,
                        commitID: fileInfo.commitID,
                        revision: fileInfo.revision,
                        filePath: fileInfo.filePath,
                    })

                    // Model
                    const languageId = getModeFromPath(fileInfo.filePath)
                    const model = { uri, languageId, text: fileInfo.content }

                    // Viewer
                    const editorData: CodeEditorData = {
                        type: 'CodeEditor' as const,
                        resource: uri,
                        selections: codeViewEvent.getSelections
                            ? codeViewEvent.getSelections(codeViewEvent.element)
                            : [],
                        isActive: true,
                    }

                    const extensionHostAPI = await extensionsController.extHostAPI

                    const rootURI = makeRepoGitURI({
                        repoName: fileInfo.repoName,
                        commitID: fileInfo.commitID,
                        revision: fileInfo.revision,
                    })
                    const [, viewerId] = await Promise.all([
                        // Only add the model if it doesn't exist
                        // (there may be several code views on the page pointing to the same model)
                        extensionHostAPI.addTextDocumentIfNotExists(model),
                        extensionHostAPI.addViewerIfNotExists(editorData),
                        addRootReference(rootURI, fileInfo.revision),
                    ])

                    if (codeHost.observeLineSelection) {
                        codeViewEvent.subscriptions.add(
                            codeHost.observeLineSelection
                                .pipe(
                                    map(lprToSelectionsZeroIndexed),
                                    distinctUntilChanged(isEqual),
                                    tap(selections => {
                                        extensionHostAPI
                                            .setEditorSelections(viewerId, selections)
                                            .catch(error =>
                                                console.error(
                                                    'Error updating editor selections on extension host',
                                                    error
                                                )
                                            )
                                    })
                                )

                                .subscribe()
                        )
                    }

                    // Subscribe for removal
                    codeViewEvent.subscriptions.add(() => {
                        Promise.all([deleteRootReference(rootURI), extensionHostAPI.removeViewer(viewerId)]).catch(
                            error =>
                                console.error('Sourcegraph: error removing viewer and workspace root', {
                                    error: asError(error),
                                })
                        )
                    })

                    return {
                        ...editorData,
                        ...viewerId,
                        model,
                    }
                }

                const initializeModelAndViewerForDiffOrFileInfo = async (
                    diffOrFileInfo: DiffOrBlobInfo<FileInfoWithContent>
                ): Promise<DiffOrBlobInfo<FileInfoWithContent & { editor: CodeEditorWithPartialModel }>> => {
                    if ('blob' in diffOrFileInfo) {
                        return {
                            blob: {
                                ...diffOrFileInfo.blob,
                                editor: await initializeModelAndViewerForFileInfo(diffOrFileInfo.blob),
                            },
                        }
                    }
                    if (diffOrFileInfo.head && diffOrFileInfo.base) {
                        // For diffs, both editors are created (for head and base)
                        // but only one of them is passed into
                        // the `scope` of the CodeViewToolbar component.
                        const [headEditor, baseEditor] = await Promise.all([
                            initializeModelAndViewerForFileInfo(diffOrFileInfo.head),
                            initializeModelAndViewerForFileInfo(diffOrFileInfo.base),
                        ])
                        return {
                            head: {
                                ...diffOrFileInfo.head,
                                editor: headEditor,
                            },
                            base: {
                                ...diffOrFileInfo.base,
                                editor: baseEditor,
                            },
                        }
                    }
                    if (diffOrFileInfo.base) {
                        return {
                            base: {
                                ...diffOrFileInfo.base,
                                editor: await initializeModelAndViewerForFileInfo(diffOrFileInfo.base),
                            },
                            head: undefined,
                        }
                    }
                    return {
                        head: {
                            ...diffOrFileInfo.head,
                            editor: await initializeModelAndViewerForFileInfo(diffOrFileInfo.head),
                        },
                        base: undefined,
                    }
                }

                const diffOrFileInfoWithEditor = await initializeModelAndViewerForDiffOrFileInfo(diffOrBlobInfo)

                let scopeEditor: CodeEditorWithPartialModel

                if ('blob' in diffOrFileInfoWithEditor) {
                    scopeEditor = diffOrFileInfoWithEditor.blob.editor
                } else if (diffOrFileInfoWithEditor.head) {
                    scopeEditor = diffOrFileInfoWithEditor.head.editor
                } else {
                    scopeEditor = diffOrFileInfoWithEditor.base.editor
                }

                if (wasRemoved) {
                    return
                }

                const domFunctions = {
                    ...codeViewEvent.dom,
                    // If any parent element has the sourcegraph-extension-element
                    // class then that element does not have any code. We
                    // must check for "any parent element" because extensions
                    // create their DOM changes before the blob is tokenized
                    // into multiple elements.
                    getCodeElementFromTarget: (target: HTMLElement): HTMLElement | null =>
                        target.closest('.sourcegraph-extension-element') !== null
                            ? null
                            : codeViewEvent.dom.getCodeElementFromTarget(target),
                }

                // Add hover code navigation
                const resolveContext: ContextResolver<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec> = ({
                    part,
                }) => {
                    if ('blob' in diffOrBlobInfo) {
                        return defaultRevisionToCommitID(diffOrBlobInfo.blob)
                    }
                    if (diffOrBlobInfo.head && part === 'head') {
                        return defaultRevisionToCommitID(diffOrBlobInfo.head)
                    }
                    if (diffOrBlobInfo.base && part === 'base') {
                        return defaultRevisionToCommitID(diffOrBlobInfo.base)
                    }
                    throw new Error(`Could not resolve context for diff part ${JSON.stringify(part)}`)
                }

                const adjustPosition = getPositionAdjuster?.(platformContext.requestGraphQL)
                codeViewEvent.subscriptions.add(
                    hoverifier.hoverify({
                        dom: domFunctions,
                        positionEvents: of(element).pipe(
                            findPositionsFromEvents({
                                domFunctions,
                                tokenize: !!(typeof overrideTokenize === 'boolean'
                                    ? overrideTokenize
                                    : codeHost.codeViewsRequireTokenization),
                            })
                        ),
                        resolveContext,
                        adjustPosition,
                        scrollBoundaries: codeViewEvent.getScrollBoundaries
                            ? codeViewEvent.getScrollBoundaries(codeViewEvent.element)
                            : [],
                        overrideTokenize,
                    })
                )

                element.classList.add('sg-mounted')
                // Render toolbar
                if (getToolbarMount && !minimalUI) {
                    const mount = getToolbarMount(element)
                    render(
                        <CodeViewToolbar
                            {...codeHost.codeViewToolbarClassProps}
                            actionItemClass={
                                codeViewEvent.toolbarButtonProps?.actionItemClass ??
                                codeHost.codeViewToolbarClassProps?.actionItemClass
                            }
                            hideActions={hideActions}
                            fileInfoOrError={diffOrBlobInfo}
                            sourcegraphURL={sourcegraphURL}
                            telemetryService={telemetryService}
                            telemetryRecorder={telemetryRecorder}
                            platformContext={platformContext}
                            extensionsController={extensionsController}
                            buttonProps={toolbarButtonProps}
                            location={H.createLocation(window.location)}
                            scope={scopeEditor}
                            // The bound function is constant
                            onSignInClose={nextSignInClose}
                        />,
                        mount
                    )
                }
            })().catch(error => {
                console.error('Sourcegraph: uncaught error handling code view', asError(error))
            })
        })
    )

    return subscriptions
}

const CODE_HOSTS: CodeHost[] = [
    bitbucketServerCodeHost,
    bitbucketCloudCodeHost,
    githubCodeHost,
    gitlabCodeHost,
    phabricatorCodeHost,
    gerritCodeHost,
]

const CLOUD_CODE_HOST_HOSTS = ['github.com', 'gitlab.com']

export const determineCodeHost = (sourcegraphURL?: string): CodeHost | undefined => {
    const codeHost = CODE_HOSTS.find(codeHost => codeHost.check())

    if (!codeHost) {
        return undefined
    }

    // Prevent repo lookups for code hosts that we know cannot have repositories
    // cloned on sourcegraph.com. Repo lookups trigger cloning, which will
    // inevitably fail in this case.
    if (isDefaultSourcegraphUrl(sourcegraphURL)) {
        const { hostname } = new URL(location.href)
        const validCodeHost = CLOUD_CODE_HOST_HOSTS.some(cloudHost => cloudHost === hostname)
        if (!validCodeHost) {
            console.log(
                `Sourcegraph code host integration: stopped initialization since ${hostname} is not a supported code host when Sourcegraph URL is ${DEFAULT_SOURCEGRAPH_URL}.\n List of supported code hosts on ${DEFAULT_SOURCEGRAPH_URL}: ${CLOUD_CODE_HOST_HOSTS.join(
                    ', '
                )}`
            )
            return undefined
        }
    }

    return codeHost
}

function initializeGithubSearchInputEnhancement(
    searchEnhancement: NonNullable<GithubCodeHost['searchEnhancement']>,
    sourcegraphURL: string,
    mutations: Observable<MutationRecordLike[]>
): Subscription {
    const { searchViewResolver, resultViewResolver, onChange } = searchEnhancement
    const searchURL = createURLWithUTM(new URL('/search', sourcegraphURL), {
        utm_source: getPlatformName(),
        utm_campaign: 'global-search',
    })

    const searchView = mutations.pipe(
        trackViews([searchViewResolver]),
        switchMap(({ element }) =>
            fromEvent(element, 'input').pipe(
                map(event => (event.target as HTMLInputElement).value),
                startWith((element as HTMLInputElement).value)
            )
        ),
        map(value => ({
            value,
            searchURL: searchURL.href,
        })),
        observeOn(asyncScheduler)
    )
    const resultView = mutations.pipe(trackViews([resultViewResolver]), observeOn(asyncScheduler))

    return combineLatest([searchView, resultView])
        .pipe(map(([search, { element: resultElement }]) => ({ ...search, resultElement })))
        .subscribe(onChange)
}

export function injectCodeIntelligenceToCodeHost(
    mutations: Observable<MutationRecordLike[]>,
    codeHost: CodeHost,
    { sourcegraphURL, assetsURL }: SourcegraphIntegrationURLs,
    isExtension: boolean
): Subscription {
    const subscriptions = new Subscription()
    const { platformContext, extensionsController } = initializeExtensions(
        codeHost,
        { sourcegraphURL, assetsURL },
        isExtension
    )
    const { requestGraphQL } = platformContext

    if (extensionsController !== null) {
        subscriptions.add(extensionsController)
    }

    const codeHostReady = codeHost.prepareCodeHost ? from(codeHost.prepareCodeHost(requestGraphQL)) : of(true)

    const isTelemetryEnabled = combineLatest([
        observeSendTelemetry(isExtension),
        from(codeHost.getContext?.().then(context => context.privateRepository) ?? Promise.resolve(true)),
    ]).pipe(
        map(
            ([sendTelemetry, isPrivateRepo]) =>
                sendTelemetry &&
                /** Enable telemetry if: a) this is a self-hosted Sourcegraph instance; b) or public repository; */
                (!isDefaultSourcegraphUrl(sourcegraphURL) || !isPrivateRepo)
        )
    )

    const innerTelemetryService = new EventLogger(requestGraphQL, sourcegraphURL)
    const telemetryService = new ConditionalTelemetryService(innerTelemetryService, isTelemetryEnabled)
    subscriptions.add(telemetryService)

    const telemetryRecorderProvider = new ConditionalTelemetryRecorderProvider(isTelemetryEnabled, requestGraphQL)
    const telemetryRecorder = telemetryRecorderProvider.getRecorder()
    subscriptions.add(telemetryRecorder)
    platformContext.telemetryRecorder = telemetryRecorder

    let codeHostSubscription: Subscription
    // In the browser extension, observe whether the `disableExtension` storage flag is set.
    // In the native integration, this flag does not exist.
    const extensionDisabled = isExtension ? observeStorageKey('sync', 'disableExtension') : of(false)

    // RFC 68: hide some UI features in the GitLab native integration.
    // This can be overridden using the `sourcegraphMinimalUI` local storage flag.
    const minimalUIStorageFlag = localStorage.getItem('sourcegraphMinimalUI')
    const minimalUI =
        minimalUIStorageFlag !== null ? minimalUIStorageFlag === 'true' : codeHost.type === 'gitlab' && !isExtension
    // Flag to hide the actions in the code view toolbar (hide ActionNavItems) leaving only the "Open on Sourcegraph" button in the toolbar.
    const hideActions = codeHost.type === 'gerrit'

    const renderWithThemeProvider = (element: React.ReactNode, container: Element | null): void => {
        if (!container) {
            return
        }

        const root = createRoot(container)
        root.render(<WildcardThemeProvider isBranded={false}>{element}</WildcardThemeProvider>)
    }

    subscriptions.add(
        // eslint-disable-next-line rxjs/no-async-subscribe
        combineLatest([codeHostReady, extensionDisabled]).subscribe(async ([isCodeHostReady, disableExtension]) => {
            if (disableExtension) {
                // We don't need to unsubscribe if the extension starts with disabled state.
                if (codeHostSubscription) {
                    codeHostSubscription.unsubscribe()
                }
                console.log('Browser extension is disabled')
            } else if (isCodeHostReady && extensionsController !== null) {
                codeHostSubscription = await handleCodeHost({
                    mutations,
                    codeHost,
                    extensionsController,
                    platformContext,
                    telemetryService,
                    telemetryRecorder,
                    render: renderWithThemeProvider as Renderer,
                    minimalUI,
                    hideActions,
                    background,
                })
                subscriptions.add(codeHostSubscription)
                console.log(`${isExtension ? 'Browser extension' : 'Native integration'} is enabled`)
            }
        })
    )
    return subscriptions
}
