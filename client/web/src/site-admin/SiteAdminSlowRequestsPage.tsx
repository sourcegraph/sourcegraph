import React, { useCallback, useEffect, useState, type ReactNode } from 'react'

import { mdiChevronDown, mdiContentCopy } from '@mdi/js'
import classNames from 'classnames'
import copy from 'copy-to-clipboard'
import { map } from 'rxjs/operators'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { dataOrThrowErrors } from '@sourcegraph/http-client'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Code,
    Container,
    Icon,
    Link,
    PageHeader,
    Popover,
    PopoverContent,
    PopoverTrigger,
    Position,
    Text,
    Tooltip,
} from '@sourcegraph/wildcard'

import { requestGraphQL } from '../backend/graphql'
import {
    FilteredConnection,
    type Filter,
    type FilteredConnectionQueryArguments,
} from '../components/FilteredConnection'
import { PageTitle } from '../components/PageTitle'
import type { SlowRequestsResult, SlowRequestsVariables } from '../graphql-operations'

import { SLOW_REQUESTS } from './backend'

import styles from './SiteAdminSlowRequestsPage.module.scss'

export interface SiteAdminSlowRequestsPageProps extends TelemetryProps, TelemetryV2Props {}

type SlowRequest = SlowRequestsResult['slowRequests']['nodes'][0]

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

export const SiteAdminSlowRequestsPage: React.FunctionComponent<
    React.PropsWithChildren<SiteAdminSlowRequestsPageProps>
> = ({ telemetryService, telemetryRecorder }) => {
    useEffect(() => {
        telemetryService.logPageView('SiteAdminSlowRequests')
        telemetryRecorder.recordEvent('admin.slowRequests', 'view')
    }, [telemetryService, telemetryRecorder])

    const querySlowRequests = useCallback(
        (args: FilteredConnectionQueryArguments & { failed?: boolean }) =>
            requestGraphQL<SlowRequestsResult, SlowRequestsVariables>(SLOW_REQUESTS, {
                after: args.after ?? null,
            }).pipe(
                map(dataOrThrowErrors),
                map(data => {
                    data.slowRequests.nodes = data.slowRequests.nodes?.filter(
                        request =>
                            (!args.query || matchesString(request, args.query)) && args.failed !== isSuccessful(request)
                    )
                    return data.slowRequests
                })
            ),
        []
    )

    return (
        <div className="site-admin-slow-requests-page">
            <PageTitle title="Slow requests - Admin" />
            <PageHeader
                path={[{ text: 'Slow requests' }]}
                headingElement="h2"
                description={
                    <>
                        This is the log of recent slow GraphQL requests received by the Sourcegraph instance. Handy for
                        seeing what's happening between clients and our API.
                    </>
                }
                className="mb-3"
            />

            <Text>
                The <Icon aria-label="Copy cURL command" svgPath={mdiContentCopy} /> button will copy the GraphQL
                request as a cURL command in your clipboard. You will need to have $ACCESS_TOKEN set in your environment
                or to replace it in the copied command.
            </Text>

            <Text>
                Slow requests capture is configured through <Link to="/site-admin/configuration">site config</Link>:
            </Text>
            <ul>
                <li>
                    Minimum duration for a GraphQL request to be considered slow{' '}
                    <strong>observability.logSlowGraphQLRequests</strong>
                </li>
                <li>
                    Maximum count of captured requests to keep{' '}
                    <strong>observability.captureSlowGraphQLRequestsLimit</strong>
                </li>
            </ul>

            <Container className="mb-3">
                <FilteredConnection<SlowRequest>
                    className="mb-0"
                    listComponent="div"
                    listClassName={classNames('list-group mb-3', styles.requestsGrid)}
                    noun="request"
                    pluralNoun="requests"
                    queryConnection={querySlowRequests}
                    nodeComponent={SlowRequestNode}
                    filters={filters}
                    cursorPaging={true}
                />
            </Container>
        </div>
    )
}

const SlowRequestNode: React.FunctionComponent<{ node: React.PropsWithChildren<SlowRequest> }> = ({ node }) => {
    const roundedSecond = Number(node.duration.toFixed(2))
    const [copied, setCopied] = useState(false)

    const copyToClipboard = (text: string): void => {
        copy(text)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
    }

    return (
        <React.Fragment key={node.index}>
            <span className={styles.separator} />
            <div className="flex-bounded">
                <Timestamp date={node.start} noAbout={true} strict={true} />
            </div>
            <div>
                <Tooltip content={'Repo Name: ' + (node.repository?.name || '')}>
                    <Code>{node.repository?.name ? shortenRepoName(node.repository?.name) : ''}</Code>
                </Tooltip>
            </div>
            <div>
                <Tooltip content="Duration">
                    <span>
                        {roundedSecond.toFixed(2)} second{roundedSecond === 1 ? '' : 's'}
                    </span>
                </Tooltip>
            </div>
            <div>
                <Tooltip content="Request Name">
                    <strong>{node.name}</strong>
                </Tooltip>
            </div>
            <div>
                <Tooltip content={'Filepath: ' + (node?.filepath || '')}>
                    <Code>{node.filepath ? ellipsis(node.filepath, 30) : ''}</Code>
                </Tooltip>
            </div>
            <div className={classNames('d-flex flex-row')}>
                <div>
                    <Tooltip
                        content={
                            copied ? 'cURL command copied' : 'Copy cURL command (remember to set $SRC_ACCESS_TOKEN)'
                        }
                    >
                        <Button className="ml-2" onClick={() => copyToClipboard(buildCurlCommand(node))}>
                            <Icon aria-label="Copy cURL command" svgPath={mdiContentCopy} />
                        </Button>
                    </Tooltip>
                </div>
                <SimplePopover label="More info">
                    <small className={styles.moreInfo}>
                        <Text>
                            <strong>Name: </strong>
                            {node.name}
                        </Text>
                        <Text>
                            <strong>User: </strong>
                            {node.user?.username}
                        </Text>
                        <Text>
                            <strong>Date/time started: </strong>
                            <Timestamp date={node.start} preferAbsolute={true} noAbout={true} />
                        </Text>
                        <Text>
                            <strong>Duration: </strong>
                            {roundedSecond.toFixed(2)} second{roundedSecond === 1 ? '' : 's'}
                        </Text>
                        <Text>
                            <strong>Errors: </strong>
                            {node.errors.length > 0 ? node.errors : 'none'}
                        </Text>
                        <Text>
                            <strong>Variables: </strong>
                            <pre>{node.variables}</pre>
                        </Text>
                        <Text>
                            <strong>Query: </strong>
                            <pre>{node.query}</pre>
                        </Text>
                    </small>
                </SimplePopover>
            </div>
        </React.Fragment>
    )
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

function isSuccessful(request: SlowRequest): boolean {
    return request.errors.length === 0
}

function matchesString(request: SlowRequest, query: string): boolean {
    const lQuery = query.toLowerCase()
    return (
        request.name.toLowerCase().includes(lQuery) ||
        request.repository?.name?.toLowerCase().includes(lQuery) ||
        request.errors.some(error => error.toLowerCase().includes(lQuery), false)
    )
}

function ellipsis(str: string, len: number): string {
    if (str.length <= len) {
        return str
    }
    return '...' + str.slice(str.length - len, str.length)
}

function shortenRepoName(repoName: string): string {
    return repoName?.split('/').at(-1) || ''
}

function buildCurlCommand(request: SlowRequest): string {
    const headers = "-H 'Authorization: token $SRC_ACCESS_TOKEN'"
    const body = `{"query": "${request.query}", "variables": ${request.variables}}`.replaceAll('\n', '')
    return `curl -X POST ${headers} -d '${body}' '${window.location.protocol}//${window.location.host}/.api/graphql?${request.name}'`
}
