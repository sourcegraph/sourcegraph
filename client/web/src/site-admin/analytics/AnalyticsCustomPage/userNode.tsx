import React from 'react'

import { Link } from '@sourcegraph/wildcard'

import { AnalyticsUserActivityFields } from '../../../graphql-operations'

import styles from './index.module.scss'

interface UserNodeProps {
    node: AnalyticsUserActivityFields
    setFilterPackage?: (node: AnalyticsUserActivityFields) => void
}

export const UserNode: React.FunctionComponent<React.PropsWithChildren<UserNodeProps>> = ({ node }) => (
    <li className="list-group-item px-0 py-2">
        <div className={styles.node}>
            <div className={styles.user}>
                <Link to={`/users/${node.userID}`}>{node.username}</Link>
            </div>
            {node.periods.map(period => (
                <div className={styles.period} key={node.userID + ':' + period.date}>
                    {period.count}
                </div>
            ))}
            <div className={styles.period}>{node.totalEventCount}</div>
        </div>
    </li>
)
