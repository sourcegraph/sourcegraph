import React, { useEffect, useState } from 'react'

import type { PartialMessage } from '@bufbuild/protobuf'
import { QueryClientProvider } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useDebounce } from 'use-debounce'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Container, PageHeader } from '@sourcegraph/wildcard'

import {
    ConnectionContainer,
    ConnectionError,
    ConnectionForm,
    ConnectionList,
    ConnectionLoading,
} from '../../../../components/FilteredConnection/ui'
import { PageTitle } from '../../../../components/PageTitle'

import {
    queryClient,
    useListEnterpriseSubscriptionLicenses,
    type EnterprisePortalEnvironment,
} from './enterpriseportal'
import { EnterprisePortalEnvSelector, getDefaultEnterprisePortalEnv } from './EnterprisePortalEnvSelector'
import {
    type ListEnterpriseSubscriptionLicensesFilter,
    EnterpriseSubscriptionLicenseType,
} from './enterpriseportalgen/subscriptions_pb'
import { SiteAdminProductLicenseNode } from './SiteAdminProductLicenseNode'

interface Props extends TelemetryV2Props {}

/**
 * Displays the product licenses that have been created on Sourcegraph.com.
 */
export const SiteAdminLicenseKeyLookupPage: React.FunctionComponent<React.PropsWithChildren<Props>> = props => (
    <QueryClientProvider client={queryClient}>
        <Page {...props} />
    </QueryClientProvider>
)

const QUERY_PARAM_KEY = 'query'
const QUERY_PARAM_ENV = 'env'
const QUERY_PARAM_FILTER = 'filter'

type FilterType = 'key_substring' | 'sf_opp_id'

const MAX_RESULTS = 100

const baseFilters: PartialMessage<ListEnterpriseSubscriptionLicensesFilter>[] = [
    {
        filter: {
            // This UI only manages old-school license keys.
            case: 'type',
            value: EnterpriseSubscriptionLicenseType.KEY,
        },
    },
]

const Page: React.FunctionComponent<React.PropsWithChildren<Props>> = ({ telemetryRecorder }) => {
    useEffect(() => telemetryRecorder.recordEvent('admin.licenseKeyLookup', 'view'), [telemetryRecorder])

    const [searchParams, setSearchParams] = useSearchParams()
    const [env, setEnv] = useState<EnterprisePortalEnvironment>(
        (searchParams.get(QUERY_PARAM_ENV) as EnterprisePortalEnvironment) || getDefaultEnterprisePortalEnv()
    )
    const [query, setQuery] = useState<string>(searchParams.get(QUERY_PARAM_KEY) ?? '')
    const [debouncedQuery] = useDebounce(query, 200)

    const [filters, setFilters] = useState<{
        filter: FilterType
    }>({
        filter: (searchParams.get(QUERY_PARAM_FILTER) as FilterType) ?? 'key_substring',
    })

    useEffect(() => {
        searchParams.set(QUERY_PARAM_KEY, query?.trim() ?? '')
        searchParams.set(QUERY_PARAM_ENV, env)
        searchParams.set(QUERY_PARAM_FILTER, filters.filter)
        setSearchParams(searchParams)
    }, [query, searchParams, setSearchParams, filters, env])

    let listFilters: PartialMessage<ListEnterpriseSubscriptionLicensesFilter>[] = []
    switch (filters.filter) {
        case 'key_substring': {
            listFilters = [
                {
                    filter: {
                        case: 'licenseKeySubstring',
                        value: debouncedQuery,
                    },
                },
            ]
            break
        }
        case 'sf_opp_id': {
            listFilters = [
                {
                    filter: {
                        case: 'salesforceOpportunityId',
                        value: debouncedQuery,
                    },
                },
            ]
            break
        }
    }
    const { error, isFetching, data, refetch } = useListEnterpriseSubscriptionLicenses(
        env,
        baseFilters.concat(listFilters),
        {
            limit: MAX_RESULTS,
            // Only load when we have a query, and at least one filter
            shouldLoad: !!(debouncedQuery && listFilters.length > 0),
        }
    )

    return (
        <div className="site-admin-product-subscriptions-page">
            <PageTitle title="Enterprise subscriptions" />
            <PageHeader
                path={[{ text: 'License key lookup' }]}
                headingElement="h2"
                description="Find matching licenses and their associated enterprise subscriptions"
                className="mb-3"
                actions={<EnterprisePortalEnvSelector env={env} setEnv={setEnv} />}
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
                                label: 'Filter by',
                                options: [
                                    {
                                        args: {},
                                        label: 'License key substring',
                                        value: 'key_substring',
                                    },
                                    {
                                        args: {},
                                        label: 'Salesforce opportunity ID',
                                        value: 'sf_opp_id',
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
                            {isFetching && <ConnectionLoading />}
                            {data?.licenses && data?.licenses.length >= MAX_RESULTS && (
                                <ConnectionError
                                    errors={[
                                        `Only ${MAX_RESULTS} results are shown at a time - narrow your search for more accurate results.`,
                                    ]}
                                />
                            )}
                            {data && (
                                <ConnectionList as="ul" aria-label="Enterprise subscription licenses">
                                    {data?.licenses?.map(node => (
                                        <SiteAdminProductLicenseNode
                                            key={node.id}
                                            env={env}
                                            node={node}
                                            showSubscription={true}
                                            onRevokeCompleted={refetch}
                                            telemetryRecorder={telemetryRecorder}
                                        />
                                    ))}
                                </ConnectionList>
                            )}
                        </Container>
                    </>
                )}
            </ConnectionContainer>
        </div>
    )
}
