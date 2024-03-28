import { useEffect, type FunctionComponent, type PropsWithChildren } from 'react'

import classNames from 'classnames'

import { renderMarkdown } from '@sourcegraph/common'
import { AlertType } from '@sourcegraph/shared/src/graphql-operations'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Alert, Markdown } from '@sourcegraph/wildcard'

import { DismissibleAlert, type DismissibleAlertProps } from '../components/DismissibleAlert'
import type { SiteFlagAlertFields } from '../graphql-operations'

// For telemetry
const v2AlertTypes: { [key in AlertType]: number } = {
    ERROR: 1,
    WARNING: 2,
    INFO: 3,
}

interface Props extends TelemetryV2Props {
    alert: SiteFlagAlertFields
    className?: string
}

/**
 * A global alert that is shown at the top of the viewport.
 */
export const GlobalAlert: FunctionComponent<PropsWithChildren<Props>> = ({
    alert,
    className: commonClassName,
    telemetryRecorder,
}) => {
    useEffect(
        () =>
            telemetryRecorder.recordEvent('alert.global', 'view', {
                metadata: { type: alert.type ? v2AlertTypes[alert.type] : 0 },
            }),
        [telemetryRecorder, alert.type]
    )

    const content = <Markdown dangerousInnerHTML={renderMarkdown(alert.message)} />
    const className = classNames(commonClassName, 'd-flex')

    if (alert.isDismissibleWithKey) {
        return (
            <DismissibleAlert
                partialStorageKey={`alert.${alert.isDismissibleWithKey}`}
                className={className}
                variant={alertVariantForType(alert.type)}
            >
                {content}
            </DismissibleAlert>
        )
    }
    return (
        <Alert className={className} variant={alertVariantForType(alert.type)}>
            {content}
        </Alert>
    )
}

function alertVariantForType(type: AlertType): DismissibleAlertProps['variant'] {
    switch (type) {
        case AlertType.INFO: {
            return 'info'
        }
        case AlertType.WARNING: {
            return 'warning'
        }
        case AlertType.ERROR: {
            return 'danger'
        }
        default: {
            return 'warning'
        }
    }
}
