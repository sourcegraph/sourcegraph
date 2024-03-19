import type { MutationTuple } from '@apollo/client'
import { parse } from 'jsonc-parser'

import { useQuery, useMutation } from '@sourcegraph/http-client'

import type {
    SiteConfigResult,
    SiteConfigVariables,
    UpdateSiteConfigurationResult,
    UpdateSiteConfigurationVariables,
} from '../graphql-operations'

import { SITE_CONFIG_QUERY, LICENSE_KEY_MUTATION } from './queries'
import type { UseOnboardingChecklistResult, OnboardingChecklistResult, EffectiveContent, LicenseInfo } from './types'

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

function getChecklistItems(data: SiteConfigResult): OnboardingChecklistResult {
    const config = parse(data.site.configuration.effectiveContents) as EffectiveContent
    const { licenseKey: key, externalURL } = config
    const { tags = [], userCount = 10, expiresAt = '' } = (data.site.configuration.licenseInfo as LicenseInfo) || {}

    return {
        id: data.site.configuration.id,
        licenseKey: {
            key,
            tags,
            userCount,
            expiresAt,
        },
        config: data.site.configuration.effectiveContents,
        checklistItem: {
            licenseKey: key !== '',
            externalURL: externalURL !== '',
            emailSmtp: config['email.smtp']?.host !== '' || false,
            authProviders: config['auth.providers'].length > 1,
            externalServices: data.externalServices?.nodes?.length > 0 || false,
        },
    }
}

export const useUpdateLicenseKeyMutation = (): MutationTuple<
    UpdateSiteConfigurationResult,
    UpdateSiteConfigurationVariables
> => useMutation(LICENSE_KEY_MUTATION)
