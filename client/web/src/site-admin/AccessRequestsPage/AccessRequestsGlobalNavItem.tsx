import type { FC } from 'react'

import { mdiAccountPlus } from '@mdi/js'

import { pluralize } from '@sourcegraph/common'
import { useQuery } from '@sourcegraph/http-client'
import { ButtonLink, Text, Icon } from '@sourcegraph/wildcard'

import type { AccessRequestsCountResult, AccessRequestsCountVariables } from '../../graphql-operations'
import { checkRequestAccessAllowed } from '../../util/checkRequestAccessAllowed'

import { ACCESS_REQUESTS_COUNT } from './queries'

interface AccessRequestsGlobalNavItemProps {
    className?: string
}

/**
 * A link to the access requests page that the number of pending requests.
 * Does not render anything if request access is not allowed or there are no pending requests.
 */
export const AccessRequestsGlobalNavItem: FC<AccessRequestsGlobalNavItemProps> = props => {
    const { className } = props
    const isRequestAccessAllowed = checkRequestAccessAllowed(window.context)

    const { data } = useQuery<AccessRequestsCountResult, AccessRequestsCountVariables>(ACCESS_REQUESTS_COUNT, {
        fetchPolicy: 'network-only',
        skip: !isRequestAccessAllowed,
    })

    if (!data?.accessRequests.totalCount) {
        return null
    }

    return (
        <ButtonLink variant="success" size="sm" to="/site-admin/account-requests" className={className}>
            <Icon svgPath={mdiAccountPlus} size="md" aria-label="Account requests icons" color="var(--success-2)" />
            <Text className="mx-1" weight="bold" as="span">
                {data?.accessRequests.totalCount}
            </Text>
            Account {pluralize('request', data?.accessRequests.totalCount)}
        </ButtonLink>
    )
}
