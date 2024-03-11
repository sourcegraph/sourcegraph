import React from 'react'

import classNames from 'classnames'

import { FilterLink, type RevisionsProps, TabIndex } from '@sourcegraph/branded'
import { styles } from '@sourcegraph/branded/src/search-ui/results/sidebar/SearchFilterSection'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { FilterType } from '@sourcegraph/shared/src/search/query/filters'
import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary/useTemporarySetting'
import { Button, LoadingSpinner, Tab, TabList, TabPanel, TabPanels, Tabs, Text } from '@sourcegraph/wildcard'

import { useShowMorePagination } from '../../../components/FilteredConnection/hooks/useShowMorePagination'
import {
    type SearchSidebarGitRefsResult,
    type SearchSidebarGitRefsVariables,
    type SearchSidebarGitRefFields,
    GitRefType,
} from '../../../graphql-operations'

import revisionStyles from './Revisions.module.scss'

const DEFAULT_FIRST = 10
const GIT_REVS_QUERY = gql`
    query SearchSidebarGitRefs($repo: String, $first: Int, $query: String, $type: GitRefType) {
        repository(name: $repo) {
            ... on Repository {
                __typename
                id
                gitRefs(first: $first, query: $query, type: $type) {
                    __typename
                    nodes {
                        ...SearchSidebarGitRefFields
                    }
                    totalCount
                    pageInfo {
                        hasNextPage
                    }
                }
            }
        }
    }

    fragment SearchSidebarGitRefFields on GitRef {
        __typename
        id
        name
        displayName
    }
`

interface RevisionListProps {
    repoName: string
    type: GitRefType
    onFilterClick: (value: string) => void
    pluralNoun: string
    query: string
}

const RevisionList: React.FunctionComponent<React.PropsWithChildren<RevisionListProps>> = ({
    repoName,
    type,
    onFilterClick,
    pluralNoun,
    query,
}) => {
    const { connection, fetchMore, hasNextPage, loading, error } = useShowMorePagination<
        SearchSidebarGitRefsResult,
        SearchSidebarGitRefsVariables,
        SearchSidebarGitRefFields
    >({
        query: GIT_REVS_QUERY,
        variables: {
            first: DEFAULT_FIRST,
            repo: repoName,
            query,
            type,
        },
        getConnection: result => {
            const data = dataOrThrowErrors(result)
            if (!data?.repository?.gitRefs) {
                throw new Error('Unable to fetch repo revisions.')
            }
            return data?.repository?.gitRefs
        },
    })

    if (loading) {
        return (
            <div className={classNames('d-flex justify-content-center mt-4', styles.sidebarSectionNoResults)}>
                <LoadingSpinner />
            </div>
        )
    }

    if (error || !connection || connection.error) {
        return (
            <Text className={classNames('text-muted', styles.sidebarSectionNoResults)}>
                <span className="text-muted">Unable to fetch repository revisions.</span>
            </Text>
        )
    }

    if (connection.nodes.length === 0) {
        return (
            <Text className={classNames('text-muted', styles.sidebarSectionNoResults)}>
                {query
                    ? `None of the ${pluralNoun} in this repository match this filter.`
                    : `This repository doesn't have any ${pluralNoun}.`}
            </Text>
        )
    }

    return (
        <>
            <ul className={styles.sidebarSectionList}>
                {connection?.nodes.map(node => (
                    <FilterLink
                        key={node.name}
                        label={node.displayName}
                        value={node.name}
                        onFilterChosen={onFilterClick}
                    />
                ))}
            </ul>
            {(connection.totalCount ?? 0) > DEFAULT_FIRST ? (
                <Text className={classNames('text-muted d-flex', styles.sidebarSectionFooter)}>
                    <small className="flex-1" data-testid="summary">
                        {connection?.nodes.length} of {connection?.totalCount} {pluralNoun}
                    </small>
                    {hasNextPage ? (
                        <Button className={styles.sidebarSectionButtonLink} onClick={fetchMore} variant="link">
                            Show more
                        </Button>
                    ) : null}
                </Text>
            ) : null}
        </>
    )
}

export const Revisions: React.FunctionComponent<React.PropsWithChildren<RevisionsProps>> = React.memo(
    ({ repoName, onFilterClick, query, _initialTab }) => {
        const [persistedTabIndex, setPersistedTabIndex] = useTemporarySetting('search.sidebar.revisions.tab')
        const onRevisionFilterClick = (value: string): void =>
            onFilterClick([
                { type: 'updateOrAppendFilter', field: FilterType.rev, value },
                { type: 'appendFilter', field: FilterType.repo, value: `^${repoName}$`, unique: true },
            ])
        return (
            <Tabs
                defaultIndex={_initialTab ?? persistedTabIndex ?? 0}
                onChange={setPersistedTabIndex}
                className={revisionStyles.tabs}
            >
                <TabList>
                    <Tab index={TabIndex.BRANCHES}>Branches</Tab>
                    <Tab index={TabIndex.TAGS}>Tags</Tab>
                </TabList>
                <TabPanels>
                    <TabPanel>
                        <RevisionList
                            pluralNoun="branches"
                            repoName={repoName}
                            type={GitRefType.GIT_BRANCH}
                            onFilterClick={onRevisionFilterClick}
                            query={query}
                        />
                    </TabPanel>
                    <TabPanel>
                        <RevisionList
                            pluralNoun="tags"
                            repoName={repoName}
                            type={GitRefType.GIT_TAG}
                            onFilterClick={onRevisionFilterClick}
                            query={query}
                        />
                    </TabPanel>
                </TabPanels>
            </Tabs>
        )
    }
)
Revisions.displayName = 'Revisions'

export const getRevisions = (props: Omit<RevisionsProps, 'query'>) =>
    function RevisionsSection(query: string) {
        // eslint-disable-next-line no-restricted-syntax
        return <Revisions {...props} query={query} />
    }
