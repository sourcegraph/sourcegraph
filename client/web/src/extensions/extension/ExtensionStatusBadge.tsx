import React from 'react'
import { Badge } from '../../components/Badge'

/**
 * Shows a "WIP" badge for extensions.
 */
export const ExtensionStatusBadge: React.FunctionComponent<{ viewerCanAdminister: boolean }> = ({
    viewerCanAdminister,
}) => (
    <Badge
        status="wip"
        tooltip={
            viewerCanAdminister
                ? 'Remove "WIP" from the title when this extension is ready for use.'
                : 'Work in progress (not ready for use)'
        }
        className="text-uppercase"
    />
)
