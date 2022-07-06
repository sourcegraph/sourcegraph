import React from 'react'

import { mdiChartLineVariant, mdiInformationOutline } from '@mdi/js'

import { H2, Icon, Tooltip } from '@sourcegraph/wildcard'

export const AnalyticsPageTitle: React.FunctionComponent = ({ children }) => (
    <H2 className="mb-4 d-flex align-items-center">
        <Icon
            className="mr-1"
            color="var(--link-color)"
            svgPath={mdiChartLineVariant}
            size="sm"
            aria-label="Analytics icon"
        />
        {children}
        <Tooltip content="Data is updated every 24 hour.">
            <Icon
                className="ml-1"
                svgPath={mdiInformationOutline}
                aria-label="Analytics info icon"
                size="sm"
                color="var(--link-color)"
            />
        </Tooltip>
    </H2>
)
