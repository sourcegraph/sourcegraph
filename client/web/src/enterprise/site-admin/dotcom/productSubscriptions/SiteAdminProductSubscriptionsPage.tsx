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
import { EnterprisePortalEnvSelector, getDefaultEnterprisePortalEnv } from './EnterprisePortalEnvSelector'
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
    const [env, setEnv] = useState<EnterprisePortalEnvironment>(
        (searchParams.get(QUERY_PARAM_ENV) as EnterprisePortalEnvironment) || getDefaultEnterprisePortalEnv()
    )
    const [query, setQuery] = useState<string>(searchParams.get(QUERY_PARAM_KEY) ?? '')
    const [filters, setFilters] = useState<{ filter: FilterType }>({
        filter: (searchParams.get(QUERY_PARAM_FILTER) as FilterType) ?? 'display_name',
    })

    useEffect(() => {
        searchParams.set(QUERY_PARAM_KEY, query?.trim() ?? '')
        searchParams.set(QUERY_PARAM_ENV, env)
        searchParams.set(QUERY_PARAM_FILTER, filters.filter)
        setSearchParams(searchParams)
    }, [query, searchParams, setSearchParams, filters, env])

    const [debouncedQuery] = useDebounce(query, 200)

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
    const { error, isFetching, data } = useListEnterpriseSubscriptions(env, listFilters, {
        limit: MAX_RESULTS,
        // Only load when we have a query, and at least one filter
        shouldLoad: !!(debouncedQuery && listFilters.length > 0),
    })

    return (
        <div className="site-admin-product-subscriptions-page">
            <PageTitle title="Enterprise subscriptions" />
            <PageHeader
                headingElement="h2"
                path={[{ text: 'Enterprise subscriptions' }]}
                actions={
                    <div className="align-items-end d-flex">
                        <EnterprisePortalEnvSelector env={env} setEnv={setEnv} />
                        <div>
                            <Button to={`./new?env=${env}`} variant="primary" as={Link} display="block">
                                <Icon aria-hidden={true} svgPath={mdiPlus} />
                                Create Enterprise subscription
                            </Button>
                        </div>
                    </div>
                }
                className="mb-3"
            />

            <ConnectionContainer>
                <Container className="mb-3">
                    <ConnectionForm
                        inputValue={query}
                        filterValues={filters}
                        inputClassName="ml-2"
                        filters={[
                            {
                                id: 'filter',
                                type: 'select',
                                label: 'Filter',
                                options: [
                                    {
                                        args: {},
                                        label: 'Display name',
                                        value: 'display_name',
                                        tooltip: 'Partial, case-insensitive match on the subscription display name',
                                    },
                                    {
                                        args: {},
                                        label: 'Salesforce subscription ID',
                                        value: 'sf_sub_id',
                                        tooltip:
                                            'Exact match on the Salesforce subscription ID associated with the subscription',
                                    },
                                ],
                            },
                        ]}
                        onInputChange={event => {
                            setQuery(event.target.value)
                        }}
                        onFilterSelect={(filter, value) => {
                            if (value) {
                                setFilters({ ...filters, [filter.id]: value as FilterType })
                            }
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
                                            <SiteAdminProductSubscriptionNode key={node.id} node={node} env={env} />
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
