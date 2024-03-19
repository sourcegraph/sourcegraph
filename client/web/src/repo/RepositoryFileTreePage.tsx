import type { FC } from 'react'

import { Navigate, useLocation } from 'react-router-dom'

import { appendLineRangeQueryParameter } from '@sourcegraph/common'
import { TraceSpanProvider } from '@sourcegraph/observability-client'
import { getModeFromPath } from '@sourcegraph/shared/src/languages'
import { isLegacyFragment, parseQueryAndHash, toRepoURL } from '@sourcegraph/shared/src/util/url'
import { LoadingSpinner } from '@sourcegraph/wildcard'

import { ErrorBoundary } from '../components/ErrorBoundary'
import type { SourcegraphContext } from '../jscontext'
import type { NotebookProps } from '../notebooks'
import type { OwnConfigProps } from '../own/OwnConfigProps'
import { GettingStartedTour } from '../tour/GettingStartedTour'
import { formatHash, formatLineOrPositionOrRange, parseBrowserRepoURL } from '../util/url'

import { BlobPage } from './blob/BlobPage'
import type { RepoRevisionContainerContext } from './RepoRevisionContainer'
import { RepoRevisionSidebar } from './RepoRevisionSidebar'
import { TreePage } from './tree/TreePage'

import styles from './RepositoryFileTreePage.module.scss'

export interface RepositoryFileTreePageProps extends RepoRevisionContainerContext, NotebookProps, OwnConfigProps {
    objectType: 'blob' | 'tree' | undefined
    globalContext: Pick<SourcegraphContext, 'externalURL'>
}

/** Dev feature flag to make benchmarking the file tree in isolation easier. */
const hideRepoRevisionContent = localStorage.getItem('hideRepoRevContent')

/**
 * A page that shows a file or a directory (tree view) in a repository at the
 * current revision.
 */
export const RepositoryFileTreePage: FC<RepositoryFileTreePageProps> = props => {
    const {
        repo,
        resolvedRevision,
        repoName,
        objectType: maybeObjectType,
        globalContext,
        platformContext,
        ...context
    } = props

    const location = useLocation()
    const { filePath = '' } = parseBrowserRepoURL(location.pathname) // empty string is root

    // Redirect tree and blob routes pointing to the root to the repo page
    if (maybeObjectType && filePath.replaceAll(/\/+$/g, '') === '') {
        return <Navigate to={toRepoURL({ repoName, revision: context.revision })} replace={true} />
    }

    const objectType = maybeObjectType || 'tree'
    const mode = getModeFromPath(filePath)

    // Redirect OpenGrok-style line number hashes (#123, #123-321) to query parameter (?L123, ?L123-321)
    const hashLineNumberMatch = location.hash.match(/^#?(\d+)(-\d+)?$/)
    if (objectType === 'blob' && hashLineNumberMatch) {
        const startLineNumber = parseInt(hashLineNumberMatch[1], 10)
        const endLineNumber = hashLineNumberMatch[2] ? parseInt(hashLineNumberMatch[2].slice(1), 10) : undefined
        const url = appendLineRangeQueryParameter(
            location.pathname + location.search,
            `L${startLineNumber}` + (endLineNumber ? `-${endLineNumber}` : '')
        )
        return <Navigate to={url} replace={true} />
    }

    // For blob pages with legacy URL fragment hashes like "#L17:19-21:23$foo:bar"
    // redirect to the modern URL fragment hashes like "#L17:19-21:23&tab=foo:bar"
    if (!hideRepoRevisionContent && objectType === 'blob' && isLegacyFragment(location.hash)) {
        const parsedQuery = parseQueryAndHash(location.search, location.hash)
        const hashParameters = new URLSearchParams()
        if (parsedQuery.viewState) {
            hashParameters.set('tab', parsedQuery.viewState)
        }
        const range = formatLineOrPositionOrRange(parsedQuery)
        const url = appendLineRangeQueryParameter(location.pathname + location.search, range ? `L${range}` : undefined)
        return <Navigate to={url + formatHash(hashParameters)} replace={true} />
    }

    return (
        <>
            <RepoRevisionSidebar
                className="repo-revision-container__sidebar"
                revision={context.revision}
                settingsCascade={context.settingsCascade}
                telemetryService={context.telemetryService}
                telemetryRecorder={platformContext.telemetryRecorder}
                authenticatedUser={context.authenticatedUser}
                isSourcegraphDotCom={context.isSourcegraphDotCom}
                commitID={resolvedRevision?.commitID}
                filePath={filePath}
                repoID={repo?.id}
                repoName={repoName}
                isDir={objectType === 'tree'}
                defaultBranch={resolvedRevision?.defaultBranch || 'HEAD'}
            />
            {!hideRepoRevisionContent && (
                <>
                    <GettingStartedTour.Info isSourcegraphDotCom={context.isSourcegraphDotCom} className="mr-3 mb-3" />
                    <ErrorBoundary location={location}>
                        {objectType === 'blob' ? (
                            <TraceSpanProvider name="BlobPage">
                                <BlobPage
                                    {...context}
                                    commitID={resolvedRevision?.commitID}
                                    filePath={filePath}
                                    repoID={repo?.id}
                                    repoName={repoName}
                                    repoUrl={repo?.url}
                                    repoServiceType={repo?.externalRepository?.serviceType}
                                    mode={mode}
                                    repoHeaderContributionsLifecycleProps={
                                        context.repoHeaderContributionsLifecycleProps
                                    }
                                    fetchHighlightedFileLineRanges={props.fetchHighlightedFileLineRanges}
                                    className={styles.pageContent}
                                    context={globalContext}
                                    platformContext={platformContext}
                                    telemetryRecorder={platformContext.telemetryRecorder}
                                />
                            </TraceSpanProvider>
                        ) : resolvedRevision ? (
                            // TODO: see if we can render without resolvedRevision.commitID
                            <TreePage
                                {...props}
                                commitID={resolvedRevision?.commitID}
                                filePath={filePath}
                                repo={repo}
                                repoName={repoName}
                                isSourcegraphDotCom={context.isSourcegraphDotCom}
                                className={styles.pageContent}
                                authenticatedUser={context.authenticatedUser}
                                context={globalContext}
                            />
                        ) : (
                            <LoadingSpinner />
                        )}
                    </ErrorBoundary>
                </>
            )}
        </>
    )
}
