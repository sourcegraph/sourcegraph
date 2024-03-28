import React, { useCallback, useEffect } from 'react'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Alert, Link } from '@sourcegraph/wildcard'

interface Props extends TelemetryV2Props {
    noLicenseWarningUserCount: number | null
    className?: string
}

/**
 * A global alert that appears telling all users that they have exceeded the limit of free users allowed.
 */
export const FreeUsersExceededAlert: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    noLicenseWarningUserCount,
    className,
    telemetryRecorder,
}) => {
    useEffect(() => telemetryRecorder.recordEvent('alert.freeUsersExceeded', 'view'), [telemetryRecorder])
    const onClickCTA = useCallback(
        () => telemetryRecorder.recordEvent('alert.freeUsersExceeded.CTA', 'click'),
        [telemetryRecorder]
    )
    return (
        <Alert className={className} variant="danger">
            This Sourcegraph instance has reached{' '}
            {noLicenseWarningUserCount === null ? 'the limit for' : noLicenseWarningUserCount} free users, and an admin
            must{' '}
            <Link className="site-alert__link" to="https://sourcegraph.com/contact/sales" onClick={onClickCTA}>
                <span className="underline">contact Sourcegraph to start a free trial or purchase a license</span>
            </Link>{' '}
            to add more
        </Alert>
    )
}
