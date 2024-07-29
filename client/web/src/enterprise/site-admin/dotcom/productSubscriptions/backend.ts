import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'

import {
    useShowMorePagination,
    type UseShowMorePaginationResult,
} from '../../../../components/FilteredConnection/hooks/useShowMorePagination'
import type {
    DotComProductLicensesResult,
    DotComProductLicensesVariables,
    ProductLicenseFields,
    ProductLicensesResult,
    ProductLicensesVariables,
} from '../../../../graphql-operations'

export const DOTCOM_PRODUCT_SUBSCRIPTION = gql`
    query DotComProductSubscription($uuid: String!) {
        dotcom {
            productSubscription(uuid: $uuid) {
                id
                name
                account {
                    id
                    username
                    displayName
                }
                productLicenses {
                    nodes {
                        id
                        info {
                            tags
                            userCount
                            expiresAt
                            salesforceSubscriptionID
                            salesforceOpportunityID
                        }
                        licenseKey
                        createdAt
                        revokedAt
                        revokeReason
                        siteID
                        version
                    }
                    totalCount
                    pageInfo {
                        hasNextPage
                    }
                }
                activeLicense {
                    id
                    info {
                        productNameWithBrand
                        tags
                        userCount
                        expiresAt
                        salesforceSubscriptionID
                        salesforceOpportunityID
                    }
                    licenseKey
                    createdAt
                }
                currentSourcegraphAccessToken
                createdAt
                isArchived
                url
            }
        }
    }
`

export const ARCHIVE_PRODUCT_SUBSCRIPTION = gql`
    mutation ArchiveProductSubscription($id: ID!) {
        dotcom {
            archiveProductSubscription(id: $id) {
                alwaysNil
            }
        }
    }
`

export const GENERATE_PRODUCT_LICENSE = gql`
    mutation GenerateProductLicenseForSubscription($productSubscriptionID: ID!, $license: ProductLicenseInput!) {
        dotcom {
            generateProductLicenseForSubscription(productSubscriptionID: $productSubscriptionID, license: $license) {
                id
            }
        }
    }
`

const siteAdminProductLicenseFragment = gql`
    fragment ProductLicenseFields on ProductLicense {
        id
        subscription {
            id
            uuid
            name
            account {
                ...ProductLicenseSubscriptionAccount
            }
            activeLicense {
                id
            }
            urlForSiteAdmin
        }
        licenseKey
        siteID
        info {
            ...ProductLicenseInfoFields
        }
        createdAt
        revokedAt
        revokeReason
        version
    }

    fragment ProductLicenseInfoFields on ProductLicenseInfo {
        productNameWithBrand
        tags
        userCount
        expiresAt
        salesforceSubscriptionID
        salesforceOpportunityID
    }

    fragment ProductLicenseSubscriptionAccount on User {
        id
        username
        displayName
    }
`

export const PRODUCT_LICENSES = gql`
    query ProductLicenses($first: Int, $subscriptionUUID: String!) {
        dotcom {
            productSubscription(uuid: $subscriptionUUID) {
                productLicenses(first: $first) {
                    nodes {
                        ...ProductLicenseFields
                    }
                    totalCount
                    pageInfo {
                        hasNextPage
                    }
                }
            }
        }
    }
    ${siteAdminProductLicenseFragment}
`

export const useProductSubscriptionLicensesConnection = (
    subscriptionUUID: string
): UseShowMorePaginationResult<ProductLicensesResult, ProductLicenseFields> =>
    useShowMorePagination<ProductLicensesResult, ProductLicensesVariables, ProductLicenseFields>({
        query: PRODUCT_LICENSES,
        variables: {
            subscriptionUUID,
        },
        getConnection: result => {
            const { dotcom } = dataOrThrowErrors(result)
            return dotcom.productSubscription.productLicenses
        },
        options: {
            fetchPolicy: 'cache-and-network',
        },
    })

const QUERY_PRODUCT_LICENSES = gql`
    query DotComProductLicenses($first: Int, $licenseKeySubstring: String) {
        dotcom {
            productLicenses(first: $first, licenseKeySubstring: $licenseKeySubstring) {
                nodes {
                    ...ProductLicenseFields
                }
                totalCount
                pageInfo {
                    hasNextPage
                }
            }
        }
    }
    ${siteAdminProductLicenseFragment}
`

export const useQueryProductLicensesConnection = (
    licenseKeySubstring: string
): UseShowMorePaginationResult<DotComProductLicensesResult, ProductLicenseFields> =>
    useShowMorePagination<DotComProductLicensesResult, DotComProductLicensesVariables, ProductLicenseFields>({
        query: QUERY_PRODUCT_LICENSES,
        variables: {
            licenseKeySubstring,
        },
        getConnection: result => {
            const { dotcom } = dataOrThrowErrors(result)
            return dotcom.productLicenses
        },
        options: {
            fetchPolicy: 'cache-and-network',
            skip: !licenseKeySubstring,
        },
    })
