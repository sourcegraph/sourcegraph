import React, { useCallback, useEffect, useState } from 'react'

import { mdiChevronDown } from '@mdi/js'
import classNames from 'classnames'
import { RouteComponentProps } from 'react-router'
import { of } from 'rxjs'
import { map } from 'rxjs/operators'

import { ErrorAlert } from '@sourcegraph/branded/src/components/alerts'
import { useQuery } from '@sourcegraph/http-client/src'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Container,
    H3,
    Icon,
    Link,
    LoadingSpinner,
    PageHeader,
    Popover,
    PopoverContent,
    PopoverTrigger,
    Position,
    Text,
} from '@sourcegraph/wildcard'

import {
    FilteredConnection,
    FilteredConnectionFilter,
    FilteredConnectionQueryArguments,
} from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import { Timestamp } from '../components/time/Timestamp'
import { OutboundRequestsResult, OutboundRequestsVariables } from '../graphql-operations'

import { OUTBOUND_REQUESTS, OUTBOUND_REQUESTS_PAGE_POLL_INTERVAL } from './backend'

import styles from './SiteAdminOutboundRequestsPage.module.scss'

export interface SiteAdminOutboundRequestsPageProps extends RouteComponentProps, TelemetryProps {
    now?: () => Date
}

export type OutboundRequest = OutboundRequestsResult['outboundRequests'][0]

export const SiteAdminOutboundRequestsPage: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminOutboundRequestsPageProps>
> = ({ history, location, telemetryService }) => {
    const [items, setItems] = useState<OutboundRequest[]>([])
    // const [previousData, setPreviousData] = useState<OutboundRequestsResult | null>(null)
    useEffect(() => {
        telemetryService.logPageView('SiteAdminOutboundRequests')
    }, [telemetryService])

    const lastKey = items[items.length - 1]?.key ?? null
    const { data, loading, error, stopPolling, refetch, startPolling } = useQuery<
        OutboundRequestsResult,
        OutboundRequestsVariables
    >(OUTBOUND_REQUESTS, {
        variables: { lastKey },
        pollInterval: OUTBOUND_REQUESTS_PAGE_POLL_INTERVAL,
    })

    if (data?.outboundRequests?.length && (!lastKey || data?.outboundRequests[0].key > lastKey)) {
        const newItems = items
            .concat(...data.outboundRequests)
            .slice(Math.max(items.length + data.outboundRequests.length - 50, 0))
        stopPolling()
        setItems(newItems)
        refetch({ lastKey: newItems[newItems.length - 1]?.key ?? null })
            .then(() => {})
            .catch(() => {})
        startPolling(OUTBOUND_REQUESTS_PAGE_POLL_INTERVAL)
    }

    const queryOutboundRequests = useCallback(
        (args: FilteredConnectionQueryArguments & { failed?: boolean }) =>
            of([...items].reverse()).pipe(
                map(items => {
                    const filtered = items?.filter(
                        request =>
                            (!args.query || request.url.includes(args.query)) &&
                            (args.failed !== false || request.statusCode < 400) &&
                            (args.failed !== true || request.statusCode >= 400)
                    )
                    return {
                        nodes: filtered ?? [],
                        totalCount: (filtered ?? []).length,
                    }
                })
            ),
        [items]
    )

    const filters: FilteredConnectionFilter[] = [
        {
            id: 'filters',
            label: 'Filter by success',
            type: 'select',
            values: [
                {
                    label: 'All',
                    value: 'all',
                    tooltip: 'Show all requests',
                    args: {},
                },
                {
                    label: 'Failed',
                    value: 'failed',
                    tooltip: 'Show only failed requests',
                    args: { failed: true },
                },
                {
                    label: 'Successful',
                    value: 'successful',
                    tooltip: 'Show only successful requests',
                    args: { failed: false },
                },
            ],
        },
    ]

    return (
        <div className="site-admin-migrations-page">
            <PageTitle title="Outbound requests - Admin" />
            <PageHeader
                path={[{ text: 'Outbound requests' }]}
                headingElement="h2"
                description={
                    <>
                        This is the log of recent external requests sent by the Sourcegraph instance. Handy for seeing
                        what's happening between Sourcegraph and other services.{' '}
                        <strong>The list updates every five seconds.</strong>
                    </>
                }
                className="mb-3"
            />

            <Container className="mb-3">
                {error && !loading && <ErrorAlert error={error} />}
                {loading && !error && <LoadingSpinner />}
                {window.context.outboundRequestLogLimit ? (
                    <FilteredConnection<OutboundRequest>
                        className="mb-0"
                        listComponent="div"
                        listClassName={classNames('list-group mb-3', styles.requestsGrid)}
                        noun="request"
                        pluralNoun="requests"
                        queryConnection={queryOutboundRequests}
                        nodeComponent={MigrationNode}
                        filters={filters}
                        history={history}
                        location={location}
                    />
                ) : (
                    <>
                        <Text>Outbound request logging is currently disabled.</Text>
                        <Text>
                            Set `outboundRequestLogLimit` to a non-zero value in your{' '}
                            <Link to="/site-admin/configuration">site config</Link> to enable it.
                        </Text>
                    </>
                )}
            </Container>
        </div>
    )
}

const MigrationNode: React.FunctionComponent<{ node: React.PropsWithChildren<OutboundRequest> }> = ({ node }) => {
    const roundedSecond = Math.round((node.duration + Number.EPSILON) * 100) / 100
    return (
        <React.Fragment key={node.key}>
            <span className={styles.separator} />
            <div className={classNames('d-flex flex-column', styles.progress)}>
                <Text>
                    <Timestamp date={node.startedAt} noAbout={true} />
                    <br />
                    Method: <strong>{node.method}</strong>
                    <br />
                    Status: <strong>{node.statusCode}</strong>
                    <br />
                    Took{' '}
                    <strong>
                        {roundedSecond} second{roundedSecond === 1 ? '' : 's'}
                    </strong>
                    .
                </Text>
            </div>
            <div className={classNames('d-flex flex-column', styles.information)}>
                <div>
                    <H3>{node.url}</H3>

                    <Text className="m-0 text-muted">
                        <span>
                            <HeaderPopover
                                headers={node.requestHeaders}
                                label={`Req headers (${node.requestHeaders?.length})`}
                            />
                        </span>{' '}
                        <span>
                            <StringPopover value={node.requestBody} label="Request body" />
                        </span>{' '}
                        <span>
                            <HeaderPopover
                                headers={node.responseHeaders}
                                label={`Resp headers (${node.responseHeaders?.length})`}
                            />
                        </span>{' '}
                        <span>
                            <StringPopover value={node.requestBody} label="Error message" />
                        </span>{' '}
                        <span>
                            <StringPopover value={node.creationStackFrame} label="Creation stack trace" />
                        </span>{' '}
                        <span>
                            <StringPopover value={node.callStackFrame} label="Call stack trace" />
                        </span>
                    </Text>
                </div>
            </div>
        </React.Fragment>
    )
}

const HeaderPopover: React.FunctionComponent<
    React.PropsWithChildren<{ headers: OutboundRequest['requestHeaders'] | undefined; label: string }>
> = ({ headers, label }) => {
    const [isOpen, setIsOpen] = useState(false)
    const handleOpenChange = useCallback(({ isOpen }: { isOpen: boolean }) => setIsOpen(isOpen), [setIsOpen])
    return headers ? (
        <Popover isOpen={isOpen} onOpenChange={handleOpenChange}>
            <PopoverTrigger as={Button} variant="secondary" outline={true}>
                {label} <Icon aria-label="Show details" svgPath={mdiChevronDown} />
            </PopoverTrigger>
            <PopoverContent position={Position.bottom} focusLocked={false}>
                <div className="p-2">
                    <div className="d-flex flex-column">
                        {headers.map(header => (
                            <div key={header.name}>
                                <strong>{header.name}</strong>: {header.values.join(', ')}
                            </div>
                        ))}
                    </div>
                </div>
            </PopoverContent>
        </Popover>
    ) : (
        <></>
    )
}

const StringPopover: React.FunctionComponent<{ value: string; label: string }> = ({ value, label }) => {
    const [isOpen, setIsOpen] = useState(false)
    const handleOpenChange = useCallback(({ isOpen }: { isOpen: boolean }) => setIsOpen(isOpen), [setIsOpen])
    return value ? (
        <Popover isOpen={isOpen} onOpenChange={handleOpenChange}>
            <PopoverTrigger as={Button} variant="secondary" outline={true}>
                {`${label} (length: ${value.length} chars)`} <Icon aria-label="Show details" svgPath={mdiChevronDown} />
            </PopoverTrigger>
            <PopoverContent position={Position.bottom} focusLocked={false}>
                <div className="p-2">
                    <div className="d-flex flex-column">
                        <div>{value}</div>
                    </div>
                </div>
            </PopoverContent>
        </Popover>
    ) : (
        <>{label}: (empty)</>
    )
}
