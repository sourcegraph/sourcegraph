import React from 'react'
import { Redirect } from 'react-router'
import { LayoutRouteProps, routes } from '../routes'
import { lazyComponent } from '../util/lazyComponent'
import { isErrorLike } from '../../../shared/src/util/errors'
import { Settings } from '../schema/settings.schema'

export const enterpriseRoutes: readonly LayoutRouteProps<{}>[] = [
    {
        // Allow unauthenticated viewers to view the "new subscription" page to price out a subscription (instead
        // of just dumping them on a sign-in page).
        path: '/subscriptions/new',
        exact: true,
        render: lazyComponent(
            () => import('./user/productSubscriptions/NewProductSubscriptionPageOrRedirectUser'),
            'NewProductSubscriptionPageOrRedirectUser'
        ),
    },
    {
        // Redirect from old /user/subscriptions/new -> /subscriptions/new.
        path: '/user/subscriptions/new',
        exact: true,
        render: () => <Redirect to="/subscriptions/new" />,
    },
    {
        path: '/campaigns',
        render: ({ match }) => <Redirect to={match.path.replace('/campaigns', '/batch-changes')} />,
    },
    {
        path: '/batch-changes',
        render: lazyComponent(() => import('./batches/global/GlobalBatchChangesArea'), 'GlobalBatchChangesArea'),
        condition: props => props.showBatchChanges,
    },
    {
        path: '/stats',
        render: lazyComponent(() => import('./search/stats/SearchStatsPage'), 'SearchStatsPage'),
        condition: ({ settingsCascade }) => {
            if (settingsCascade.final === null || isErrorLike(settingsCascade.final)) {
                return false
            }
            const settings: Settings = settingsCascade.final
            return Boolean(settings.experimentalFeatures?.searchStats)
        },
    },
    {
        path: '/code-monitoring',
        render: lazyComponent(
            () => import('./code-monitoring/global/GlobalCodeMonitoringArea'),
            'GlobalCodeMonitoringArea'
        ),
        condition: props =>
            !isErrorLike(props.settingsCascade.final) &&
            !!props.settingsCascade.final?.experimentalFeatures?.codeMonitoring,
    },
    ...routes,
]
