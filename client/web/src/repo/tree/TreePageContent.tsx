import React, { useMemo } from 'react'

import { mdiCog, mdiFileOutline, mdiSourceCommit, mdiGlasses, mdiInformationOutline } from '@mdi/js'
import classNames from 'classnames'
import { escapeRegExp } from 'lodash'

import { metadataToTag, TagList, topicToTag } from '@sourcegraph/branded'
import { encodeURIPathComponent, numberWithCommas, pluralize } from '@sourcegraph/common'
import { gql, useQuery } from '@sourcegraph/http-client'
import { TeamAvatar } from '@sourcegraph/shared/src/components/TeamAvatar'
import { UserAvatar } from '@sourcegraph/shared/src/components/UserAvatar'
import { SearchPatternType, type TreeFields } from '@sourcegraph/shared/src/graphql-operations'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import { Badge, Button, ButtonLink, Card, CardHeader, Icon, Link, Text, Tooltip } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import {
    ConnectionContainer,
    ConnectionError,
    ConnectionList,
    ConnectionLoading,
    ConnectionSummary,
    SummaryContainer,
} from '../../components/FilteredConnection/ui'
import { useFeatureFlag } from '../../featureFlags/useFeatureFlag'
import type {
    TreeHistoryFields,
    TreePageOwnershipNodeFields,
    TreePageOwnershipResult,
    TreePageOwnershipVariables,
    TreePageRepositoryContributorNodeFields,
    TreePageRepositoryContributorsResult,
    TreePageRepositoryContributorsVariables,
    TreePageRepositoryFields,
} from '../../graphql-operations'
import { PersonLink } from '../../person/PersonLink'
import { quoteIfNeeded, searchQueryForRepoRevision } from '../../search'
import { buildSearchURLQueryFromQueryState, useNavbarQueryState } from '../../stores'
import { canWriteRepoMetadata } from '../../util/rbac'
import { OWNER_FIELDS, RECENT_CONTRIBUTOR_FIELDS, RECENT_VIEW_FIELDS } from '../blob/own/grapqlQueries'
import { getRefType } from '../utils'

import { FilesCard, ReadmePreviewCard } from './TreePagePanels'

import menuStyles from './TreePage.module.scss'
import styles from './TreePageContent.module.scss'
import contributorsStyles from './TreePageContentContributors.module.scss'
import panelStyles from './TreePagePanels.module.scss'

const COUNT = 20

const ExtraInfoSectionItem: React.FunctionComponent<React.PropsWithChildren<{}>> = ({ children }) => (
    <div className={styles.extraInfoSectionItem}>{children}</div>
)

const ExtraInfoSectionItemHeader: React.FunctionComponent<
    React.PropsWithChildren<{ title: string; tooltip?: React.ReactNode }>
> = ({ title, tooltip, children }) => (
    <div className="d-flex align-items-center justify-content-between mb-2">
        <div className="d-flex align-items-center">
            <Text className="mr-1 mb-0" weight="bold">
                {title}
            </Text>
            <Tooltip content={tooltip}>
                <Icon
                    svgPath={mdiInformationOutline}
                    aria-label={title}
                    className={classNames('text-muted', styles.extraInfoSectionItemHeaderIcon)}
                />
            </Tooltip>
        </div>
        {children}
    </div>
)

const ExtraInfoSection: React.FC<{
    repo: TreePageRepositoryFields
    className?: string
    hasWritePermissions?: boolean
}> = ({ repo, className, hasWritePermissions }) => {
    const [enableRepositoryMetadata] = useFeatureFlag('repository-metadata', true)

    const queryState = useNavbarQueryState(state => state.queryState)

    const metadataTags = useMemo(
        () => repo.metadata.map(item => metadataToTag(item, queryState, true, buildSearchURLQueryFromQueryState)),
        [repo.metadata, queryState]
    )

    const topicTags = useMemo(
        () => repo.topics.map(topic => topicToTag(topic, queryState, true, buildSearchURLQueryFromQueryState)),
        [repo.topics, queryState]
    )

    return (
        <Card className={className}>
            <ExtraInfoSectionItem>
                <ExtraInfoSectionItemHeader title="Description" tooltip="Synchronized from the code host" />
                {repo.description && <Text>{repo.description}</Text>}
            </ExtraInfoSectionItem>
            {/* Not all code hosts support the concept of "topics", hence we only show topics if we have them */}
            {topicTags.length > 0 && (
                <ExtraInfoSectionItem>
                    <ExtraInfoSectionItemHeader title="Topics" tooltip={<>Topics synced from the code host</>} />
                    <TagList tags={topicTags} />
                </ExtraInfoSectionItem>
            )}
            {enableRepositoryMetadata && (
                <ExtraInfoSectionItem>
                    <ExtraInfoSectionItemHeader
                        title="Metadata"
                        tooltip={
                            <>
                                Repository metadata allows you to search, filter and navigate between repositories.
                                Users with the Repository metadata write role can add repository metadata via the web,
                                cli or API. Learn more about{' '}
                                <Link to="/help/admin/repo/metadata" className={styles.linkDark}>
                                    Repository Metadata
                                </Link>
                                .
                            </>
                        }
                    >
                        {hasWritePermissions && (
                            <Tooltip content="Edit repository metadata">
                                <ButtonLink
                                    to={`/${encodeURIPathComponent(repo.name)}/-/metadata`}
                                    className={classNames('p-0', styles.extraInfoSectionItemHeaderIcon)}
                                >
                                    <Icon
                                        svgPath={mdiCog}
                                        aria-label="Edit repository metadata"
                                        className="text-muted"
                                    />
                                </ButtonLink>
                            </Tooltip>
                        )}
                    </ExtraInfoSectionItemHeader>
                    {metadataTags.length ? <TagList tags={metadataTags} /> : <Text className="text-muted">None</Text>}
                </ExtraInfoSectionItem>
            )}
        </Card>
    )
}

interface TreePageContentProps extends TelemetryProps, PlatformContextProps {
    filePath: string
    tree: TreeFields
    treeWithHistory?: TreeHistoryFields[]
    repo: TreePageRepositoryFields
    commitID: string
    revision: string
    isPackage: boolean
    authenticatedUser: AuthenticatedUser | null
    showOwnership: boolean
}

export const TreePageContent: React.FunctionComponent<React.PropsWithChildren<TreePageContentProps>> = props => {
    const { filePath, tree, treeWithHistory, repo, revision, isPackage, showOwnership } = props

    const isRoot = filePath === ''

    const readmeEntry = useMemo(() => {
        for (const entry of tree.entries) {
            const name = entry.name.toLocaleLowerCase()
            if (!entry.isDirectory && (name === 'readme.md' || name === 'readme' || name === 'readme.txt')) {
                return entry
            }
        }
        return null
    }, [tree.entries])

    const [enableOwnershipPanels] = useFeatureFlag('enable-ownership-panels', true)
    const hasRepoMetaWritePermissions = canWriteRepoMetadata(props.authenticatedUser)

    return (
        <>
            {!isRoot && (
                <div className={menuStyles.menu}>
                    <Tooltip content="Git commits">
                        <Button
                            className="flex-shrink-0"
                            to={`/${encodeURIPathComponent(repo.name)}${
                                revision && `@${encodeURIPathComponent(revision)}`
                            }/-/commits/${encodeURIPathComponent(filePath)}`}
                            variant="secondary"
                            outline={true}
                            as={Link}
                        >
                            <Icon aria-hidden={true} svgPath={mdiSourceCommit} />{' '}
                            <span className={menuStyles.text}>Commits</span>
                        </Button>
                    </Tooltip>
                </div>
            )}

            {(readmeEntry || isRoot) && (
                <section className={classNames('container mb-3 px-0', styles.section)}>
                    {readmeEntry && (
                        <ReadmePreviewCard
                            entry={readmeEntry}
                            repoName={repo.name}
                            revision={revision}
                            className={styles.files}
                        />
                    )}
                    {isRoot && (
                        <ExtraInfoSection
                            repo={repo}
                            className={styles.extraInfo}
                            hasWritePermissions={hasRepoMetaWritePermissions}
                        />
                    )}
                </section>
            )}

            <section
                className={classNames(
                    'test-tree-entries container mb-3 px-0',
                    styles.section,
                    !readmeEntry ? 'mt-3' : undefined
                )}
            >
                <FilesCard historyEntries={treeWithHistory} entries={tree.entries} className={styles.files} />

                {!isPackage && (
                    <div className={styles.contributors}>
                        {enableOwnershipPanels && showOwnership && (
                            <Card>
                                <CardHeader className={panelStyles.cardColHeaderWrapper}>Own</CardHeader>
                                <Ownership {...props} />
                            </Card>
                        )}
                        <Card className={enableOwnershipPanels && showOwnership ? 'mt-3' : undefined}>
                            <CardHeader className={panelStyles.cardColHeaderWrapper}>Contributors</CardHeader>
                            <Contributors {...props} />
                        </Card>
                    </div>
                )}
            </section>
        </>
    )
}

const CONTRIBUTORS_QUERY = gql`
    query TreePageRepositoryContributors(
        $repo: ID!
        $first: Int
        $revisionRange: String
        $afterDate: String
        $path: String
    ) {
        node(id: $repo) {
            ... on Repository {
                sourceType
                contributors(first: $first, revisionRange: $revisionRange, afterDate: $afterDate, path: $path) {
                    ...TreePageRepositoryContributorConnectionFields
                }
            }
        }
    }

    fragment TreePageRepositoryContributorConnectionFields on RepositoryContributorConnection {
        totalCount
        pageInfo {
            hasNextPage
        }
        nodes {
            ...TreePageRepositoryContributorNodeFields
        }
    }

    fragment TreePageRepositoryContributorNodeFields on RepositoryContributor {
        person {
            name
            displayName
            email
            avatarURL
            user {
                username
                url
                displayName
                avatarURL
            }
        }
        count
    }
`

interface ContributorsProps extends TreePageContentProps {}

const Contributors: React.FC<ContributorsProps> = ({ repo, filePath }) => {
    const spec: QuerySpec = {
        revisionRange: '',
        after: '',
        path: filePath,
    }

    const { data, error, loading } = useQuery<
        TreePageRepositoryContributorsResult,
        TreePageRepositoryContributorsVariables
    >(CONTRIBUTORS_QUERY, {
        variables: {
            first: COUNT,
            repo: repo.id,
            revisionRange: spec.revisionRange,
            afterDate: spec.after,
            path: filePath,
        },
    })

    const node = data?.node && data?.node.__typename === 'Repository' ? data.node : null
    const connection = node?.contributors

    return (
        <ConnectionContainer>
            {error && <ConnectionError errors={[error.message]} />}
            {connection && connection.nodes.length > 0 && (
                <ConnectionList
                    className={classNames('test-filtered-contributors-connection', styles.table)}
                    as="table"
                >
                    <tbody>
                        {connection.nodes.map(node => (
                            <RepositoryContributorNode
                                key={node.person.email}
                                node={node}
                                repoName={repo.name}
                                sourceType={repo.sourceType}
                                {...spec}
                            />
                        ))}
                    </tbody>
                </ConnectionList>
            )}
            {loading && (
                <div className={contributorsStyles.filteredConnectionLoading}>
                    <ConnectionLoading />
                </div>
            )}
            <SummaryContainer className={styles.tableSummary}>
                {connection && (
                    <>
                        <ConnectionSummary
                            compact={true}
                            connection={connection}
                            first={COUNT}
                            noun="contributor"
                            pluralNoun="contributors"
                            hasNextPage={connection.pageInfo.hasNextPage}
                        />
                        {connection.pageInfo.hasNextPage && (
                            <small>
                                <Link
                                    to={`${repo.url}/-/stats/contributors?${
                                        filePath ? 'path=' + encodeURIComponent(filePath) : ''
                                    }`}
                                >
                                    Show more
                                </Link>
                            </small>
                        )}
                    </>
                )}
            </SummaryContainer>
        </ConnectionContainer>
    )
}

const OWNERS_QUERY = gql`
    ${OWNER_FIELDS}
    ${RECENT_CONTRIBUTOR_FIELDS}
    ${RECENT_VIEW_FIELDS}

    query TreePageOwnership($repo: ID!, $first: Int, $revision: String!, $filePath: String!) {
        node(id: $repo) {
            ... on Repository {
                commit(rev: $revision) {
                    path(path: $filePath) {
                        ... on GitTree {
                            ownership(first: $first) {
                                ...TreePageOwnershipConnectionFields
                            }
                        }
                    }
                }
            }
        }
    }

    fragment TreePageOwnershipConnectionFields on OwnershipConnection {
        totalCount
        pageInfo {
            hasNextPage
        }
        nodes {
            ...TreePageOwnershipNodeFields
        }
    }

    fragment TreePageOwnershipNodeFields on Ownership {
        owner {
            ...OwnerFields
        }
        reasons {
            ...RecentContributorOwnershipSignalFields
            ...RecentViewOwnershipSignalFields
        }
    }
`

interface OwnershipProps extends TreePageContentProps {}

const Ownership: React.FC<OwnershipProps> = ({ repo, filePath }) => {
    const { data, error, loading } = useQuery<TreePageOwnershipResult, TreePageOwnershipVariables>(OWNERS_QUERY, {
        variables: {
            first: 5,
            repo: repo.id,
            revision: '',
            filePath,
        },
    })

    const node = data?.node && data?.node.__typename === 'Repository' ? data.node : null
    const connection =
        node?.commit?.path?.__typename === 'GitTree' &&
        node?.commit?.path?.ownership?.__typename === 'OwnershipConnection'
            ? node.commit.path.ownership
            : null
    return (
        <div>
            <ConnectionContainer>
                {error && <ConnectionError errors={[error.message]} />}
                {connection && connection.nodes.length > 0 && (
                    <ConnectionList
                        className={classNames('test-filtered-contributors-connection', styles.table)}
                        as="table"
                    >
                        <tbody>
                            {connection.nodes.map((node: TreePageOwnershipNodeFields) => (
                                <OwnerNode
                                    key={
                                        node.owner.__typename === 'Person'
                                            ? node.owner.email
                                            : node.owner.__typename === 'Team'
                                            ? node.owner.name
                                            : null
                                    }
                                    node={node}
                                />
                            ))}
                        </tbody>
                    </ConnectionList>
                )}
                {loading && (
                    <div className={contributorsStyles.filteredConnectionLoading}>
                        <ConnectionLoading />
                    </div>
                )}
                <SummaryContainer className={styles.tableSummary}>
                    {connection && (
                        <small>
                            <Link to={`${repo.url}/-/own?${filePath ? 'path=' + encodeURIComponent(filePath) : ''}`}>
                                Show more
                            </Link>
                        </small>
                    )}
                </SummaryContainer>
            </ConnectionContainer>
        </div>
    )
}

interface OwnerNodeProps {
    node: TreePageOwnershipNodeFields
}

const OwnerNode: React.FC<OwnerNodeProps> = ({ node }) => {
    const owner = node?.owner
    const primaryReason =
        node.reasons.find(reason => reason.__typename === 'AssignedOwner') ||
        node.reasons.find(reason => reason.__typename === 'RecentContributorOwnershipSignal') ||
        node.reasons[0]
    return (
        <tr className={classNames('list-group-item', contributorsStyles.repositoryContributorNode)}>
            <td className={contributorsStyles.person}>
                {/* TODO(#51791): Unify the component with FileOwnershipEntry. */}
                {owner.__typename === 'Person' && (
                    <>
                        <UserAvatar user={owner} className="mx-2" inline={true} />
                        <PersonLink person={owner} />
                    </>
                )}
                {owner.__typename === 'Team' && (
                    <>
                        <TeamAvatar
                            team={{ ...owner, displayName: owner.teamDisplayName }}
                            className="mx-2"
                            inline={true}
                        />
                        <Link to={`/teams/${owner.name}`}>{owner.teamDisplayName || owner.name}</Link>
                    </>
                )}
            </td>
            <td className={contributorsStyles.commits}>
                {primaryReason?.__typename === 'AssignedOwner' && (
                    <Badge tooltip="Owner assigned through sourcegraph" className={styles.badge} variant="merged">
                        owner
                    </Badge>
                )}
                {primaryReason?.__typename === 'RecentContributorOwnershipSignal' && (
                    <Badge tooltip={primaryReason.description} className={styles.badge} variant="secondary">
                        <Icon aria-label={primaryReason.title} svgPath={mdiFileOutline} /> changes
                    </Badge>
                )}
                {primaryReason?.__typename === 'RecentViewOwnershipSignal' && (
                    <Badge tooltip={primaryReason.description} className={styles.badge} variant="secondary">
                        <Icon aria-label={primaryReason.title} svgPath={mdiGlasses} /> views
                    </Badge>
                )}
                {node.reasons.length > 1 && (
                    <Badge tooltip="Multiple ownership inference signals" className={styles.badge} variant="secondary">
                        +{node.reasons.length - 1}
                    </Badge>
                )}
            </td>
        </tr>
    )
}

interface QuerySpec {
    revisionRange: string
    after: string
    path: string
}

interface RepositoryContributorNodeProps extends QuerySpec {
    node: TreePageRepositoryContributorNodeFields
    repoName: string
    sourceType: string
}

const RepositoryContributorNode: React.FC<RepositoryContributorNodeProps> = ({
    node,
    repoName,
    revisionRange,
    after,
    path,
    sourceType,
}) => {
    const query: string = [
        searchQueryForRepoRevision(repoName),
        'type:diff',
        `author:${quoteIfNeeded(node.person.email)}`,
        after ? `after:${quoteIfNeeded(after)}` : '',
        path ? `file:${quoteIfNeeded(escapeRegExp(path))}` : '',
    ]
        .join(' ')
        .replace(/\s+/, ' ')

    const refType = getRefType(sourceType)

    return (
        <tr className={classNames('list-group-item', contributorsStyles.repositoryContributorNode)}>
            <td className={contributorsStyles.person}>
                <UserAvatar inline={true} className="mr-2" user={node.person.user ? node.person.user : node.person} />
                <PersonLink person={node.person} />
            </td>
            <td className={contributorsStyles.commits}>
                <Tooltip
                    content={
                        revisionRange?.includes('..')
                            ? `All ${refType}s will be shown (revision end ranges are not yet supported)`
                            : null
                    }
                    placement="left"
                >
                    <Link to={`/search?${buildSearchURLQuery(query, SearchPatternType.standard, false)}`}>
                        {numberWithCommas(node.count)} {pluralize(refType, node.count)}
                    </Link>
                </Tooltip>
            </td>
        </tr>
    )
}
