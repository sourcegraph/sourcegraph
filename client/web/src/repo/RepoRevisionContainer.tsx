import { FC, useCallback, useMemo, useState } from 'react'

import { Route, Routes, useLocation } from 'react-router-dom'

import { StreamingSearchResultsListProps, CopyPathAction } from '@sourcegraph/branded'
import { isErrorLike } from '@sourcegraph/common'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { SearchContextProps } from '@sourcegraph/shared/src/search'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { RevisionSpec } from '@sourcegraph/shared/src/util/url'
import { Button, LoadingSpinner, Popover, PopoverContent, PopoverTrigger, Position } from '@sourcegraph/wildcard'

import { AuthenticatedUser } from '../auth'
import { BatchChangesProps } from '../batches'
import { CodeIntelligenceProps } from '../codeintel'
import { BreadcrumbSetters } from '../components/Breadcrumbs'
import { RepositoryFields } from '../graphql-operations'
import { CodeInsightsProps } from '../insights/types'
import { NotebookProps } from '../notebooks'
import { SearchStreamingProps } from '../search'
import { eventLogger } from '../tracking/eventLogger'
import { RouteV6Descriptor } from '../util/contributions'
import { parseBrowserRepoURL } from '../util/url'

import { GoToPermalinkAction } from './actions/GoToPermalinkAction'
import { ResolvedRevision } from './backend'
import { RepoRevisionChevronDownIcon, RepoRevisionWrapper } from './components/RepoRevision'
import { isPackageServiceType } from './packages/isPackageServiceType'
import { HoverThresholdProps, RepoContainerContext } from './RepoContainer'
import { RepoHeaderContributionsLifecycleProps } from './RepoHeader'
import { RepoHeaderContributionPortal } from './RepoHeaderContributionPortal'
import { RevisionsPopover } from './RevisionsPopover'
import { RepoSettingsAreaRoute } from './settings/RepoSettingsArea'
import { RepoSettingsSideBarGroup } from './settings/RepoSettingsSidebar'

import styles from './RepoRevisionContainer.module.scss'

/** Props passed to sub-routes of {@link RepoRevisionContainer}. */
export interface RepoRevisionContainerContext
    extends RepoHeaderContributionsLifecycleProps,
        SettingsCascadeProps,
        PlatformContextProps,
        TelemetryProps,
        HoverThresholdProps,
        Omit<RepoContainerContext, 'onDidUpdateExternalLinks' | 'repo' | 'resolvedRevisionOrError'>,
        Pick<SearchContextProps, 'selectedSearchContextSpec' | 'searchContextsEnabled'>,
        RevisionSpec,
        BreadcrumbSetters,
        SearchStreamingProps,
        Pick<StreamingSearchResultsListProps, 'fetchHighlightedFileLineRanges'>,
        BatchChangesProps,
        Pick<CodeIntelligenceProps, 'codeIntelligenceEnabled' | 'useCodeIntel'>,
        CodeInsightsProps,
        NotebookProps {
    repo: RepositoryFields | undefined
    resolvedRevision: ResolvedRevision | undefined

    repoName: string

    globbing: boolean

    isMacPlatform: boolean

    isSourcegraphDotCom: boolean
}

/** A sub-route of {@link RepoRevisionContainer}. */
export interface RepoRevisionContainerRoute extends RouteV6Descriptor<RepoRevisionContainerContext> {}

interface RepoRevisionContainerProps
    extends RepoHeaderContributionsLifecycleProps,
        SettingsCascadeProps,
        PlatformContextProps,
        TelemetryProps,
        HoverThresholdProps,
        Pick<SearchContextProps, 'selectedSearchContextSpec' | 'searchContextsEnabled'>,
        RevisionSpec,
        BreadcrumbSetters,
        SearchStreamingProps,
        Pick<StreamingSearchResultsListProps, 'fetchHighlightedFileLineRanges'>,
        CodeIntelligenceProps,
        BatchChangesProps,
        CodeInsightsProps,
        NotebookProps {
    routes: readonly RepoRevisionContainerRoute[]
    repoSettingsAreaRoutes: readonly RepoSettingsAreaRoute[]
    repoSettingsSidebarGroups: readonly RepoSettingsSideBarGroup[]
    repo: RepositoryFields | undefined
    authenticatedUser: AuthenticatedUser | null

    /**
     * The resolved revision or an error if it could not be resolved.
     */
    resolvedRevision: ResolvedRevision | undefined

    /** The repoName from the URL */
    repoName: string

    globbing: boolean

    isMacPlatform: boolean

    isSourcegraphDotCom: boolean
}

interface RepoRevisionBreadcrumbProps extends Pick<RepoRevisionContainerProps, 'repo' | 'revision' | 'repoName'> {
    resolvedRevision: ResolvedRevision | undefined
}

export const RepoRevisionContainerBreadcrumb: FC<RepoRevisionBreadcrumbProps> = props => {
    const { revision, resolvedRevision, repoName, repo } = props

    const [popoverOpen, setPopoverOpen] = useState(false)
    const togglePopover = useCallback(() => setPopoverOpen(previous => !previous), [])

    const revisionLabel = (revision && revision === resolvedRevision?.commitID
        ? resolvedRevision?.commitID.slice(0, 7)
        : revision.slice(0, 7)) ||
        resolvedRevision?.defaultBranch || <LoadingSpinner />

    const isPopoverContentReady = repo && resolvedRevision

    return (
        <Popover isOpen={popoverOpen} onOpenChange={event => setPopoverOpen(event.isOpen)}>
            <PopoverTrigger
                as={Button}
                className="d-flex align-items-center text-nowrap"
                key="repo-revision"
                id="repo-revision-popover"
                aria-label="Change revision"
                outline={true}
                variant="secondary"
                size="sm"
                disabled={!isPopoverContentReady}
            >
                {revisionLabel}
                <RepoRevisionChevronDownIcon aria-hidden={true} />
            </PopoverTrigger>
            <PopoverContent
                position={Position.bottomStart}
                className="pt-0 pb-0"
                aria-labelledby="repo-revision-popover"
            >
                {isPopoverContentReady && (
                    <RevisionsPopover
                        repoId={repo?.id}
                        repoName={repoName}
                        defaultBranch={resolvedRevision?.defaultBranch}
                        currentRev={revision}
                        currentCommitID={resolvedRevision?.commitID}
                        togglePopover={togglePopover}
                        onSelect={togglePopover}
                    />
                )}
            </PopoverContent>
        </Popover>
    )
}

/**
 * A container for a repository page that incorporates revisioned Git data. (For example,
 * blob and tree pages are revisioned, but the repository settings page is not.)
 */
export const RepoRevisionContainer: FC<RepoRevisionContainerProps> = props => {
    const { useBreadcrumb, resolvedRevision, revision, repo, repoName, routes } = props
    const location = useLocation()

    const breadcrumbSetters = useBreadcrumb(
        useMemo(() => {
            if (isErrorLike(resolvedRevision)) {
                return
            }

            return {
                key: 'revision',
                divider: <span className={styles.divider}>@</span>,
                element: (
                    <RepoRevisionContainerBreadcrumb
                        resolvedRevision={resolvedRevision}
                        revision={revision}
                        repoName={repoName}
                        repo={repo}
                    />
                ),
            }
        }, [resolvedRevision, revision, repo, repoName])
    )

    const isPackage = useMemo(
        () => isPackageServiceType(repo?.externalRepository.serviceType),
        [repo?.externalRepository.serviceType]
    )

    const repoRevisionContainerContext: RepoRevisionContainerContext = {
        ...props,
        ...breadcrumbSetters,
        resolvedRevision,
    }

    const { filePath } = parseBrowserRepoURL(location.pathname)

    return (
        <RepoRevisionWrapper className="px-3">
            <Routes>
                {routes.map(
                    ({ path, render, condition = () => true }) =>
                        condition(repoRevisionContainerContext) && (
                            <Route key="hardcoded-key" path={path} element={render(repoRevisionContainerContext)} />
                        )
                )}
            </Routes>
            <RepoHeaderContributionPortal
                position="left"
                id="copy-path"
                repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
            >
                {() => (
                    <CopyPathAction telemetryService={eventLogger} filePath={filePath || repoName} key="copy-path" />
                )}
            </RepoHeaderContributionPortal>
            {resolvedRevision && !isPackage && (
                <RepoHeaderContributionPortal
                    position="right"
                    priority={3}
                    id="go-to-permalink"
                    repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
                >
                    {context => (
                        <GoToPermalinkAction
                            key="go-to-permalink"
                            telemetryService={props.telemetryService}
                            revision={props.revision}
                            commitID={resolvedRevision.commitID}
                            {...context}
                        />
                    )}
                </RepoHeaderContributionPortal>
            )}
        </RepoRevisionWrapper>
    )
}
