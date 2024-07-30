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
