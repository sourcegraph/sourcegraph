import type { FC } from 'react'

import { mdiFlaskEmptyOutline } from '@mdi/js'
import { useLocation } from 'react-router-dom'

import { Button, Icon, Tooltip } from '@sourcegraph/wildcard'

import { useFeatureFlag } from '../featureFlags/useFeatureFlag'

import { reload, isSupportedRoute } from './util'

export const SvelteKitNavItem: FC = () => {
    const location = useLocation()
    const [isEnabled] = useFeatureFlag('web-next-toggle')

    if (!isEnabled || !isSupportedRoute(location.pathname)) {
        return null
    }

    return (
        <Tooltip content="Go to experimental web app">
            <Button variant="icon" onClick={reload}>
                <span className="text-muted">
                    <Icon svgPath={mdiFlaskEmptyOutline} aria-hidden={true} inline={false} />
                </span>
            </Button>
        </Tooltip>
    )
}
