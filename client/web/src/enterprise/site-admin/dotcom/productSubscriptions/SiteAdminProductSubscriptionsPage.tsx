import React, { useEffect, useState } from 'react'

import type { PartialMessage } from '@bufbuild/protobuf'
import { mdiPlus } from '@mdi/js'
import { QueryClientProvider } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useDebounce } from 'use-debounce'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Button, Container, Icon, Link, PageHeader } from '@sourcegraph/wildcard'

import {
    ConnectionContainer,
    ConnectionError,
    ConnectionForm,
    ConnectionList,
    ConnectionLoading,
} from '../../../../components/FilteredConnection/ui'
import { PageTitle } from '../../../../components/PageTitle'

import { queryClient, useListEnterpriseSubscriptions, type EnterprisePortalEnvironment } from './enterpriseportal'
import type { ListEnterpriseSubscriptionsFilter } from './enterpriseportalgen/subscriptions_pb'
import {
    SiteAdminProductSubscriptionNode,
    SiteAdminProductSubscriptionNodeHeader,
} from './SiteAdminProductSubscriptionNode'

interface Props extends TelemetryV2Props {}

/**
 * Displays the enterprise subscriptions (formerly known as "product subscriptions") that have been
 * created on Sourcegraph.com.
 */
export const SiteAdminProductSubscriptionsPage: React.FunctionComponent<React.PropsWithChildren<Props>> = props => (
    <QueryClientProvider client={queryClient}>
        <Page {...props} />
    </QueryClientProvider>
)

const QUERY_PARAM_KEY = 'query'
const QUERY_PARAM_ENV = 'env'
const QUERY_PARAM_FILTER = 'filter'

type FilterType = 'display_name' | 'sf_sub_id'

const MAX_RESULTS = 100

const Page: React.FunctionComponent<React.PropsWithChildren<Props>> = ({ telemetryRecorder }) => {
    useEffect(() => telemetryRecorder.recordEvent('admin.productSubscriptions', 'view'), [telemetryRecorder])

    const [searchParams, setSearchParams] = useSearchParams()

    const [query, setQuery] = useState<string>(searchParams.get(QUERY_PARAM_KEY) ?? '')
    const [filters, setFilters] = useState<{
        env: EnterprisePortalEnvironment
        filter: FilterType
    }>({
        env:
            (searchParams.get(QUERY_PARAM_ENV) as EnterprisePortalEnvironment) ?? window.context.deployType === 'dev'
                ? 'local'
                : 'prod',
        filter: (searchParams.get(QUERY_PARAM_FILTER) as FilterType) ?? 'display_name',
    })

    useEffect(() => {
        searchParams.set(QUERY_PARAM_KEY, query?.trim() ?? '')
        searchParams.set(QUERY_PARAM_ENV, filters.env)
        searchParams.set(QUERY_PARAM_FILTER, filters.filter)
        setSearchParams(searchParams)
    }, [query, searchParams, setSearchParams, filters])

    const [debouncedQuery] = useDebounce(query, 500)

    let listFilters: PartialMessage<ListEnterpriseSubscriptionsFilter>[] = []
    switch (filters.filter) {
        case 'display_name': {
            listFilters = [
                {
                    filter: {
                        case: 'displayName',
                        value: debouncedQuery,
                    },
                },
            ]
            break
        }
        case 'sf_sub_id': {
            listFilters = [
                {
                    filter: {
                        case: 'salesforce',
                        value: { subscriptionId: debouncedQuery },
                    },
                },
            ]
            break
        }
    }
    const { error, isFetching, data } = useListEnterpriseSubscriptions(filters.env, listFilters, {
        limit: MAX_RESULTS,
        // Only load when we have a query, and at least one filter
        shouldLoad: !!(debouncedQuery && listFilters.length > 0),
    })

    return (
        <div className="site-admin-product-subscriptions-page">
            <PageTitle title="Enterprise instance subscriptions" />
            <PageHeader
                headingElement="h2"
                path={[{ text: 'Enterprise instance subscriptions' }]}
                actions={
                    <Button to="./new" variant="primary" as={Link}>
                        <Icon aria-hidden={true} svgPath={mdiPlus} />
                        Create Enterprise subscription
                    </Button>
                }
                className="mb-3"
            />

            <ConnectionContainer>
                <Container className="mb-3">
                    <ConnectionForm
                        inputValue={query}
                        filterValues={filters}
                        filters={[
                            {
                                id: 'env',
                                type: 'select',
                                label: 'Environment',
                                options: [
                                    {
                                        label: 'Production',
                                        value: 'prod',
                                        args: {},
                                    },
                                    {
                                        label: 'Development',
                                        value: 'dev',
                                        args: {},
                                    },
                                ].concat(
                                    window.context.deployType === 'dev'
                                        ? [
                                              {
                                                  label: 'Local',
                                                  value: 'local',
                                                  args: {},
                                              },
                                          ]
                                        : []
                                ),
                            },
                            {
                                id: 'filter',
                                type: 'select',
                                label: 'Filter by',
                                options: [
                                    {
                                        args: {},
                                        label: 'Display name',
                                        value: 'display_name',
                                    },
                                    {
                                        args: {},
                                        label: 'Salesforce subscription ID',
                                        value: 'sf_sub_id',
                                    },
                                ],
                            },
                        ]}
                        onInputChange={event => {
                            setQuery(event.target.value)
                        }}
                        onFilterSelect={(filter, value) => {
                            setFilters({ ...filters, [filter.id]: value })
                        }}
                        inputPlaceholder="Enter a query to list subscriptions"
                    />
                </Container>

                {debouncedQuery && filters.filter && (
                    <>
                        <Container className="mb-3">
                            {error && <ConnectionError errors={[error.message]} />}
                            {data?.subscriptions && data?.subscriptions.length >= MAX_RESULTS && (
                                <ConnectionError
                                    errors={[
                                        `Only ${MAX_RESULTS} results are shown at a time - narrow your search for more accurate results.`,
                                    ]}
                                />
                            )}
                            {isFetching && <ConnectionLoading />}
                            {data && (
                                <ConnectionList as="table" aria-label="Enterprise subscriptions">
                                    <SiteAdminProductSubscriptionNodeHeader />
                                    <tbody>
                                        {data?.subscriptions?.map(node => (
                                            <SiteAdminProductSubscriptionNode
                                                key={node.id}
                                                node={node}
                                                env={filters.env}
                                            />
                                        ))}
                                    </tbody>
                                </ConnectionList>
                            )}
                        </Container>
                    </>
                )}
            </ConnectionContainer>
        </div>
    )
}
