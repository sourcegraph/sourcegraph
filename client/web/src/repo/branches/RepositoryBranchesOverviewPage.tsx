import React, { useMemo } from 'react'

import { mdiChevronRight } from '@mdi/js'

import { Link, LoadingSpinner, CardHeader, Card, Icon, ErrorAlert } from '@sourcegraph/wildcard'

import { Page } from '../../components/Page'
import { PageTitle } from '../../components/PageTitle'
import { eventLogger } from '../../tracking/eventLogger'
import { GitReferenceNode } from '../GitReference'

import { useBranches } from './backend'
import type { RepositoryBranchesAreaPageProps } from './RepositoryBranchesArea'

import styles from './RepositoryBranchesOverviewPage.module.scss'

interface Props extends RepositoryBranchesAreaPageProps {}

/** A page with an overview of the repository's branches. */
export const RepositoryBranchesOverviewPage: React.FunctionComponent<Props> = ({ repo }) => {
    useMemo(() => {
        eventLogger.logViewEvent('RepositoryBranchesOverview')
    }, [])

    const { loading, error, activeBranches, defaultBranch, hasMoreActiveBranches } = useBranches(repo.id, 10)

    if (loading) {
        return <LoadingSpinner className="mt-2 mx-auto" />
    }

    if (error) {
        return <ErrorAlert className="mt-2" error={error} />
    }

    return (
        <Page>
            <PageTitle title="Branches" />

            <div>
                {defaultBranch && (
                    <Card className={styles.card}>
                        <CardHeader>Default branch</CardHeader>
                        <ul className="list-group list-group-flush">
                            <GitReferenceNode
                                node={defaultBranch}
                                ariaLabel={`View this repository using ${defaultBranch.displayName} as the selected revision`}
                            />
                        </ul>
                    </Card>
                )}
                {activeBranches.length > 0 && (
                    <Card className={styles.card}>
                        <CardHeader>Active branches</CardHeader>
                        <ul className="list-group list-group-flush" data-testid="active-branches-list">
                            {activeBranches.map((gitReference, index) => (
                                <GitReferenceNode
                                    key={index}
                                    node={gitReference}
                                    ariaLabel={`View this repository using ${gitReference.displayName} as the selected revision`}
                                />
                            ))}
                            {hasMoreActiveBranches && (
                                <li className="list-group-item list-group-item-action">
                                    <Link
                                        className="py-2 d-flex align-items-center"
                                        to={`/${repo.name}/-/branches/all`}
                                    >
                                        View more branches
                                        <Icon aria-hidden={true} svgPath={mdiChevronRight} />
                                    </Link>
                                </li>
                            )}
                        </ul>
                    </Card>
                )}
            </div>
        </Page>
    )
}
