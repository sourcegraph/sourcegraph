import React, { useMemo, useEffect, FC } from 'react'

import {
    mdiAccount,
    mdiBrain,
    mdiCog,
    mdiFolder,
    mdiHistory,
    mdiSourceBranch,
    mdiSourceRepository,
    mdiTag,
} from '@mdi/js'
import classNames from 'classnames'
import { Navigate } from 'react-router-dom'
import { catchError } from 'rxjs/operators'

import { asError, encodeURIPathComponent, ErrorLike, isErrorLike } from '@sourcegraph/common'
import { gql } from '@sourcegraph/http-client'
import { fetchTreeEntries } from '@sourcegraph/shared/src/backend/repo'
import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { Settings } from '@sourcegraph/shared/src/schema/settings.schema'
import { SearchContextProps } from '@sourcegraph/shared/src/search'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { toURIWithPath, toPrettyBlobURL } from '@sourcegraph/shared/src/util/url'
import {
    Container,
    PageHeader,
    LoadingSpinner,
    useObservable,
    Link,
    Icon,
    ButtonGroup,
    Button,
    Text,
    ErrorAlert,
    Tooltip,
} from '@sourcegraph/wildcard'

import { BatchChangesProps } from '../../batches'
import { RepoBatchChangesButton } from '../../batches/RepoBatchChangesButton'
import { CodeIntelligenceProps } from '../../codeintel'
import { BreadcrumbSetters } from '../../components/Breadcrumbs'
import { PageTitle } from '../../components/PageTitle'
import { useFeatureFlag } from '../../featureFlags/useFeatureFlag'
import { RepositoryFields } from '../../graphql-operations'
import { basename } from '../../util/path'
import { FilePathBreadcrumbs } from '../FilePathBreadcrumbs'

import { TreePageContent } from './TreePageContent'

import styles from './TreePage.module.scss'

interface Props
    extends SettingsCascadeProps<Settings>,
        PlatformContextProps,
        TelemetryProps,
        CodeIntelligenceProps,
        BatchChangesProps,
        Pick<SearchContextProps, 'selectedSearchContextSpec'>,
        BreadcrumbSetters {
    repo: RepositoryFields | undefined
    repoName: string
    /** The tree's path in TreePage. We call it filePath for consistency elsewhere. */
    filePath: string
    commitID: string
    revision: string
    globbing: boolean
    isSourcegraphDotCom: boolean
    className?: string
}

export const treePageRepositoryFragment = gql`
    fragment TreePageRepositoryFields on Repository {
        id
        name
        description
        viewerCanAdminister
        url
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
    className,
    ...props
}) => {
    const isRoot = filePath === ''

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
                className: 'flex-shrink-past-contents',
                element: (
                    <FilePathBreadcrumbs
                        key="path"
                        repoName={repoName}
                        revision={revision}
                        filePath={filePath}
                        isDir={true}
                        telemetryService={props.telemetryService}
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

    const showCodeInsights =
        !isErrorLike(settingsCascade.final) &&
        !!settingsCascade.final?.experimentalFeatures?.codeInsights &&
        settingsCascade.final['insights.displayLocation.directory'] === true

    const [ownEnabled] = useFeatureFlag('search-ownership')

    // Add DirectoryViewer
    const uri = toURIWithPath({ repoName, commitID, filePath })

    const getPageTitle = (): string => {
        const repoString = displayRepoName(repoName)
        if (filePath) {
            return `${basename(filePath)} - ${repoString}`
        }
        return `${repoString}`
    }

    const RootHeaderSection = (): React.ReactElement => (
        <div className="d-flex flex-wrap justify-content-between px-0">
            <div className={styles.header}>
                <PageHeader className="mb-3 test-tree-page-title">
                    <PageHeader.Heading as="h2" styleAs="h1">
                        <PageHeader.Breadcrumb icon={mdiSourceRepository}>
                            {displayRepoName(repo?.name || '')}
                        </PageHeader.Breadcrumb>
                    </PageHeader.Heading>
                </PageHeader>
                {repo?.description && <Text>{repo.description}</Text>}
            </div>
            <div className={styles.menu}>
                <ButtonGroup>
                    <Tooltip content="Branches">
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
                    <Tooltip content="Tags">
                        <Button
                            className="flex-shrink-0"
                            to={`/${encodeURIPathComponent(repoName)}/-/tags`}
                            variant="secondary"
                            outline={true}
                            as={Link}
                        >
                            <Icon aria-hidden={true} svgPath={mdiTag} /> <span className={styles.text}>Tags</span>
                        </Button>
                    </Tooltip>
                    <Tooltip content="Compare">
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
                    {codeIntelligenceEnabled && (
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
                    {batchChangesEnabled && (
                        <Tooltip content="Batch changes">
                            <RepoBatchChangesButton
                                className="flex-shrink-0"
                                textClassName={styles.text}
                                repoName={repoName}
                            />
                        </Tooltip>
                    )}
                    {ownEnabled && (
                        <Tooltip content="Ownership">
                            <Button
                                className="flex-shrink-0"
                                to={`/${encodeURIPathComponent(repoName)}/-/own`}
                                variant="secondary"
                                outline={true}
                                as={Link}
                            >
                                <Icon aria-hidden={true} svgPath={mdiAccount} />{' '}
                                <span className={styles.text}>Ownership</span>
                            </Button>
                        </Tooltip>
                    )}
                    {repo?.viewerCanAdminister && (
                        <Tooltip content="Settings">
                            <Button
                                className="flex-shrink-0"
                                to={`/${encodeURIPathComponent(repoName)}/-/settings`}
                                variant="secondary"
                                outline={true}
                                as={Link}
                                aria-label="Repository settings"
                            >
                                <Icon aria-hidden={true} svgPath={mdiCog} />
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
                            repo={repo}
                            revision={revision}
                            commitID={commitID}
                            {...props}
                        />
                    )}
                </div>
            </Container>
        </div>
    )
}
