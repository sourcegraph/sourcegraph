import { firstValueFrom } from 'rxjs'

import { logger } from '@sourcegraph/common'

import type { TemporarySettings, TemporarySettingsSchema } from './TemporarySettings'
import type { TemporarySettingsStorage } from './TemporarySettingsStorage'

interface Migration {
    localStorageKey: string
    temporarySettingsKey: keyof TemporarySettings
    type: 'boolean' | 'number' | 'string' | 'json'
    transform?: (value: any) => any
    preserve?: boolean
}

const migrations: Migration[] = [
    {
        localStorageKey: 'days-active-count',
        temporarySettingsKey: 'user.daysActiveCount',
        type: 'number',
    },
    {
        localStorageKey: 'has-dismissed-survey-toast',
        temporarySettingsKey: 'npsSurvey.hasTemporarilyDismissed',
        type: 'boolean',
    },
    {
        localStorageKey: 'has-permanently-dismissed-survey-toast',
        temporarySettingsKey: 'npsSurvey.hasPermanentlyDismissed',
        type: 'boolean',
    },
    {
        localStorageKey: 'finished-welcome-flow',
        temporarySettingsKey: 'signup.finishedWelcomeFlow',
        type: 'boolean',
    },
    {
        localStorageKey: 'quick-start-tour',
        temporarySettingsKey: 'onboarding.quickStartTour',
        type: 'json',
        transform: (value: { state: { tours: TemporarySettingsSchema['onboarding.quickStartTour'] } }) =>
            value.state.tours,
        preserve: true,
    },
    {
        localStorageKey: 'diff-mode-visualizer',
        temporarySettingsKey: 'repo.commitPage.diffMode',
        type: 'string',
    },
]

const parse = (type: Migration['type'], localStorageValue: string | null): boolean | number | any => {
    if (localStorageValue === null) {
        return
    }

    if (type === 'boolean') {
        return localStorageValue === 'true'
    }

    if (type === 'number') {
        return parseInt(localStorageValue, 10)
    }

    if (type === 'json') {
        return JSON.parse(localStorageValue)
    }

    if (type === 'string') {
        return localStorageValue
    }

    return
}

export async function migrateLocalStorageToTemporarySettings(storage: TemporarySettingsStorage): Promise<void> {
    for (const migration of migrations) {
        // Use the first value of the setting to check if it exists.
        // Only migrate if the setting is not already set.
        const temporarySetting = await firstValueFrom(storage.get(migration.temporarySettingsKey), {
            defaultValue: undefined,
        })
        if (temporarySetting === undefined) {
            try {
                const value = parse(migration.type, localStorage.getItem(migration.localStorageKey))
                if (!value) {
                    continue
                }

                storage.set(migration.temporarySettingsKey, migration.transform?.(value) ?? value)
                if (!migration.preserve) {
                    localStorage.removeItem(migration.localStorageKey)
                }
            } catch (error) {
                logger.error(
                    `Failed to migrate temporary settings "${migration.temporarySettingsKey}" from localStorage using key "${migration.localStorageKey}"`,
                    error
                )
            }
        }
    }
}
