import { useMemo } from 'react'

import { CodyProRoutes } from './codyProRoutes'
import { useSubscriptionSummary } from './management/api/react-query/subscriptions'
import { getManageSubscriptionPageURL, isEmbeddedCodyProUIEnabled } from './util'

export const useCodyProNavLinks = (): { to: string; label: string }[] => {
    const { data } = useSubscriptionSummary()

    return useMemo(() => {
        if (!data || data.userRole !== 'admin') {
            return []
        }

        const items = [{ to: getManageSubscriptionPageURL(), label: 'Manage subscription' }]

        if (isEmbeddedCodyProUIEnabled()) {
            items.push({ to: CodyProRoutes.ManageTeam, label: 'Manage team' })
        }

        return items
    }, [data])
}
