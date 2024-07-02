import { startCase } from 'lodash'

import { isErrorLike } from '@sourcegraph/common'
import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import { SearchMode } from '@sourcegraph/shared/src/search'
import type { SettingsCascadeOrError, SettingsSubjectCommonFields } from '@sourcegraph/shared/src/settings/settings'

import type { AuthenticatedUser } from '../auth'
import type { LegacyLayoutProps } from '../LegacyLayout'

/** A fallback settings subject that can be constructed synchronously at initialization time. */
export function siteSubjectNoAdmin(): SettingsSubjectCommonFields {
    return {
        id: window.context.siteGQLID,
        viewerCanAdminister: false,
    }
}

export function viewerSubjectFromSettings(
    cascade: SettingsCascadeOrError,
    authenticatedUser?: AuthenticatedUser | null
): LegacyLayoutProps['viewerSubject'] {
    if (authenticatedUser) {
        return authenticatedUser
    }
    if (cascade && !isErrorLike(cascade) && cascade.subjects && cascade.subjects.length > 0) {
        return cascade.subjects[0].subject
    }
    return siteSubjectNoAdmin()
}
/**
 * Returns the user-configured default search mode or undefined if not
 * configured by the user.
 */
export function defaultSearchModeFromSettings(settingsCascade: SettingsCascadeOrError): SearchMode | undefined {
    // If keyword search is enabled, always use the precise search mode. Keyword search is not designed to
    // work with smart search, and we plan to remove smart search.
    const patternType = defaultPatternTypeFromSettings(settingsCascade)
    if (patternType === SearchPatternType.keyword) {
        return SearchMode.Precise
    }

    switch (getFromSettings(settingsCascade, 'search.defaultMode')) {
        case 'precise': {
            return SearchMode.Precise
        }
        case 'smart': {
            return SearchMode.SmartSearch
        }
    }
    return undefined
}

/**
 * Returns the user-configured search pattern type or the default 'keyword' if not configured.
 */
export function defaultPatternTypeFromSettings(settingsCascade: SettingsCascadeOrError): SearchPatternType {
    const defaultPatternType: SearchPatternType | undefined = getFromSettings(
        settingsCascade,
        'search.defaultPatternType'
    )
    return defaultPatternType ?? SearchPatternType.keyword
}

/**
 * Returns the user-configured case sensitivity setting or undefined if not configured.
 */
export function defaultCaseSensitiveFromSettings(settingsCascade: SettingsCascadeOrError): boolean | undefined {
    return getFromSettings(settingsCascade, 'search.defaultCaseSensitive')
}

/**
 * Returns undefined if the settings cannot be loaded or if the setting doesn't
 * exist.
 */
function getFromSettings<T>(settingsCascade: SettingsCascadeOrError, setting: string): T | undefined {
    if (!settingsCascade.final) {
        return undefined
    }
    if (isErrorLike(settingsCascade.final)) {
        return undefined
    }

    const value = settingsCascade.final[setting]
    if (value !== undefined) {
        return value as T
    }

    return undefined
}

export const prettifySystemRole = (role: string): string => startCase(role.replaceAll('_', ' ').toLowerCase())
export const prettifyNamespace = (namespace: string): string => startCase(namespace.replaceAll('_', ' ').toLowerCase())
export const prettifyAction = (action: string): string => startCase(action.replaceAll('_', ' ').toLowerCase())
