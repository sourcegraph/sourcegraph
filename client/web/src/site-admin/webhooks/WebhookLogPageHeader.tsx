import classNames from 'classnames'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import React, { useCallback } from 'react'

import { useQuery } from '@sourcegraph/shared/src/graphql/graphql'
import { Button, Select } from '@sourcegraph/wildcard'

import { WebhookLogPageHeaderResult } from '../../graphql-operations'

import { SelectedExternalService, WEBHOOK_LOG_PAGE_HEADER } from './backend'
import { PerformanceGauge } from './PerformanceGauge'
import styles from './WebhookLogPageHeader.module.scss'

export interface Props {
    externalService: SelectedExternalService
    onlyErrors: boolean

    onSelectExternalService: (externalService: SelectedExternalService) => void
    onSetOnlyErrors: (onlyErrors: boolean) => void
}

export const WebhookLogPageHeader: React.FunctionComponent<Props> = ({
    externalService,
    onlyErrors,
    onSelectExternalService: onExternalServiceSelected,
    onSetOnlyErrors: onSetErrors,
}) => {
    const onErrorToggle = useCallback(() => onSetErrors(!onlyErrors), [onlyErrors, onSetErrors])
    const onSelect = useCallback(
        (value: string) => {
            onExternalServiceSelected(value)
        },
        [onExternalServiceSelected]
    )

    const { data } = useQuery<WebhookLogPageHeaderResult>(WEBHOOK_LOG_PAGE_HEADER, {})
    const errorCount = data?.webhookLogs.totalCount ?? 0

    return (
        <div className="d-flex align-items-end">
            <PerformanceGauge
                className="mr-3"
                count={data?.webhookLogs.totalCount}
                countClassName={errorCount > 0 ? 'text-danger' : undefined}
                label="recent error"
            />
            <PerformanceGauge count={data?.externalServices.totalCount} label="external service" />
            <div className="flex-fill" />
            <div className={styles.selectContainer}>
                <Select
                    aria-label="External service"
                    className="mb-0"
                    onChange={({ target: { value } }) => onSelect(value)}
                    value={externalService}
                >
                    <option key="all" value="all">
                        All webhooks
                    </option>
                    <option key="unmatched" value="unmatched">
                        Unmatched webhooks
                    </option>
                    {data?.externalServices.nodes.map(({ displayName, id }) => (
                        <option key={id} value={id}>
                            {displayName}
                        </option>
                    ))}
                </Select>
            </div>
            <div className="ml-3">
                <Button variant="danger" onClick={onErrorToggle} outline={!onlyErrors}>
                    <AlertCircleIcon className={classNames('icon-inline', styles.icon, onlyErrors && styles.enabled)} />
                    <span className="ml-1">Only errors</span>
                </Button>
            </div>
        </div>
    )
}
