import React, { useEffect, useState } from 'react'

import { useSearchParams } from 'react-router-dom'

import { Container, PageHeader } from '@sourcegraph/wildcard'

import {
    ConnectionContainer,
    ConnectionError,
    ConnectionLoading,
    ConnectionList,
    SummaryContainer,
    ConnectionSummary,
    ShowMoreButton,
    ConnectionForm,
} from '../../../../components/FilteredConnection/ui'
import { PageTitle } from '../../../../components/PageTitle'
import { eventLogger } from '../../../../tracking/eventLogger'

import { useQueryProductLicensesConnection } from './backend'
import { SiteAdminProductLicenseNode } from './SiteAdminProductLicenseNode'

interface Props {}

const SEARCH_PARAM_KEY = 'query'

/**
 * Displays the product licenses that have been created on Sourcegraph.com.
 */
export const SiteAdminLicenseKeyLookupPage: React.FunctionComponent<React.PropsWithChildren<Props>> = () => {
    useEffect(() => eventLogger.logPageView('SiteAdminLicenseKeyLookup'), [])

    const [searchParams, setSearchParams] = useSearchParams()

    const [search, setSearch] = useState<string>(searchParams.get(SEARCH_PARAM_KEY) ?? '')

    const { loading, hasNextPage, fetchMore, refetchAll, connection, error } = useQueryProductLicensesConnection(
        search,
        20
    )

    useEffect(() => {
        const query = search?.trim() ?? ''
        searchParams.set(SEARCH_PARAM_KEY, query)
        setSearchParams(searchParams)
    }, [search, searchParams, setSearchParams])

    return (
        <div className="site-admin-product-subscriptions-page">
            <PageTitle title="Enterprise subscriptions" />
            <PageHeader
                path={[{ text: 'License key lookup' }]}
                headingElement="h2"
                description="Find matching licenses and their associated enterprise subscriptions"
                className="mb-3"
            />
            <ConnectionContainer>
                <Container className="mb-3">
                    <ConnectionForm
                        inputValue={search}
                        onInputChange={event => {
                            const search = event.target.value
                            setSearch(search)
                        }}
                        inputPlaceholder="Enter a partial license key to find matches"
                        inputClassName="mb-0"
                        formClassName="mb-0"
                    />
                </Container>
                {search && (
                    <>
                        <Container className="mb-3">
                            {error && <ConnectionError errors={[error.message]} />}
                            {loading && !connection && <ConnectionLoading />}
                            <ConnectionList
                                as="ul"
                                className="list-group list-group-flush mb-0"
                                aria-label="Subscription licenses"
                            >
                                {connection?.nodes?.map(node => (
                                    <SiteAdminProductLicenseNode
                                        key={node.id}
                                        node={node}
                                        showSubscription={true}
                                        onRevokeCompleted={refetchAll}
                                    />
                                ))}
                            </ConnectionList>
                            {connection && (
                                <SummaryContainer className="mt-2 mb-0">
                                    <ConnectionSummary
                                        first={15}
                                        centered={true}
                                        connection={connection}
                                        noun="product license"
                                        pluralNoun="product licenses"
                                        hasNextPage={hasNextPage}
                                        noSummaryIfAllNodesVisible={true}
                                        emptyElement={
                                            <div className="w-100 text-center text-muted">
                                                No matching license key found
                                            </div>
                                        }
                                        className="mb-0"
                                    />
                                    {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                                </SummaryContainer>
                            )}
                        </Container>
                    </>
                )}
            </ConnectionContainer>
        </div>
    )
}
