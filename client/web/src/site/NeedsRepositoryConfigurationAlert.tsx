import React, { useCallback, useEffect } from 'react'

import classNames from 'classnames'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Link } from '@sourcegraph/wildcard'

import { DismissibleAlert } from '../components/DismissibleAlert'
import { PageRoutes } from '../routes.constants'

interface Props extends TelemetryV2Props {
    className?: string
}

/**
 * A global alert telling the site admin that they need to configure repositories
 * on this site.
 */
export const NeedsRepositoryConfigurationAlert: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    className,
    telemetryRecorder,
}) => {
    useEffect(() => telemetryRecorder.recordEvent('alert.needsRepoConfig', 'view'), [telemetryRecorder])
    const onClickCTA = useCallback(
        () => telemetryRecorder.recordEvent('alert.needsRepoConfig.CTA', 'click'),
        [telemetryRecorder]
    )
    return (
        <DismissibleAlert
            partialStorageKey="needsRepositoryConfiguration"
            variant="success"
            className={classNames('d-flex align-items-center', className)}
        >
            <Link
                className="site-alert__link"
                to={`${PageRoutes.SetupWizard}/remote-repositories`}
                onClick={onClickCTA}
            >
                <span className="underline">Go to setup wizard</span>
            </Link>
            &nbsp;to add remote repositories from GitHub, GitLab, etc.
        </DismissibleAlert>
    )
}
