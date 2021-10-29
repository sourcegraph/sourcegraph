import { ListboxGroup, ListboxGroupLabel, ListboxInput, ListboxList, ListboxPopover } from '@reach/listbox'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'
import React from 'react'

import { useObservable } from '@sourcegraph/shared/src/util/useObservable'
import { AuthenticatedUser, authenticatedUser } from '@sourcegraph/web/src/auth'

import {
    InsightDashboard,
    InsightDashboardOwner,
    InsightsDashboardType,
    isGlobalDashboard,
    isOrganizationDashboard,
    isPersonalDashboard,
    RealInsightDashboard,
} from '../../../../../core/types'

import { MenuButton } from './components/menu-button/MenuButton'
import { SelectDashboardOption, SelectOption } from './components/select-option/SelectOption'
import styles from './DashboardSelect.module.scss'

const LABEL_ID = 'insights-dashboards--select'

export interface DashboardSelectProps {
    value: string | undefined
    dashboards: InsightDashboard[]

    onSelect: (dashboard: InsightDashboard) => void
    className?: string
}

/**
 * Renders dashboard select component for the code insights dashboard page selection UI.
 */
export const DashboardSelect: React.FunctionComponent<DashboardSelectProps> = props => {
    const { value, dashboards, onSelect, className } = props
    const user = useObservable(authenticatedUser)
    if (!user) {
        return null
    }

    const handleChange = (value: string): void => {
        const dashboard = dashboards.find(dashboard => dashboard.id === value)

        if (dashboard) {
            onSelect(dashboard)
        }
    }

    const organizationGroups = getDashboardOrganizationsGroups(dashboards, user.organizations.nodes)

    return (
        <div className={className}>
            <VisuallyHidden id={LABEL_ID}>Choose a dashboard</VisuallyHidden>

            <ListboxInput aria-labelledby={LABEL_ID} value={value ?? 'unknown'} onChange={handleChange}>
                <MenuButton dashboards={dashboards} />

                <ListboxPopover className={classNames(styles.popover)} portal={true}>
                    <ListboxList className={classNames(styles.list, 'dropdown-menu')}>
                        <SelectOption
                            value={InsightsDashboardType.All}
                            label="All Insights"
                            className={styles.option}
                        />

                        {dashboards.some(isPersonalDashboard) && (
                            <ListboxGroup>
                                <ListboxGroupLabel className={classNames(styles.groupLabel, 'text-muted')}>
                                    Private
                                </ListboxGroupLabel>

                                {dashboards.filter(isPersonalDashboard).map(dashboard => (
                                    <SelectDashboardOption
                                        key={dashboard.id}
                                        dashboard={dashboard}
                                        className={styles.option}
                                    />
                                ))}
                            </ListboxGroup>
                        )}

                        {dashboards.some(isGlobalDashboard) && (
                            <ListboxGroup>
                                <ListboxGroupLabel className={classNames(styles.groupLabel, 'text-muted')}>
                                    Global
                                </ListboxGroupLabel>

                                {dashboards.filter(isGlobalDashboard).map(dashboard => (
                                    <SelectDashboardOption
                                        key={dashboard.id}
                                        dashboard={dashboard}
                                        className={styles.option}
                                    />
                                ))}
                            </ListboxGroup>
                        )}

                        {organizationGroups.map(group => (
                            <ListboxGroup key={group.id}>
                                <ListboxGroupLabel className={classNames(styles.groupLabel, 'text-muted')}>
                                    {group.name}
                                </ListboxGroupLabel>

                                {group.dashboards.map(dashboard => (
                                    <SelectDashboardOption
                                        key={dashboard.id}
                                        dashboard={dashboard}
                                        className={styles.option}
                                    />
                                ))}
                            </ListboxGroup>
                        ))}
                    </ListboxList>
                </ListboxPopover>
            </ListboxInput>
        </div>
    )
}

interface DashboardOrganizationGroup {
    id: string
    name: string
    dashboards: RealInsightDashboard[]
}

/**
 * Returns organization dashboards grouped by dashboard owner id
 */
const getDashboardOrganizationsGroups = (
    dashboards: InsightDashboard[],
    organizations: AuthenticatedUser['organizations']['nodes']
): DashboardOrganizationGroup[] => {
    // We need a map of the organization names when using the new GraphQL API
    const organizationsMap = organizations.reduce<Record<string, InsightDashboardOwner>>(
        (map, organization) => ({
            ...map,
            [organization.id]: {
                id: organization.id,
                name: organization.displayName ?? organization.name,
            },
        }),
        {}
    )

    const groupsDictionary = dashboards
        .map((dashboard: InsightDashboard) => {
            const owner =
                ('owner' in dashboard && dashboard.owner) ||
                ('grants' in dashboard &&
                    dashboard.grants?.organizations &&
                    organizationsMap[dashboard.grants?.organizations[0]])
            // Grabbing the first organization to minimize changes with existing api
            // TODO: handle multiple organizations when settings API is deprecated

            if (!owner) {
                return dashboard
            }

            return {
                ...dashboard,
                owner,
            }
        })
        .filter(isOrganizationDashboard)
        .reduce<Record<string, DashboardOrganizationGroup>>((store, dashboard) => {
            if (!dashboard.owner) {
                // TODO: remove this check after settings api is deprecated
                throw new Error('`owner` is missing from the dashboard')
            }

            if (!store[dashboard.owner.id]) {
                store[dashboard.owner.id] = {
                    id: dashboard.owner.id,
                    name: dashboard.owner.name,
                    dashboards: [],
                }
            }

            store[dashboard.owner.id].dashboards.push(dashboard)

            return store
        }, {})

    return Object.values(groupsDictionary)
}
