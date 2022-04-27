import React from 'react'

import { ListboxOption } from '@reach/listbox'
import classNames from 'classnames'

import { TruncatedText } from '../../../../../../../components'
import { CustomInsightDashboard } from '../../../../../../../core'
import { getDashboardOwnerName, getDashboardTitle } from '../../helpers/get-dashboard-title'
import { InsightsBadge } from '../insights-badge/InsightsBadge'

import styles from './SelectOption.module.scss'

export interface SelectOptionProps {
    /** Value for the list-option element */
    value: string

    /** List-box label text */
    label: string

    /** Badge text */
    badge?: string

    className?: string

    filter?: string
}

/**
 * Displays simple text (label) list select (list-box) option.
 */
export const SelectOption: React.FunctionComponent<SelectOptionProps> = props => {
    const { value, label, badge, className, filter = '' } = props
    const matcher = new RegExp(`(${filter})`, 'ig')
    const splitLabel = label.split(matcher)
    const parsedLabel =
        filter.length > 0
            ? splitLabel.map((chunk, index) =>
                  matcher.test(chunk) ? <b key={index}>{chunk}</b> : <span key={index}>{chunk}</span>
              )
            : label

    return (
        <ListboxOption className={classNames(styles.option, className)} value={value}>
            <TruncatedText title={label} className={styles.text}>
                {parsedLabel}
            </TruncatedText>
            {badge && <InsightsBadge value={badge} className={styles.badge} />}
        </ListboxOption>
    )
}

interface SelectDashboardOptionProps {
    dashboard: CustomInsightDashboard
    className?: string
    filter?: string
}

/**
 * Displays select dashboard list-box options.
 */
export const SelectDashboardOption: React.FunctionComponent<SelectDashboardOptionProps> = props => {
    const { dashboard, className, filter = '' } = props

    return (
        <SelectOption
            value={dashboard.id}
            label={getDashboardTitle(dashboard)}
            badge={getDashboardOwnerName(dashboard)}
            filter={filter}
            className={className}
        />
    )
}
