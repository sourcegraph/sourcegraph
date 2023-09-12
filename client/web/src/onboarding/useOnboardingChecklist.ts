import type { MutationTuple, ApolloQueryResult } from '@apollo/client'
import { parse } from 'jsonc-parser'
import type {
    SiteConfigResult,
    SiteConfigVariables,
    UpdateSiteConfigurationResult,
    UpdateSiteConfigurationVariables,
} from 'src/graphql-operations'

import type { ErrorLike } from '@sourcegraph/common'
import { useQuery, useMutation } from '@sourcegraph/http-client'

import { SITE_CONFIG_QUERY, LICENSE_KEY_MUTATION } from './queries'

interface OnboardingChecklistResult {
    licenseKey: LicenseInfo
    id: number
    config: string
    checklistItem: OnboardingChecklistItem
}

export interface LicenseInfo {
    key: string
    tags: string[]
    userCount: number
    expiresAt: string
}

export interface OnboardingChecklistItem {
    licenseKey: boolean
    externalURL: boolean
    emailSmtp: boolean
    authProviders: boolean
    externalServices: boolean
    usersPermissions: boolean
}

interface UseOnboardingChecklistResult {
    loading: boolean
    error?: ErrorLike
    data?: OnboardingChecklistResult
    refetch: () => Promise<ApolloQueryResult<SiteConfigResult>>
}

export const useOnboardingChecklistQuery = (): UseOnboardingChecklistResult => {
    const { loading, error, data, refetch } = useQuery<SiteConfigResult, SiteConfigVariables>(SITE_CONFIG_QUERY, {
        fetchPolicy: 'no-cache',
    })

    return {
        loading,
        error,
        ...(data && { data: getChecklistItems(data) }),
        refetch,
    }
}

interface EffectiveContent {
    licenseKey: string
    externalURL: string
    'email.smtp': {
        host: string
    }
    'auth.providers': string[]
}

function getChecklistItems(data: SiteConfigResult): OnboardingChecklistResult {
    const config = parse(data.site.configuration.effectiveContents) as EffectiveContent
    const licenseInfo = data.site.configuration.licenseInfo as LicenseInfo

    return {
        id: data.site.configuration.id,
        licenseKey: {
            key: config.licenseKey,
            tags: licenseInfo?.tags ?? [],
            userCount: licenseInfo?.userCount ?? 10,
            expiresAt: licenseInfo?.expiresAt ?? '',
        },
        config: data.site.configuration.effectiveContents,
        checklistItem: {
            licenseKey: config.licenseKey !== '',
            externalURL: config.externalURL !== '',
            emailSmtp: config['email.smtp']?.host !== '' || false,
            authProviders: config['auth.providers'].length > 1,
            externalServices: data.externalServices?.nodes?.length > 0 || false,
            usersPermissions:
                data.externalServices?.nodes?.every(({ unrestrictedAccess }) => !unrestrictedAccess) ?? false,
        },
    }
}

export const useUpdateLicenseKey = (): MutationTuple<UpdateSiteConfigurationResult, UpdateSiteConfigurationVariables> =>
    useMutation(LICENSE_KEY_MUTATION)
