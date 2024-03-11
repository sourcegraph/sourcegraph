import * as React from 'react'

import classNames from 'classnames'
import type { Observable } from 'rxjs'
import { map } from 'rxjs/operators'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { createAggregateError, numberWithCommas, memoizeObservable } from '@sourcegraph/common'
import { gql } from '@sourcegraph/http-client'
import { Badge, Icon, LinkOrSpan } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../backend/graphql'
import {
    type GitRefConnectionFields,
    type GitRefFields,
    GitRefType,
    type RepositoryGitRefsResult,
    type RepositoryGitRefsVariables,
    type Scalars,
} from '../graphql-operations'

import styles from './GitReference.module.scss'

export interface GitReferenceNodeProps {
    node: GitRefFields

    /** Link URL; if undefined, node.url is used. */
    url?: string

    /** Whether any ancestor element higher up in the tree is an `<a>` element. */
    ancestorIsLink?: boolean

    children?: React.ReactNode

    className?: string

    icon?: React.ComponentType<{ className?: string }>

    onClick?: React.MouseEventHandler<HTMLAnchorElement>
    nodeLinkClassName?: string

    ariaLabel?: string

    isPackageVersion?: boolean
}

export const GitReferenceNode: React.FunctionComponent<React.PropsWithChildren<GitReferenceNodeProps>> = ({
    node,
    url,
    ancestorIsLink,
    children,
    className,
    onClick,
    icon: ReferenceIcon,
    nodeLinkClassName,
    ariaLabel,
    isPackageVersion,
}) => {
    const mostRecentSig =
        !isPackageVersion &&
        node.target.commit &&
        (node.target.commit.committer && node.target.commit.committer.date > node.target.commit.author.date
            ? node.target.commit.committer
            : node.target.commit.author)
    const behindAhead = node.target.commit?.behindAhead
    url = url !== undefined ? url : node.url

    return (
        <li key={node.id} className={classNames('d-block list-group-item', styles.gitRefNode, className)}>
            <LinkOrSpan
                className={classNames(styles.gitRefNodeLink, nodeLinkClassName)}
                to={!ancestorIsLink ? url : undefined}
                onClick={onClick}
                data-testid="git-ref-node"
                aria-label={ariaLabel}
            >
                <span className="d-flex flex-wrap align-items-center">
                    {ReferenceIcon && <Icon className="mr-1" as={ReferenceIcon} aria-hidden={true} />}
                    {/*
                    a11y-ignore
                    Rule: "color-contrast" (Elements must have sufficient color contrast)
                    GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/33343
                */}
                    <Badge className="a11y-ignore px-1 py-0 mr-2 text-break text-wrap text-justify" as="code">
                        {node.displayName}
                    </Badge>
                    {mostRecentSig && (
                        <small>
                            Updated <Timestamp date={mostRecentSig.date} />{' '}
                            {mostRecentSig.person && <>by {mostRecentSig.person.displayName}</>}
                        </small>
                    )}
                </span>
                {behindAhead && (
                    <small>
                        {numberWithCommas(behindAhead.behind)} behind, {numberWithCommas(behindAhead.ahead)} ahead
                    </small>
                )}
                {children}
            </LinkOrSpan>
        </li>
    )
}

export const gitReferenceFragments = gql`
    fragment GitRefFields on GitRef {
        id
        displayName
        name
        abbrevName
        url
        target {
            commit {
                author {
                    ...SignatureFieldsForReferences
                }
                committer {
                    ...SignatureFieldsForReferences
                }
                behindAhead(revspec: "HEAD") @include(if: $withBehindAhead) {
                    behind
                    ahead
                }
            }
        }
    }

    fragment SignatureFieldsForReferences on Signature {
        __typename
        person {
            displayName
            user {
                username
            }
        }
        date
    }
`

export const REPOSITORY_GIT_REFS = gql`
    query RepositoryGitRefs($repo: ID!, $first: Int, $query: String, $type: GitRefType!, $withBehindAhead: Boolean!) {
        node(id: $repo) {
            __typename
            ... on Repository {
                gitRefs(first: $first, query: $query, type: $type) {
                    __typename
                    ...GitRefConnectionFields
                }
            }
        }
    }

    fragment GitRefConnectionFields on GitRefConnection {
        nodes {
            __typename
            ...GitRefFields
        }
        totalCount
        pageInfo {
            hasNextPage
        }
    }

    ${gitReferenceFragments}
`

export const queryGitReferences = memoizeObservable(
    (args: {
        repo: Scalars['ID']
        first?: number
        query?: string
        type: GitRefType
        withBehindAhead?: boolean
    }): Observable<GitRefConnectionFields> =>
        requestGraphQL<RepositoryGitRefsResult, RepositoryGitRefsVariables>(REPOSITORY_GIT_REFS, {
            query: args.query ?? null,
            first: args.first ?? null,
            repo: args.repo,
            type: args.type,
            withBehindAhead:
                args.withBehindAhead !== undefined ? args.withBehindAhead : args.type === GitRefType.GIT_BRANCH,
        }).pipe(
            map(({ data, errors }) => {
                if (data?.node?.__typename !== 'Repository' || !data?.node?.gitRefs) {
                    throw createAggregateError(errors)
                }
                return data.node.gitRefs
            })
        ),
    args => `${args.repo}:${String(args.first)}:${String(args.query)}:${args.type}`
)
