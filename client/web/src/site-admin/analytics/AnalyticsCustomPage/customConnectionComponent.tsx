import React from 'react'

import { mdiDownload } from '@mdi/js'
import { format, parseISO } from 'date-fns'

import { dataOrThrowErrors } from '@sourcegraph/http-client'
import { AnchorLink, Button, ErrorAlert, H2, Icon, LoadingSpinner } from '@sourcegraph/wildcard'

import { useShowMorePagination } from '../../../components/FilteredConnection/hooks/useShowMorePagination'
import { ConnectionSummary } from '../../../components/FilteredConnection/ui'
import {
    AnalyticsUserActivityFields,
    CustomUsersConnectionResult,
    CustomUsersConnectionVariables,
} from '../../../graphql-operations'
import { IResult } from '../useChartFilters'

import { CUSTOM_USERS_CONNECTION } from './queries'
import { UserNode } from './userNode'

import styles from './index.module.scss'

interface Props extends Pick<IResult, 'dateRange' | 'grouping'> {
    debouncedSearchText: string[]
}

const DEFAULT_FIRST = 15

export const AnalyticsCustomConnectionComponent: React.FunctionComponent<Props> = (props: Props) => {
    const { dateRange, debouncedSearchText, grouping } = props
    const queryVariables = {
        dateRange: dateRange.value,
        grouping: grouping.value,
        events: debouncedSearchText,
        first: DEFAULT_FIRST,
        after: null,
    }

    const {
        connection,
        error: usersError,
        loading: usersLoading,
        fetchMore,
        hasNextPage,
    } = useShowMorePagination<CustomUsersConnectionResult, CustomUsersConnectionVariables, AnalyticsUserActivityFields>(
        {
            query: CUSTOM_USERS_CONNECTION,
            variables: queryVariables,
            getConnection: result => {
                const data = dataOrThrowErrors(result)
                return data.site.analytics.custom.userActivity
            },
            options: {
                fetchPolicy: 'cache-and-network',
                useURL: true,
            },
        }
    )

    return (
        <>
            <H2 className="my-3">Events by user</H2>
            <div className={styles.connectionContainer}>
                {usersError && !usersLoading && <ErrorAlert error={usersError} />}
                <div className="d-flex align-items-end justify-content-between mt-3">
                    {connection && (
                        <ConnectionSummary
                            connection={connection}
                            hasNextPage={hasNextPage}
                            first={DEFAULT_FIRST}
                            noun="user"
                            pluralNoun="users"
                            className="mb-0"
                        />
                    )}
                </div>
                {usersLoading && !usersError && <LoadingSpinner className="d-block mx-auto mt-3" />}
                {connection?.nodes && connection.nodes.length > 0 && (
                    <ul className="list-group list-group-flush mt-2">
                        <li className="list-group-item px-0 py-2 font-weight-bold">
                            <div className={styles.node}>
                                <div className={styles.user}>User</div>
                                {connection?.nodes[0].periods.map(period => (
                                    <div className={styles.period} key={period.date}>
                                        {format(parseISO(period.date), 'dd MMM')}
                                    </div>
                                ))}
                                <div className={styles.period}>Total</div>
                            </div>
                        </li>
                        {(connection?.nodes || []).map(node => (
                            <UserNode node={node} key={node.userID} />
                        ))}
                    </ul>
                )}
                {connection?.nodes && connection.totalCount !== connection.nodes.length && hasNextPage && (
                    <div>
                        <Button variant="link" size="sm" onClick={fetchMore}>
                            Show more
                        </Button>
                    </div>
                )}
                <div className="mt-3">
                    <Button
                        to={`/site-admin/admin-analytics-all-events/archive?names=${debouncedSearchText}&dateRange=${dateRange.value}`}
                        download="true"
                        className="mr-4"
                        variant="secondary"
                        outline={true}
                        as={AnchorLink}
                    >
                        <Icon svgPath={mdiDownload} aria-label="Download usage stats" className="mr-1" />
                        Download all matching events
                    </Button>
                </div>
            </div>
        </>
    )
}
