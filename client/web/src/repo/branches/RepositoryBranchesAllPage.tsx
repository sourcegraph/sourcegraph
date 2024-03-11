import { type FC, useCallback, useEffect } from 'react'

import type { Observable } from 'rxjs'

import { Container } from '@sourcegraph/wildcard'

import { FilteredConnection, type FilteredConnectionQueryArguments } from '../../components/FilteredConnection'
import { Page } from '../../components/Page'
import { PageTitle } from '../../components/PageTitle'
import { GitRefType, type GitRefConnectionFields, type GitRefFields } from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'
import { GitReferenceNode, queryGitReferences } from '../GitReference'

import type { RepositoryBranchesAreaPageProps } from './RepositoryBranchesArea'

interface Props extends RepositoryBranchesAreaPageProps {}

/** A page that shows all of a repository's branches. */
export const RepositoryBranchesAllPage: FC<Props> = props => {
    const { repo } = props

    useEffect(() => {
        eventLogger.logViewEvent('RepositoryBranchesAll')
    }, [])

    const queryBranches = useCallback(
        (args: FilteredConnectionQueryArguments): Observable<GitRefConnectionFields> =>
            queryGitReferences({ ...args, repo: repo.id, type: GitRefType.GIT_BRANCH }),
        [repo.id]
    )

    return (
        <Page>
            <PageTitle title="All branches" />

            <Container>
                <FilteredConnection<GitRefFields>
                    inputClassName="w-100"
                    listClassName="list-group list-group-flush"
                    noun="branch"
                    pluralNoun="branches"
                    queryConnection={queryBranches}
                    nodeComponent={GitReferenceNode}
                    ariaLabelFunction={(branchDisplayName: string) =>
                        `View this repository using ${branchDisplayName} as the selected revision`
                    }
                    defaultFirst={20}
                    autoFocus={true}
                />
            </Container>
        </Page>
    )
}
