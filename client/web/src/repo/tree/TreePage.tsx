import React, { type FC, useEffect, useMemo } from 'react'

import {
    mdiAccount,
    mdiBrain,
    mdiCog,
    mdiFolder,
    mdiHistory,
    mdiPackageVariantClosed,
    mdiSourceBranch,
    mdiSourceCommit,
    mdiSourceFork,
    mdiSourceRepository,
    mdiTag,
    mdiVectorPolyline,
} from '@mdi/js'
import classNames from 'classnames'
import { Navigate } from 'react-router-dom'
import { catchError } from 'rxjs/operators'

import { asError, encodeURIPathComponent, type ErrorLike, isErrorLike, basename } from '@sourcegraph/common'
import { gql, useQuery } from '@sourcegraph/http-client'
import { fetchTreeEntries } from '@sourcegraph/shared/src/backend/repo'
import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { Settings } from '@sourcegraph/shared/src/schema/settings.schema'
import type { SearchContextProps } from '@sourcegraph/shared/src/search'
import type { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { toPrettyBlobURL } from '@sourcegraph/shared/src/util/url'
import {
    Badge,
    Button,
    ButtonGroup,
    Container,
    ErrorAlert,
    Icon,
    Link,
    LoadingSpinner,
    PageHeader,
    Tooltip,
    useObservable,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import type { BatchChangesProps } from '../../batches'
import { RepoBatchChangesButton } from '../../batches/RepoBatchChangesButton'
import type { CodeIntelligenceProps } from '../../codeintel'
import { isCodyEnabled } from '../../cody/isCodyEnabled'
import type { BreadcrumbSetters } from '../../components/Breadcrumbs'
import { PageTitle } from '../../components/PageTitle'
import type { FileCommitsResult, FileCommitsVariables, RepositoryFields } from '../../graphql-operations'
import type { SourcegraphContext } from '../../jscontext'
import type { OwnConfigProps } from '../../own/OwnConfigProps'
import { TryCodyWidget } from '../components/TryCodyWidget/TryCodyWidget'
import { FilePathBreadcrumbs } from '../FilePathBreadcrumbs'
import { isPackageServiceType } from '../packages/isPackageServiceType'

import { TreePageContent } from './TreePageContent'
import { treeHistoryFragment } from './TreePagePanels'

import styles from './TreePage.module.scss'

const FILE_COMMITS_QUERY = gql`
    ${treeHistoryFragment}
    query FileCommits($repoName: String!, $revision: String!, $filePath: String!, $first: Int) {
        repository(name: $repoName) {
            id
            commit(rev: $revision) {
                id
                tree(path: $filePath) {
                    entries(first: $first) {
                        ...TreeHistoryFields
                    }
                }
            }
        }
    }
`
export interface Props
    extends SettingsCascadeProps<Settings>,
        PlatformContextProps,
        TelemetryProps,
        CodeIntelligenceProps,
        BatchChangesProps,
        Pick<SearchContextProps, 'selectedSearchContextSpec'>,
        BreadcrumbSetters,
        OwnConfigProps {
    repo: RepositoryFields | undefined
    repoName: string
    /** The tree's path in TreePage. We call it filePath for consistency elsewhere. */
    filePath: string
    commitID: string
    revision: string
    isSourcegraphDotCom: boolean
    className?: string
    authenticatedUser: AuthenticatedUser | null
    context: Pick<SourcegraphContext, 'externalURL'>
}

export const treePageRepositoryFragment = gql`
    fragment TreePageRepositoryFields on Repository {
        id
        name
        description
        viewerCanAdminister
        url
        metadata {
            key
            value
        }
        topics
        sourceType
    }
`

export const TreePage: FC<Props> = ({
    repo,
    repoName,
    commitID,
    revision,
    filePath,
    settingsCascade,
    useBreadcrumb,
    codeIntelligenceEnabled,
    batchChangesEnabled,
    isSourcegraphDotCom,
    authenticatedUser,
    ownEnabled,
    className,
    context,
    ...props
}) => {
    const isRoot = filePath === ''
    const isPackage = useMemo(
        () => isPackageServiceType(repo?.externalRepository.serviceType),
        [repo?.externalRepository.serviceType]
    )
    useEffect(() => {
        if (isRoot) {
            props.telemetryService.logViewEvent('Repository')
        } else {
            props.telemetryService.logViewEvent('Tree')
        }
    }, [isRoot, props.telemetryService])

    useBreadcrumb(
        useMemo(() => {
            if (isRoot) {
                return
            }

            return {
                key: 'treePath',
                className: 'flex-shrink-past-contents flex-grow-1',
                element: (
                    <FilePathBreadcrumbs
                        key="path"
                        repoName={repoName}
                        revision={revision}
                        filePath={filePath}
                        isDir={true}
                        telemetryService={props.telemetryService}
                        // TODO (dadlerj): update to use a real telemetry recorder
                        telemetryRecorder={noOpTelemetryRecorder}
                    />
                ),
            }
        }, [isRoot, filePath, repoName, revision, props.telemetryService])
    )

    const treeOrError = useObservable(
        useMemo(
            () =>
                fetchTreeEntries({
                    repoName,
                    commitID,
                    revision,
                    filePath,
                    first: 2500,
                    requestGraphQL: props.platformContext.requestGraphQL,
                }).pipe(catchError((error): [ErrorLike] => [asError(error)])),
            [repoName, commitID, revision, filePath, props.platformContext]
        )
    )

    const { data: fileCommitData } = useQuery<FileCommitsResult, FileCommitsVariables>(FILE_COMMITS_QUERY, {
        variables: {
            repoName,
            revision,
            filePath,
            first: 2500,
        },
    })
    const treeWithHistory = fileCommitData?.repository?.commit?.tree?.entries

    const showOwnership = ownEnabled && !isSourcegraphDotCom

    const getPageTitle = (): string => {
        const repoString = displayRepoName(repoName)
        if (filePath) {
            return `${basename(filePath)} - ${repoString}`
        }
        return `${repoString}`
    }

    const getIcon = (): string => {
        if (isPackage) {
            return mdiPackageVariantClosed
        }
        if (repo?.isFork) {
            return mdiSourceFork
        }
        return mdiSourceRepository
    }

    const RootHeaderSection = (): React.ReactElement => (
        <div className="d-flex flex-wrap justify-content-between px-0">
            <div className={styles.header}>
                <PageHeader className="mb-3 test-tree-page-title">
                    <PageHeader.Heading as="h2" styleAs="h1">
                        <Icon aria-hidden={true} svgPath={getIcon()} className="mr-2" />
                        <span data-testid="repo-header">{displayRepoName(repo?.name || '')}</span>
                        {repo?.isFork && (
                            <Badge variant="outlineSecondary" className="mx-2 mt-1" data-testid="repo-fork-badge">
                                Fork
                            </Badge>
                        )}
                    </PageHeader.Heading>
                </PageHeader>
            </div>
            <div className={styles.menu}>
                <ButtonGroup>
                    <Tooltip content="Git commits">
                        <Button
                            className="flex-shrink-0"
                            to={`/${encodeURIPathComponent(repoName)}/-/commits`}
                            variant="secondary"
                            outline={true}
                            as={Link}
                        >
                            <Icon aria-hidden={true} svgPath={mdiSourceCommit} />{' '}
                            <span className={styles.text}>Commits</span>
                        </Button>
                    </Tooltip>
                    {!isPackage && (
                        <Tooltip content="Git branches">
                            <Button
                                className="flex-shrink-0"
                                to={`/${encodeURIPathComponent(repoName)}/-/branches`}
                                variant="secondary"
                                outline={true}
                                as={Link}
                            >
                                <Icon aria-hidden={true} svgPath={mdiSourceBranch} />{' '}
                                <span className={styles.text}>Branches</span>
                            </Button>
                        </Tooltip>
                    )}
                    <Tooltip content={isPackage ? 'Package versions' : 'Git tags'}>
                        <Button
                            className="flex-shrink-0"
                            to={`/${encodeURIPathComponent(repoName)}/-${isPackage ? '/versions' : '/tags'}`}
                            variant="secondary"
                            outline={true}
                            as={Link}
                        >
                            <Icon aria-hidden={true} svgPath={mdiTag} />{' '}
                            <span className={styles.text}>{isPackage ? 'Versions' : 'Tags'}</span>
                        </Button>
                    </Tooltip>
                    <Tooltip content="Compare branches">
                        <Button
                            className="flex-shrink-0"
                            to={
                                revision
                                    ? `/${encodeURIPathComponent(repoName)}/-/compare/...${encodeURIComponent(
                                          revision
                                      )}`
                                    : `/${encodeURIPathComponent(repoName)}/-/compare`
                            }
                            variant="secondary"
                            outline={true}
                            as={Link}
                        >
                            <Icon aria-hidden={true} svgPath={mdiHistory} />{' '}
                            <span className={styles.text}>Compare</span>
                        </Button>
                    </Tooltip>
                    {/** the code graph dashboard is only accessible to site admins */}
                    {codeIntelligenceEnabled && authenticatedUser?.siteAdmin && (
                        <Tooltip content="Code graph data">
                            <Button
                                className="flex-shrink-0"
                                to={`/${encodeURIPathComponent(repoName)}/-/code-graph`}
                                variant="secondary"
                                outline={true}
                                as={Link}
                            >
                                <Icon aria-hidden={true} svgPath={mdiBrain} />{' '}
                                <span className={styles.text}>Code graph data</span>
                            </Button>
                        </Tooltip>
                    )}
                    {window.context?.codyEnabled && window.context?.embeddingsEnabled && (
                        <Tooltip content="Embeddings">
                            <Button
                                className="flex-shrink-0"
                                to={`/${encodeURIPathComponent(repoName)}/-/embeddings`}
                                variant="secondary"
                                outline={true}
                                as={Link}
                            >
                                <Icon aria-hidden={true} svgPath={mdiVectorPolyline} />{' '}
                                <span className={styles.text}>Embeddings</span>
                            </Button>
                        </Tooltip>
                    )}
                    {batchChangesEnabled && !isPackage && (
                        <Tooltip content="Batch changes">
                            <RepoBatchChangesButton
                                className="flex-shrink-0"
                                textClassName={styles.text}
                                repoName={repoName}
                            />
                        </Tooltip>
                    )}
                    {showOwnership && (
                        <Tooltip content="Repository ownership settings">
                            <Button
                                className="flex-shrink-0"
                                to={`/${encodeURIPathComponent(repoName)}/-/own`}
                                variant="secondary"
                                outline={true}
                                as={Link}
                                onClick={() => props.telemetryService.log('repoPage:ownershipPage:clicked')}
                            >
                                <Icon aria-hidden={true} svgPath={mdiAccount} />{' '}
                                <span className={styles.text}>Ownership</span>
                            </Button>
                        </Tooltip>
                    )}
                    {repo?.viewerCanAdminister && (
                        <Tooltip content="Repository settings">
                            <Button
                                className="flex-shrink-0"
                                to={`/${encodeURIPathComponent(repoName)}/-/settings`}
                                variant="secondary"
                                outline={true}
                                as={Link}
                                aria-label="Repository settings"
                            >
                                <Icon aria-hidden={true} svgPath={mdiCog} />{' '}
                                <span className={styles.text}>Settings</span>
                            </Button>
                        </Tooltip>
                    )}
                </ButtonGroup>
            </div>
        </div>
    )

    return (
        <div className={classNames(styles.treePage, className)}>
            {(isSourcegraphDotCom || isCodyEnabled()) && (
                <TryCodyWidget
                    className="mb-2"
                    telemetryService={props.telemetryService}
                    // TODO (dadlerj): update to use a real telemetry recorder
                    telemetryRecorder={noOpTelemetryRecorder}
                    type="repo"
                    authenticatedUser={authenticatedUser}
                    context={context}
                    isSourcegraphDotCom={isSourcegraphDotCom}
                />
            )}
            <Container className={styles.container}>
                <div className={classNames(styles.header)}>
                    <PageTitle title={getPageTitle()} />

                    <header className="mb-3">
                        {isRoot ? (
                            <RootHeaderSection />
                        ) : (
                            <PageHeader className="mb-3 mr-2 test-tree-page-title">
                                <PageHeader.Heading as="h2" styleAs="h1">
                                    <PageHeader.Breadcrumb icon={mdiFolder}>{filePath}</PageHeader.Breadcrumb>
                                </PageHeader.Heading>
                            </PageHeader>
                        )}
                    </header>

                    {treeOrError === undefined || repo === undefined ? (
                        <div>
                            <LoadingSpinner /> Loading files and directories
                        </div>
                    ) : isErrorLike(treeOrError) ? (
                        // If the tree is actually a blob, be helpful and redirect to the blob page.
                        // We don't have error names on GraphQL errors.
                        /not a directory/i.test(treeOrError.message) ? (
                            <Navigate to={toPrettyBlobURL({ repoName, revision, commitID, filePath })} replace={true} />
                        ) : (
                            <ErrorAlert error={treeOrError} />
                        )
                    ) : (
                        <TreePageContent
                            filePath={filePath}
                            tree={treeOrError}
                            treeWithHistory={treeWithHistory}
                            repo={repo}
                            revision={revision}
                            commitID={commitID}
                            isPackage={isPackage}
                            authenticatedUser={authenticatedUser}
                            showOwnership={showOwnership}
                            {...props}
                        />
                    )}
                </div>
            </Container>
        </div>
    )
}
