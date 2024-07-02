import React, { useCallback, useEffect, useState, type ReactNode } from 'react'

import { mdiChevronDown } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'
import copy from 'copy-to-clipboard'
import { of } from 'rxjs'
import { delay, map } from 'rxjs/operators'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { useQuery } from '@sourcegraph/http-client/src'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Code,
    Container,
    ErrorAlert,
    Icon,
    Link,
    LoadingSpinner,
    PageHeader,
    Popover,
    PopoverContent,
    PopoverTrigger,
    Position,
    Text,
    Tooltip,
} from '@sourcegraph/wildcard'

import {
    FilteredConnection,
    type Filter,
    type FilteredConnectionQueryArguments,
} from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import type { OutboundRequestsResult, OutboundRequestsVariables } from '../graphql-operations'

import { OUTBOUND_REQUESTS, OUTBOUND_REQUESTS_PAGE_POLL_INTERVAL_MS } from './backend'
import { parseProductReference } from './SiteAdminFeatureFlagsPage'

import styles from './SiteAdminOutboundRequestsPage.module.scss'

export interface SiteAdminOutboundRequestsPageProps extends TelemetryProps, TelemetryV2Props {}

export type OutboundRequest = OutboundRequestsResult['outboundRequests']['nodes'][0]

const filters: Filter[] = [
    {
        id: 'filters',
        label: 'Filter',
        type: 'select',
        options: [
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

export const SiteAdminOutboundRequestsPage: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminOutboundRequestsPageProps>
> = ({ telemetryService, telemetryRecorder }) => {
    const [items, setItems] = useState<OutboundRequest[]>([])

    useEffect(() => {
        telemetryService.logPageView('SiteAdminOutboundRequests')
        telemetryRecorder.recordEvent('admin.outboundRequests', 'view')
    }, [telemetryService, telemetryRecorder])

    const lastId = items.at(-1)?.id ?? null
    const { data, loading, error, stopPolling, refetch, startPolling } = useQuery<
        OutboundRequestsResult,
        OutboundRequestsVariables
    >(OUTBOUND_REQUESTS, {
        variables: { after: lastId },
        pollInterval: OUTBOUND_REQUESTS_PAGE_POLL_INTERVAL_MS,
    })
    const [polling, setPolling] = useState(true)
    const togglePolling = useCallback(() => {
        if (polling) {
            stopPolling()
        } else {
            startPolling(OUTBOUND_REQUESTS_PAGE_POLL_INTERVAL_MS)
        }
        setPolling(!polling)
    }, [polling, startPolling, stopPolling])

    useEffect(() => {
        if (data?.outboundRequests?.nodes?.length && (!lastId || data?.outboundRequests.nodes[0].id > lastId)) {
            const newItems = items
                .concat(...data.outboundRequests.nodes)
                .slice(
                    Math.max(
                        items.length +
                            data.outboundRequests.nodes.length -
                            (window.context.outboundRequestLogLimit ?? 0),
                        0
                    )
                )
            // Workaround for https://github.com/apollographql/apollo-client/issues/3053 to update the variables.
            // Weirdly enough, we don't need to wait for refetch() to complete before restarting polling.
            // See http://www.petecorey.com/blog/2019/09/23/apollo-quirks-polling-after-refetching-with-new-variables/
            stopPolling()
            setItems(newItems)
            refetch({ after: newItems.at(-1)?.id ?? null })
                .then(() => {})
                .catch(() => {})
            startPolling(OUTBOUND_REQUESTS_PAGE_POLL_INTERVAL_MS)
        }
    }, [data, lastId, items, refetch, startPolling, stopPolling])

    const queryOutboundRequests = useCallback(
        (args: FilteredConnectionQueryArguments & { failed?: boolean }) =>
            of([...items].reverse()).pipe(
                delay(200), // Without this, FilteredConnection will get into an infinite loop. :facepalm:
                map(items => {
                    const filtered = items?.filter(
                        request =>
                            (!args.query || matchesString(request, args.query)) &&
                            (args.failed !== false || isSuccessful(request)) &&
                            (args.failed !== true || !isSuccessful(request))
                    )
                    return {
                        nodes: filtered ?? [],
                        totalCount: (filtered ?? []).length,
                    }
                })
            ),
        [items]
    )

    return (
        <div className="site-admin-outbound-requests-page">
            <PageTitle title="Outbound requests - Admin" />
            <Button variant="secondary" onClick={togglePolling} className="float-right">
                {polling ? 'Pause updating' : 'Resume updating'}
            </Button>
            <PageHeader
                path={[{ text: 'Outbound requests' }]}
                headingElement="h2"
                description={
                    <>
                        This is the log of recent external requests sent by the Sourcegraph instance. Handy for seeing
                        what's happening between Sourcegraph and other services.{' '}
                        {polling ? <strong>The list updates every five seconds.</strong> : null}
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
                        nodeComponent={OutboundRequestNode}
                        filters={filters}
                    />
                ) : (
                    <>
                        <Text>Outbound request logging is currently disabled.</Text>
                        <Text>
                            Set <Code>outboundRequestLogLimit</Code> to a non-zero value in your{' '}
                            <Link to="/site-admin/configuration">site config</Link> to enable it.
                        </Text>
                    </>
                )}
            </Container>
        </div>
    )
}

const OutboundRequestNode: React.FunctionComponent<{ node: React.PropsWithChildren<OutboundRequest> }> = ({ node }) => {
    const [copied, setCopied] = useState(false)

    const copyToClipboard = (text: string): void => {
        copy(text)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    return (
        <React.Fragment key={node.id}>
            <span className={styles.separator} />
            <div className="flex-bounded">
                <Timestamp date={node.startedAt} noAbout={true} />
            </div>
            <div>
                <Tooltip content="HTTP request method">
                    <span>
                        <VisuallyHidden>Request method</VisuallyHidden>
                        <span
                            className={classNames(
                                styles.method,
                                styles[node.method.toLowerCase() as keyof typeof styles]
                            )}
                            aria-hidden={true}
                        >
                            ●
                        </span>{' '}
                        {node.method}
                    </span>
                </Tooltip>
            </div>
            <div>
                <Tooltip content="HTTP response status code">
                    <span className={isSuccessful(node) ? styles.successful : styles.failed}>
                        <VisuallyHidden>Status code</VisuallyHidden>
                        {node.statusCode}
                    </span>
                </Tooltip>
            </div>
            <div className={styles.urlColumn}>{node.url}</div>
            <div className={classNames('d-flex flex-row')}>
                <SimplePopover label="More info">
                    <small className={styles.moreInfo}>
                        <Text>
                            <strong>URL: </strong>
                            {node.url}
                        </Text>
                        <Text>
                            <strong>Status: </strong>
                            {node.statusCode}
                        </Text>
                        <Text>
                            <strong>Date/time started: </strong>
                            <Timestamp date={node.startedAt} preferAbsolute={true} noAbout={true} />
                        </Text>
                        <Text>
                            <strong>Duration: </strong>
                            {(node.durationMs / 1000).toFixed(2)} second{node.durationMs === 1000 ? '' : 's'}
                        </Text>
                        <Text>
                            <strong>Client created at: </strong>
                            <Code>{formatStackFrameLine(node.creationStackFrame)}</Code>
                        </Text>
                        <Text>
                            <strong>Request made at: </strong>
                        </Text>
                        {formatStackFrame(node.callStack)}
                        <Text>
                            <strong>Error: </strong>
                            {node.errorMessage ? node.errorMessage : 'No error'}
                        </Text>
                        {node.requestHeaders.length ? (
                            <>
                                <Text>
                                    <strong>Request headers:</strong>{' '}
                                </Text>
                                <ul>
                                    {[...node.requestHeaders]
                                        .sort((a, b) => a.name.localeCompare(b.name))
                                        .map(header => (
                                            <li key={header.name}>
                                                <strong>{header.name}</strong>: {header.values.join(', ')}
                                            </li>
                                        ))}
                                </ul>
                            </>
                        ) : (
                            'No request headers'
                        )}
                        {node.responseHeaders.length ? (
                            <>
                                <Text>
                                    <strong>Response headers:</strong>{' '}
                                </Text>
                                <ul>
                                    {[...node.responseHeaders]
                                        .sort((a, b) => a.name.localeCompare(b.name))
                                        .map(header => (
                                            <li key={header.name}>
                                                <strong>{header.name}</strong>: {header.values.join(', ')}
                                            </li>
                                        ))}
                                </ul>
                            </>
                        ) : (
                            'No request headers'
                        )}
                        <Text>
                            <strong>Request body:</strong> {node.requestBody ? node.requestBody : 'Empty body'}
                        </Text>
                    </small>
                </SimplePopover>
            </div>
            <div>
                <Tooltip content={copied ? 'Curl command copied' : 'Copy curl command (may contain REDACTED fields!)'}>
                    <Button className="ml-2" onClick={() => copyToClipboard(buildCurlCommand(node))}>
                        Copy curl
                    </Button>
                </Tooltip>
            </div>
        </React.Fragment>
    )
}

function formatStackFrame(callStack: string): React.ReactNode {
    const lines = callStack.split('\n')

    return (
        <>
            <ul>{lines.map(formatStackFrameLine)}</ul>
        </>
    )
}

function formatStackFrameLine(line: string): React.ReactNode {
    const match = line.match(/(.*):(\d+) \(Function: (.*)\)/)
    if (!match) {
        return line
    }
    const [, fileName, lineIndex, functionName] = match
    return (
        <li key={`${fileName}:${lineIndex}`}>
            <Code>
                <Link to={buildSourcegraphUrl(fileName, parseInt(lineIndex, 10))} target="_blank" rel="noopener">
                    {fileName}:{lineIndex}
                </Link>{' '}
                (Function: {functionName})
            </Code>
        </li>
    )
}

function buildSourcegraphUrl(fileName: string, lineIndex: number): string {
    const revision = parseProductReference(window.context.version)
    return `https://sourcegraph.com/github.com/sourcegraph/sourcegraph@${revision}/-/blob/${fileName}?L${lineIndex}`
}

const SimplePopover: React.FunctionComponent<{ label: string; children: ReactNode }> = ({ label, children }) => {
    const [isOpen, setIsOpen] = useState(false)
    const handleOpenChange = useCallback(({ isOpen }: { isOpen: boolean }) => setIsOpen(isOpen), [setIsOpen])
    return (
        <Popover isOpen={isOpen} onOpenChange={handleOpenChange}>
            <PopoverTrigger as={Button} variant="secondary" outline={true}>
                <small>{label}</small>
                <Icon aria-label="Show details" svgPath={mdiChevronDown} />
            </PopoverTrigger>
            <PopoverContent position={Position.bottom} focusLocked={false}>
                <div className="p-2">{children}</div>
            </PopoverContent>
        </Popover>
    )
}

function isSuccessful(request: OutboundRequest): boolean {
    return request.statusCode < 400
}

function matchesString(request: OutboundRequest, query: string): boolean {
    const lQuery = query.toLowerCase()
    return (
        request.url.toLowerCase().includes(lQuery) ||
        request.method.toLowerCase().includes(lQuery) ||
        request.requestBody.toLowerCase().includes(lQuery) ||
        request.statusCode.toString().includes(lQuery) ||
        request.errorMessage.toLowerCase().includes(lQuery) ||
        request.creationStackFrame.toLowerCase().includes(lQuery) ||
        request.callStack.toLowerCase().includes(lQuery) ||
        request.requestHeaders?.some(
            header =>
                header.name.toLowerCase().includes(lQuery) ||
                header.values.some(value => value.toLowerCase().includes(lQuery))
        ) ||
        request.responseHeaders?.some(
            header =>
                header.name.toLowerCase().includes(lQuery) ||
                header.values.some(value => value.toLowerCase().includes(lQuery))
        )
    )
}

function buildCurlCommand(request: OutboundRequest): string {
    const headers = request.requestHeaders?.map(header => `-H '${header.name}: ${header.values.join(', ')}'`).join(' ')
    const body = request.requestBody ? `-d '${request.requestBody}'` : ''
    return `curl -X ${request.method} ${headers} ${body} '${request.url}'`
}
