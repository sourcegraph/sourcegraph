import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import * as H from 'history'
import FolderIcon from 'mdi-react/FolderIcon'
import HistoryIcon from 'mdi-react/HistoryIcon'
import SourceBranchIcon from 'mdi-react/SourceBranchIcon'
import SourceCommitIcon from 'mdi-react/SourceCommitIcon'
import SourceRepositoryIcon from 'mdi-react/SourceRepositoryIcon'
import TagIcon from 'mdi-react/TagIcon'
import UserIcon from 'mdi-react/UserIcon'
import React, { useState, useMemo, useCallback, useEffect } from 'react'
import { Link, Redirect } from 'react-router-dom'
import { Observable, EMPTY } from 'rxjs'
import { catchError, map } from 'rxjs/operators'
import { ActionItem } from '../../../../shared/src/actions/ActionItem'
import { ActionsContainer } from '../../../../shared/src/actions/ActionsContainer'
import { ContributableMenu, ContributableViewContainer } from '../../../../shared/src/api/protocol'
import { ActivationProps } from '../../../../shared/src/components/activation/Activation'
import { displayRepoName } from '../../../../shared/src/components/RepoFileLink'
import { ExtensionsControllerProps } from '../../../../shared/src/extensions/controller'
import { gql, dataOrThrowErrors } from '../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../shared/src/graphql/schema'
import { PlatformContextProps } from '../../../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../../../shared/src/settings/settings'
import { asError, ErrorLike, isErrorLike } from '../../../../shared/src/util/errors'
import { memoizeObservable } from '../../../../shared/src/util/memoizeObservable'
import { queryGraphQL } from '../../backend/graphql'
import { FilteredConnection } from '../../components/FilteredConnection'
import { PageTitle } from '../../components/PageTitle'
import { PatternTypeProps, CaseSensitivityProps, CopyQueryButtonProps, SearchContextProps } from '../../search'
import { basename } from '../../util/path'
import { fetchTreeEntries } from '../backend'
import { GitCommitNode, GitCommitNodeProps } from '../commits/GitCommitNode'
import { gitCommitFragment } from '../commits/RepositoryCommitsPage'
import { ThemeProps } from '../../../../shared/src/theme'
import { ErrorAlert } from '../../components/alerts'
import { subYears, formatISO } from 'date-fns'
import { pluralize } from '../../../../shared/src/util/strings'
import { useObservable } from '../../../../shared/src/util/useObservable'
import { encodeURIPathComponent, toPrettyBlobURL, toURIWithPath } from '../../../../shared/src/util/url'
import { Settings } from '../../schema/settings.schema'
import { ViewGrid } from './ViewGrid'
import { VersionContextProps } from '../../../../shared/src/search/util'
import { BreadcrumbSetters } from '../../components/Breadcrumbs'
import { FilePathBreadcrumbs } from '../FilePathBreadcrumbs'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { TreeEntriesSection } from './TreeEntriesSection'
import { GitCommitFields, Scalars, TreePageRepositoryFields } from '../../graphql-operations'
import { getFileDecorations } from '../../backend/features'
import { FileDecorationsByPath } from '../../../../shared/src/api/extension/flatExtensionApi'
import SettingsIcon from 'mdi-react/SettingsIcon'
import { getCombinedViews } from '../../insights/backend'

const fetchTreeCommits = memoizeObservable(
    (args: {
        repo: Scalars['ID']
        revspec: string
        first?: number
        filePath?: string
        after?: string
    }): Observable<GQL.IGitCommitConnection> =>
        queryGraphQL(
            gql`
                query TreeCommits($repo: ID!, $revspec: String!, $first: Int, $filePath: String, $after: String) {
                    node(id: $repo) {
                        __typename
                        ... on Repository {
                            commit(rev: $revspec) {
                                ancestors(first: $first, path: $filePath, after: $after) {
                                    nodes {
                                        ...GitCommitFields
                                    }
                                    pageInfo {
                                        hasNextPage
                                    }
                                }
                            }
                        }
                    }
                }
                ${gitCommitFragment}
            `,
            args
        ).pipe(
            map(dataOrThrowErrors),
            map(data => {
                if (!data.node) {
                    throw new Error('Repository not found')
                }
                if (data.node.__typename !== 'Repository') {
                    throw new Error('Node is not a Repository')
                }
                if (!data.node.commit) {
                    throw new Error('Commit not found')
                }
                return data.node.commit.ancestors
            })
        ),
    args => `${args.repo}:${args.revspec}:${String(args.first)}:${String(args.filePath)}:${String(args.after)}`
)

interface Props
    extends SettingsCascadeProps<Settings>,
        ExtensionsControllerProps,
        PlatformContextProps,
        ThemeProps,
        TelemetryProps,
        ActivationProps,
        PatternTypeProps,
        CaseSensitivityProps,
        CopyQueryButtonProps,
        VersionContextProps,
        Pick<SearchContextProps, 'selectedSearchContextSpec'>,
        BreadcrumbSetters {
    repo: TreePageRepositoryFields
    /** The tree's path in TreePage. We call it filePath for consistency elsewhere. */
    filePath: string
    commitID: string
    revision: string
    location: H.Location
    history: H.History
    globbing: boolean
}

export const treePageRepositoryFragment = gql`
    fragment TreePageRepositoryFields on Repository {
        id
        name
        description
        viewerCanAdminister
    }
`

export const TreePage: React.FunctionComponent<Props> = ({
    repo,
    commitID,
    revision,
    filePath,
    patternType,
    caseSensitive,
    settingsCascade,
    useBreadcrumb,
    ...props
}) => {
    useEffect(() => {
        if (filePath === '') {
            props.telemetryService.logViewEvent('Repository')
        } else {
            props.telemetryService.logViewEvent('Tree')
        }
    }, [filePath, props.telemetryService])

    useBreadcrumb(
        useMemo(() => {
            if (!filePath) {
                return
            }
            return {
                key: 'treePath',
                className: 'flex-shrink-past-contents',
                element: (
                    <FilePathBreadcrumbs
                        key="path"
                        repoName={repo.name}
                        revision={revision}
                        filePath={filePath}
                        isDir={true}
                    />
                ),
            }
        }, [repo.name, revision, filePath])
    )

    const [showOlderCommits, setShowOlderCommits] = useState(false)

    const onShowOlderCommitsClicked = useCallback(
        (event: React.MouseEvent): void => {
            event.preventDefault()
            setShowOlderCommits(true)
        },
        [setShowOlderCommits]
    )

    const treeOrError = useObservable(
        useMemo(
            () =>
                fetchTreeEntries({
                    repoName: repo.name,
                    commitID,
                    revision,
                    filePath,
                    first: 2500,
                }).pipe(catchError((error): [ErrorLike] => [asError(error)])),
            [repo.name, commitID, revision, filePath]
        )
    )

    const fileDecorationsByPath =
        useObservable<FileDecorationsByPath>(
            useMemo(
                () =>
                    treeOrError && !isErrorLike(treeOrError)
                        ? getFileDecorations({
                              files: treeOrError.entries,
                              extensionsController: props.extensionsController,
                              repoName: repo.name,
                              commitID,
                              parentNodeUri: treeOrError.url,
                          })
                        : EMPTY,
                [treeOrError, repo.name, commitID, props.extensionsController]
            )
        ) ?? {}

    const { services } = props.extensionsController

    const showCodeInsights =
        !isErrorLike(settingsCascade.final) &&
        !!settingsCascade.final?.experimentalFeatures?.codeInsights &&
        settingsCascade.final['insights.displayLocation.directory'] !== false

    // Add DirectoryViewer
    const uri = toURIWithPath({ repoName: repo.name, commitID, filePath })
    useEffect(() => {
        if (!showCodeInsights) {
            return
        }
        const viewerId = services.viewer.addViewer({
            type: 'DirectoryViewer',
            isActive: true,
            resource: uri,
        })
        return () => services.viewer.removeViewer(viewerId)
    }, [services.viewer, services.model, uri, showCodeInsights])

    // Observe directory views
    const workspaceUri = services.workspace.roots.value[0]?.uri
    const views = useObservable(
        useMemo(
            () =>
                showCodeInsights && workspaceUri
                    ? getCombinedViews(
                          ContributableViewContainer.Directory,
                          {
                              viewer: {
                                  type: 'DirectoryViewer',
                                  directory: {
                                      uri,
                                  },
                              },
                              workspace: {
                                  uri: workspaceUri,
                              },
                          },
                          services.view
                      )
                    : EMPTY,
            [showCodeInsights, workspaceUri, uri, services.view]
        )
    )

    const getPageTitle = (): string => {
        const repoString = displayRepoName(repo.name)
        if (filePath) {
            return `${basename(filePath)} - ${repoString}`
        }
        return `${repoString}`
    }

    const queryCommits = useCallback(
        (args: { first?: number }): Observable<GQL.IGitCommitConnection> => {
            const after: string | undefined = showOlderCommits ? undefined : formatISO(subYears(Date.now(), 1))
            return fetchTreeCommits({
                ...args,
                repo: repo.id,
                revspec: revision || '',
                filePath,
                after,
            })
        },
        [filePath, repo.id, revision, showOlderCommits]
    )

    const emptyElement = showOlderCommits ? (
        <>No commits in this tree.</>
    ) : (
        <div className="test-tree-page-no-recent-commits">
            No commits in this tree in the past year.
            <br />
            <button
                type="button"
                className="btn btn-secondary btn-sm test-tree-page-show-all-commits"
                onClick={onShowOlderCommitsClicked}
            >
                Show all commits
            </button>
        </div>
    )

    const TotalCountSummary: React.FunctionComponent<{ totalCount: number }> = ({ totalCount }) => (
        <div className="mt-2">
            {showOlderCommits ? (
                <>{totalCount} total commits in this tree.</>
            ) : (
                <>
                    {totalCount} {pluralize('commit', totalCount)} in this tree in the past year.
                    <br />
                    <button type="button" className="btn btn-secondary btn-sm mt-1" onClick={onShowOlderCommitsClicked}>
                        Show all commits
                    </button>
                </>
            )}
        </div>
    )
    return (
        <div className="tree-page">
            <PageTitle title={getPageTitle()} />
            {treeOrError === undefined ? (
                <div>
                    <LoadingSpinner className="icon-inline tree-page__entries-loader" /> Loading files and directories
                </div>
            ) : isErrorLike(treeOrError) ? (
                // If the tree is actually a blob, be helpful and redirect to the blob page.
                // We don't have error names on GraphQL errors.
                /not a directory/i.test(treeOrError.message) ? (
                    <Redirect to={toPrettyBlobURL({ repoName: repo.name, revision, commitID, filePath })} />
                ) : (
                    <ErrorAlert error={treeOrError} />
                )
            ) : (
                <>
                    <header className="mb-3">
                        {treeOrError.isRoot ? (
                            <>
                                <h2 className="tree-page__title">
                                    <SourceRepositoryIcon className="icon-inline" /> {displayRepoName(repo.name)}
                                </h2>
                                {repo.description && <p>{repo.description}</p>}
                                <div className="btn-group mb-3">
                                    <Link className="btn btn-secondary" to={`${treeOrError.url}/-/commits`}>
                                        <SourceCommitIcon className="icon-inline" /> Commits
                                    </Link>
                                    <Link
                                        className="btn btn-secondary"
                                        to={`/${encodeURIPathComponent(repo.name)}/-/branches`}
                                    >
                                        <SourceBranchIcon className="icon-inline" /> Branches
                                    </Link>
                                    <Link
                                        className="btn btn-secondary"
                                        to={`/${encodeURIPathComponent(repo.name)}/-/tags`}
                                    >
                                        <TagIcon className="icon-inline" /> Tags
                                    </Link>
                                    <Link
                                        className="btn btn-secondary"
                                        to={
                                            revision
                                                ? `/${encodeURIPathComponent(
                                                      repo.name
                                                  )}/-/compare/...${encodeURIComponent(revision)}`
                                                : `/${encodeURIPathComponent(repo.name)}/-/compare`
                                        }
                                    >
                                        <HistoryIcon className="icon-inline" /> Compare
                                    </Link>
                                    <Link
                                        className="btn btn-secondary"
                                        to={`/${encodeURIPathComponent(repo.name)}/-/stats/contributors`}
                                    >
                                        <UserIcon className="icon-inline" /> Contributors
                                    </Link>
                                    {repo.viewerCanAdminister && (
                                        <Link
                                            className="btn btn-secondary"
                                            to={`/${encodeURIPathComponent(repo.name)}/-/settings`}
                                        >
                                            <SettingsIcon className="icon-inline" /> Settings
                                        </Link>
                                    )}
                                </div>
                            </>
                        ) : (
                            <h2 className="tree-page__title">
                                <FolderIcon className="icon-inline" /> {filePath}
                            </h2>
                        )}
                    </header>
                    {views && (
                        <ViewGrid
                            {...props}
                            className="tree-page__section"
                            views={views}
                            patternType={patternType}
                            settingsCascade={settingsCascade}
                            caseSensitive={caseSensitive}
                        />
                    )}
                    <section className="tree-page__section test-tree-entries">
                        <h3 className="tree-page__section-header">Files and directories</h3>
                        <TreeEntriesSection
                            parentPath={filePath}
                            entries={treeOrError.entries}
                            fileDecorationsByPath={fileDecorationsByPath}
                            isLightTheme={props.isLightTheme}
                        />
                    </section>
                    {/* eslint-disable react/jsx-no-bind */}
                    <ActionsContainer
                        {...props}
                        menu={ContributableMenu.DirectoryPage}
                        render={items => (
                            <section className="tree-page__section">
                                <h3 className="tree-page__section-header">Actions</h3>
                                {items.map(item => (
                                    <ActionItem
                                        {...props}
                                        key={item.action.id}
                                        {...item}
                                        className="btn btn-secondary mr-1 mb-1"
                                    />
                                ))}
                            </section>
                        )}
                        empty={null}
                    />
                    {/* eslint-enable react/jsx-no-bind */}
                    <div className="tree-page__section">
                        <h3 className="tree-page__section-header">Changes</h3>
                        <FilteredConnection<GitCommitFields, Pick<GitCommitNodeProps, 'className' | 'compact'>>
                            location={props.location}
                            className="mt-2 tree-page__section--commits"
                            listClassName="list-group list-group-flush"
                            noun="commit in this tree"
                            pluralNoun="commits in this tree"
                            queryConnection={queryCommits}
                            nodeComponent={GitCommitNode}
                            nodeComponentProps={{
                                className: 'list-group-item',
                                compact: true,
                            }}
                            updateOnChange={`${repo.name}:${revision}:${filePath}:${String(showOlderCommits)}`}
                            defaultFirst={7}
                            useURLQuery={false}
                            hideSearch={true}
                            emptyElement={emptyElement}
                            // eslint-disable-next-line react/jsx-no-bind
                            totalCountSummaryComponent={TotalCountSummary}
                        />
                    </div>
                </>
            )}
        </div>
    )
}
