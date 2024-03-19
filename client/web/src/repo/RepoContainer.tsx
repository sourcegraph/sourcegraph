import React, {
    createContext,
    type FC,
    type PropsWithChildren,
    type RefObject,
    Suspense,
    useContext,
    useEffect,
    useMemo,
    useRef,
    useState,
} from 'react'

import classNames from 'classnames'
import { escapeRegExp } from 'lodash'
import { createPortal } from 'react-dom'
import { type Location, useLocation, Route, Routes } from 'react-router-dom'
import { NEVER, of } from 'rxjs'
import { catchError, switchMap } from 'rxjs/operators'

import type { StreamingSearchResultsListProps } from '@sourcegraph/branded'
import { asError, type ErrorLike, isErrorLike, repeatUntil } from '@sourcegraph/common'
import {
    isCloneInProgressErrorLike,
    isRepoSeeOtherErrorLike,
    isRevisionNotFoundErrorLike,
} from '@sourcegraph/shared/src/backend/errors'
import { RepoQuestionIcon } from '@sourcegraph/shared/src/components/icons'
import { useKeyboardShortcut } from '@sourcegraph/shared/src/keyboardShortcuts/useKeyboardShortcut'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import type { Settings } from '@sourcegraph/shared/src/schema/settings.schema'
import { EditorHint, type SearchContextProps } from '@sourcegraph/shared/src/search'
import { escapeSpaces } from '@sourcegraph/shared/src/search/query/filters'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { LoadingSpinner, Panel, useObservable } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import type { BatchChangesProps } from '../batches'
import type { CodeIntelligenceProps } from '../codeintel'
import { RepoContainerEditor } from '../cody/components/RepoContainerEditor'
import { CodySidebar } from '../cody/sidebar'
import { useCodySidebar, useSidebarSize, CODY_SIDEBAR_SIZES } from '../cody/sidebar/Provider'
import type { BreadcrumbSetters, BreadcrumbsProps } from '../components/Breadcrumbs'
import { RouteError } from '../components/ErrorBoundary'
import { HeroPage } from '../components/HeroPage'
import type { ExternalLinkFields, RepositoryFields } from '../graphql-operations'
import type { CodeInsightsProps } from '../insights/types'
import type { NotebookProps } from '../notebooks'
import type { OwnConfigProps } from '../own/OwnConfigProps'
import { searchQueryForRepoRevision, type SearchStreamingProps } from '../search'
import { useV2QueryInput } from '../search/useV2QueryInput'
import { useNavbarQueryState } from '../stores'
import { EventName } from '../util/constants'
import type { RouteV6Descriptor } from '../util/contributions'
import { getLicenseFeatures } from '../util/license'
import { parseBrowserRepoURL } from '../util/url'

import { GoToCodeHostAction } from './actions/GoToCodeHostAction'
import { fetchFileExternalLinks, type ResolvedRevision, resolveRepoRevision, type Repo } from './backend'
import { AskCodyButton } from './cody/AskCodyButton'
import { RepoContainerError } from './RepoContainerError'
import { RepoHeader, type RepoHeaderContributionsLifecycleProps } from './RepoHeader'
import { RepoHeaderContributionPortal } from './RepoHeaderContributionPortal'
import { RepoLinkPicker } from './RepoLinkPicker'
import {
    RepoRevisionContainer,
    type RepoRevisionContainerContext,
    type RepoRevisionContainerRoute,
} from './RepoRevisionContainer'
import type { RepoSettingsAreaRoute } from './settings/RepoSettingsArea'
import type { RepoSettingsSideBarGroup } from './settings/RepoSettingsSidebar'
import { repoSettingsAreaPath } from './settings/routes'

import styles from './RepoContainer.module.scss'

const RepoSettingsArea = lazyComponent(() => import('./settings/RepoSettingsArea'), 'RepoSettingsArea')

/**
 * Props passed to sub-routes of {@link RepoContainer}.
 */
export interface RepoContainerContext
    extends RepoHeaderContributionsLifecycleProps,
        SettingsCascadeProps,
        PlatformContextProps,
        HoverThresholdProps,
        TelemetryProps,
        Pick<SearchContextProps, 'selectedSearchContextSpec' | 'searchContextsEnabled'>,
        BreadcrumbSetters,
        SearchStreamingProps,
        Pick<StreamingSearchResultsListProps, 'fetchHighlightedFileLineRanges'>,
        CodeIntelligenceProps,
        BatchChangesProps,
        CodeInsightsProps {
    repo: RepositoryFields
    repoName: string
    resolvedRevisionOrError: ResolvedRevision | ErrorLike | undefined
    authenticatedUser: AuthenticatedUser | null
    repoSettingsAreaRoutes: readonly RepoSettingsAreaRoute[]
    repoSettingsSidebarGroups: readonly RepoSettingsSideBarGroup[]

    onDidUpdateExternalLinks: (externalLinks: ExternalLinkFields[] | undefined) => void

    isMacPlatform: boolean

    isSourcegraphDotCom: boolean
}

/**
 * Props passed to sub-routes of {@link RepoContainer} which are specific to repository settings.
 */
export interface RepoSettingsContainerContext extends Omit<RepoContainerContext, 'repo' | 'resolvedRevisionOrError'> {}

/** A sub-route of {@link RepoContainer}. */
export interface RepoContainerRoute extends RouteV6Descriptor<RepoContainerContext> {}

interface RepoContainerProps
    extends SettingsCascadeProps<Settings>,
        PlatformContextProps,
        TelemetryProps,
        Pick<SearchContextProps, 'selectedSearchContextSpec' | 'searchContextsEnabled'>,
        BreadcrumbSetters,
        BreadcrumbsProps,
        SearchStreamingProps,
        Pick<StreamingSearchResultsListProps, 'fetchHighlightedFileLineRanges'>,
        CodeIntelligenceProps,
        BatchChangesProps,
        CodeInsightsProps,
        NotebookProps,
        OwnConfigProps {
    repoContainerRoutes: readonly RepoContainerRoute[]
    repoRevisionContainerRoutes: readonly RepoRevisionContainerRoute[]
    repoSettingsAreaRoutes: readonly RepoSettingsAreaRoute[]
    repoSettingsSidebarGroups: readonly RepoSettingsSideBarGroup[]
    authenticatedUser: AuthenticatedUser | null
    isMacPlatform: boolean
    isSourcegraphDotCom: boolean
}

export interface HoverThresholdProps {
    /**
     * Called when a hover with content is shown.
     */
    onHoverShown?: () => void
}

/**
 * Renders a horizontal bar and content for a repository page.
 */
export const RepoContainer: FC<RepoContainerProps> = props => {
    const { authenticatedUser } = props

    const location = useLocation()

    const { repoName, revision, rawRevision } = parseBrowserRepoURL(location.pathname + location.search + location.hash)

    const resolvedRevisionOrError = useObservable(
        useMemo(
            () =>
                of(undefined)
                    .pipe(
                        // Wrap in switchMap, so we don't break the observable chain when
                        // catchError returns a new observable, so repeatUntil will
                        // properly resubscribe to the outer observable and re-fetch.
                        switchMap(() =>
                            resolveRepoRevision({ repoName, revision }).pipe(
                                catchError(error => {
                                    const redirect = isRepoSeeOtherErrorLike(error)

                                    if (redirect) {
                                        redirectToExternalHost(redirect)
                                        return NEVER
                                    }

                                    if (isCloneInProgressErrorLike(error)) {
                                        return of(asError(error))
                                    }

                                    throw error
                                })
                            )
                        )
                    )
                    .pipe(
                        repeatUntil(value => !isCloneInProgressErrorLike(value), { delay: 1000 }),
                        catchError(error => of(asError(error)))
                    ),
            [repoName, revision]
        )
    )

    /**
     * A long time ago, we fetched `repo` in a separate GraphQL query.
     * This GraphQL query was merged into the `resolveRevision` query to
     * speed up the network requests waterfall. To minimize the blast radius
     * of changes required to make it work, continue working with the `repo`
     * data as if it was received from a separate query.
     */
    const repoOrError = isErrorLike(resolvedRevisionOrError) ? resolvedRevisionOrError : resolvedRevisionOrError?.repo

    // The lifecycle props for repo header contributions.
    const [repoHeaderContributionsLifecycleProps, setRepoHeaderContributionsLifecycleProps] =
        useState<RepoHeaderContributionsLifecycleProps>()

    const childBreadcrumbSetters = props.useBreadcrumb(
        useMemo(() => {
            if (isErrorLike(resolvedRevisionOrError) || isErrorLike(repoOrError)) {
                return
            }

            return {
                key: 'repository',
                element: (
                    <RepoLinkPicker
                        repositoryName={repoName}
                        repositoryURL={resolvedRevisionOrError?.rootTreeURL || repoOrError?.url || ''}
                        disabled={!resolvedRevisionOrError}
                    />
                ),
            }
        }, [resolvedRevisionOrError, repoOrError, repoName])
    )

    // must exactly match how the revision was encoded in the URL
    const repoNameAndRevision = `${repoName}${typeof rawRevision === 'string' ? `@${rawRevision}` : ''}`

    return (
        <RepoContainerRoot>
            <div className={classNames('w-100 d-flex flex-column', styles.repoContainer)}>
                <RepoHeader
                    breadcrumbs={props.breadcrumbs}
                    repoName={repoName}
                    revision={revision}
                    onLifecyclePropsChange={setRepoHeaderContributionsLifecycleProps}
                    settingsCascade={props.settingsCascade}
                    authenticatedUser={authenticatedUser}
                    platformContext={props.platformContext}
                />

                <Suspense fallback={<LoadingSpinner />}>
                    <Routes>
                        {props.authenticatedUser?.siteAdmin && (
                            <Route
                                path={repoNameAndRevision + repoSettingsAreaPath}
                                errorElement={<RouteError />}
                                // Always render the `RepoSettingsArea` even for empty repo to allow side-admins access it.
                                element={
                                    <RepoSettingsArea
                                        repoName={repoName}
                                        authenticatedUser={props.authenticatedUser}
                                        repoSettingsAreaRoutes={props.repoSettingsAreaRoutes}
                                        repoSettingsSidebarGroups={props.repoSettingsSidebarGroups}
                                        setBreadcrumb={childBreadcrumbSetters.setBreadcrumb}
                                        useBreadcrumb={childBreadcrumbSetters.useBreadcrumb}
                                        telemetryService={props.telemetryService}
                                        telemetryRecorder={props.platformContext.telemetryRecorder}
                                    />
                                }
                            />
                        )}
                        <Route
                            path="*"
                            errorElement={<RouteError />}
                            element={
                                <RepoUserContainer
                                    {...props}
                                    childBreadcrumbSetters={childBreadcrumbSetters}
                                    repoOrError={repoOrError}
                                    resolvedRevisionOrError={resolvedRevisionOrError}
                                    repoHeaderContributionsLifecycleProps={repoHeaderContributionsLifecycleProps}
                                />
                            }
                        />
                    </Routes>
                </Suspense>
            </div>
        </RepoContainerRoot>
    )
}

interface RepoContainerRootContextData {
    rootElement: RefObject<HTMLElement>
}

const RepoContainerRootContext = createContext<RepoContainerRootContextData>({
    rootElement: { current: null },
})

const RepoContainerRoot: FC<PropsWithChildren<{}>> = props => {
    const { children } = props
    const rootElementRef = useRef<HTMLDivElement>(null)

    return (
        <div ref={rootElementRef} className={classNames('w-100 d-flex flex-row')}>
            <RepoContainerRootContext.Provider value={{ rootElement: rootElementRef }}>
                {children}
            </RepoContainerRootContext.Provider>
        </div>
    )
}

const RepoContainerRootPortal: FC<PropsWithChildren<{}>> = props => {
    const { children } = props
    const { rootElement } = useContext(RepoContainerRootContext)

    if (!rootElement.current) {
        return null
    }

    return createPortal(children, rootElement.current)
}

interface RepoUserContainerProps extends RepoContainerProps {
    repoHeaderContributionsLifecycleProps?: RepoHeaderContributionsLifecycleProps
    resolvedRevisionOrError: (ResolvedRevision & Repo) | ErrorLike | undefined
    repoOrError: ErrorLike | RepositoryFields | undefined
    childBreadcrumbSetters: BreadcrumbSetters
}

const RepoUserContainer: FC<RepoUserContainerProps> = ({
    resolvedRevisionOrError,
    repoOrError,
    childBreadcrumbSetters,
    repoHeaderContributionsLifecycleProps,
    ...props
}) => {
    const { repoContainerRoutes, authenticatedUser, selectedSearchContextSpec } = props

    const location = useLocation()

    const { repoName, revision, rawRevision, filePath, commitRange, position, range } = parseBrowserRepoURL(
        location.pathname + location.search + location.hash
    )

    const {
        isSidebarOpen: isCodySidebarOpen,
        setIsSidebarOpen: setIsCodySidebarOpen,
        scope,
        setEditorScope,
        logTranscriptEvent,
    } = useCodySidebar()

    const { sidebarSize, setSidebarSize: setCodySidebarSize } = useSidebarSize()

    /* eslint-disable react-hooks/exhaustive-deps */
    const codySidebarSize = useMemo(() => sidebarSize, [isCodySidebarOpen])
    /* eslint-enable react-hooks/exhaustive-deps */

    useEffect(() => {
        const activeEditor = scope.editor.getActiveTextEditor()

        if (activeEditor?.repoName !== repoName) {
            setEditorScope(new RepoContainerEditor(repoName))
        }
    }, [scope.editor, repoName, setEditorScope])

    const focusCodyShortcut = useKeyboardShortcut('focusCody')

    // The external links to show in the repository header, if any.
    const [externalLinks, setExternalLinks] = useState<ExternalLinkFields[] | undefined>()

    // Update the navbar query to reflect the current repo / revision
    const [enableV2QueryInput] = useV2QueryInput()
    const queryPrefix = useMemo(
        () => (enableV2QueryInput && selectedSearchContextSpec ? `context:${selectedSearchContextSpec} ` : ''),
        [enableV2QueryInput, selectedSearchContextSpec]
    )
    const onNavbarQueryChange = useNavbarQueryState(state => state.setQueryState)
    useEffect(() => {
        let query = queryPrefix + searchQueryForRepoRevision(repoName, revision)
        if (filePath) {
            query = `${query.trimEnd()} file:${escapeSpaces('^' + escapeRegExp(filePath))}`
        }
        onNavbarQueryChange({
            query,
            hint: EditorHint.Blur,
        })
    }, [revision, filePath, repoName, onNavbarQueryChange, queryPrefix])

    const isError = isErrorLike(repoOrError) || isErrorLike(resolvedRevisionOrError)

    // if revision for given repo does not resolve then we still proceed to render settings routes
    // while returning empty repository for all other routes
    const isEmptyRepo = isRevisionNotFoundErrorLike(repoOrError)

    // For repo errors beyond revision not found (aka empty repository)
    // we defer to RepoContainerError for every repo container request
    if (isError && !isEmptyRepo) {
        const viewerCanAdminister = !!authenticatedUser && authenticatedUser.siteAdmin

        return (
            <RepoContainerError
                repoName={repoName}
                viewerCanAdminister={viewerCanAdminister}
                repoFetchError={repoOrError as ErrorLike}
            />
        )
    }

    const repo = isError ? undefined : repoOrError
    const resolvedRevision = isError ? undefined : resolvedRevisionOrError
    const isBrainDotVisible = getIsBrainDotVisible({
        location,
        revision,
        repoName,
    })

    const repoRevisionContainerContext: RepoRevisionContainerContext = {
        ...props,
        ...repoHeaderContributionsLifecycleProps,
        ...childBreadcrumbSetters,
        repo,
        repoName,
        revision: revision || '',
        resolvedRevision,
    }

    const perforceCodeHostUrlToSwarmUrlMap =
        (props.settingsCascade.final &&
            !isErrorLike(props.settingsCascade.final) &&
            props.settingsCascade.final?.['perforce.codeHostToSwarmMap']) ||
        {}

    const repoContainerContext: Omit<RepoContainerContext, 'repo'> = {
        ...repoRevisionContainerContext,
        resolvedRevisionOrError,
        onDidUpdateExternalLinks: setExternalLinks,
        repoName,
    }

    // must exactly match how the revision was encoded in the URL
    const repoNameAndRevision = `${repoName}${typeof rawRevision === 'string' ? `@${rawRevision}` : ''}`
    const licenseFeatures = getLicenseFeatures()
    const showAskCodyBtn = licenseFeatures.isCodyEnabled && !isCodySidebarOpen

    return (
        <>
            {focusCodyShortcut?.keybindings.map((keybinding, index) => (
                <Shortcut
                    key={index}
                    {...keybinding}
                    onMatch={() => {
                        setIsCodySidebarOpen(true)
                    }}
                />
            ))}

            {showAskCodyBtn && (
                <RepoHeaderContributionPortal
                    position="right"
                    priority={1}
                    id="cody"
                    {...repoHeaderContributionsLifecycleProps}
                >
                    {() => (
                        <AskCodyButton
                            onClick={() => {
                                logTranscriptEvent(EventName.CODY_SIDEBAR_CHAT_OPENED, 'repo.askCody', 'click', {
                                    repo,
                                    path: filePath,
                                })
                                setIsCodySidebarOpen(true)
                            }}
                        />
                    )}
                </RepoHeaderContributionPortal>
            )}

            <RepoHeaderContributionPortal
                position="right"
                priority={3}
                id="go-to-code-host"
                {...repoHeaderContributionsLifecycleProps}
            >
                {({ actionType }) => (
                    <GoToCodeHostAction
                        repo={repo}
                        repoName={repoName}
                        // We need a revision to generate code host URLs, if revision isn't available, we use the default branch or HEAD.
                        revision={rawRevision || repo?.defaultBranch?.displayName || 'HEAD'}
                        filePath={filePath}
                        commitRange={commitRange}
                        range={range}
                        position={position}
                        perforceCodeHostUrlToSwarmUrlMap={perforceCodeHostUrlToSwarmUrlMap}
                        fetchFileExternalLinks={fetchFileExternalLinks}
                        actionType={actionType}
                        source="repoHeader"
                        key="go-to-code-host"
                        externalLinks={externalLinks}
                    />
                )}
            </RepoHeaderContributionPortal>

            {isBrainDotVisible && (
                <RepoHeaderContributionPortal
                    position="right"
                    priority={7}
                    id="code-intelligence-status"
                    renderInContextMenu={true}
                    {...repoHeaderContributionsLifecycleProps}
                >
                    {({ actionType }) =>
                        props.brainDot && actionType === 'nav' ? (
                            <props.brainDot
                                key="code-intelligence-status"
                                repoName={repoName}
                                path={filePath}
                                commit={resolvedRevision?.commitID ?? ''}
                            />
                        ) : null
                    }
                </RepoHeaderContributionPortal>
            )}

            <Suspense fallback={<LoadingSpinner />}>
                <Routes>
                    {repoContainerRoutes.map(({ path, render, condition = () => true }) => (
                        <Route
                            key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                            path={repoNameAndRevision + path}
                            errorElement={<RouteError />}
                            element={
                                /**
                                 * `repoContainerRoutes` depend on `repo`. We render these routes only when
                                 * the `repo` value is resolved. If repo resolves to error due to empty repository
                                 * then we return Empty Repository.
                                 */
                                repo && condition({ ...repoContainerContext, repo }) ? (
                                    render({ ...repoContainerContext, repo })
                                ) : isEmptyRepo ? (
                                    <EmptyRepo />
                                ) : null
                            }
                        />
                    ))}
                    <Route
                        key="hardcoded-key" // see https://github.com/ReactTraining/react-router/issues/4578#issuecomment-334489490
                        path={repoNameAndRevision + '/*'}
                        errorElement={<RouteError />}
                        element={
                            isEmptyRepo ? (
                                <EmptyRepo />
                            ) : (
                                <RepoRevisionContainer
                                    {...repoRevisionContainerContext}
                                    {...childBreadcrumbSetters}
                                    routes={props.repoRevisionContainerRoutes}
                                />
                            )
                        }
                    />
                </Routes>
            </Suspense>

            {isCodySidebarOpen && (
                <RepoContainerRootPortal>
                    <Panel
                        className="cody-sidebar-panel"
                        position="right"
                        ariaLabel="Cody sidebar"
                        maxSize={CODY_SIDEBAR_SIZES.max}
                        minSize={CODY_SIDEBAR_SIZES.min}
                        defaultSize={codySidebarSize || CODY_SIDEBAR_SIZES.default}
                        storageKey="size-cache-cody-sidebar"
                        onResize={setCodySidebarSize}
                    >
                        <CodySidebar
                            onClose={() => setIsCodySidebarOpen(false)}
                            authenticatedUser={props.authenticatedUser}
                            telemetryRecorder={props.platformContext.telemetryRecorder}
                        />
                    </Panel>
                </RepoContainerRootPortal>
            )}
        </>
    )
}

function getIsBrainDotVisible({
    location,
    repoName,
    revision,
}: {
    location: Location
    repoName: string
    revision: string | undefined
}): boolean {
    // Remove leading repository name and possible leading revision, then compare the remaining routes to
    // see if we should display the braindot badge for this route. We want this to be visible on the repo
    // root page, as well as directory and code views, but not administrative/non-code views.
    //
    // + 1 for the leading `/` in the pathname
    const matchRevisionAndRest = location.pathname.slice(repoName.length + 1)
    const matchOnlyRest =
        revision && matchRevisionAndRest.startsWith(`@${revision || ''}`)
            ? matchRevisionAndRest.slice(revision.length + 1)
            : matchRevisionAndRest

    return matchOnlyRest === '' || matchOnlyRest.startsWith('/-/tree') || matchOnlyRest.startsWith('/-/blob')
}

/**
 * Performs a redirect to the host of the given URL with the path, query etc. properties of the current URL.
 */
function redirectToExternalHost(externalRedirectURL: string): void {
    const externalHostURL = new URL(externalRedirectURL)
    const redirectURL = new URL(window.location.href)
    // Preserve the path of the current URL and redirect to the repo on the external host.
    redirectURL.host = externalHostURL.host
    redirectURL.protocol = externalHostURL.protocol
    window.location.replace(redirectURL.href)
}

const EmptyRepo: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <HeroPage icon={RepoQuestionIcon} title="Empty repository" />
)
