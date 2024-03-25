import { type Writable, writable, type Updater, derived } from 'svelte/store'

import { browser } from '$app/environment'
import { ThemeSetting, Theme } from '$lib/shared'

import { temporarySetting, type TemporarySettingStore } from './temporarySettings'

const LOCAL_STORAGE_THEME_KEY = 'sourcegraph-theme'

export { ThemeSetting, Theme }

/**
 * The user's theme preference.
 */
export const themeSetting: Writable<ThemeSetting> = (function () {
    let theme: ThemeSetting = ((browser && localStorage.getItem(LOCAL_STORAGE_THEME_KEY)) ||
        ThemeSetting.System) as ThemeSetting
    let themeSettingStore: TemporarySettingStore<'user.themePreference'> | undefined

    const { subscribe } = writable(theme, set => {
        if (!themeSettingStore) {
            themeSettingStore = temporarySetting('user.themePreference')
        }

        return themeSettingStore.subscribe($themeSetting => {
            if (!$themeSetting.loading && $themeSetting.data) {
                switch ($themeSetting.data.toLowerCase()) {
                    // Handle new and old theme settings
                    case 'true':
                    case 'light': {
                        theme = ThemeSetting.Light
                        break
                    }
                    case 'false':
                    case 'dark': {
                        theme = ThemeSetting.Dark
                        break
                    }
                    default: {
                        theme = ThemeSetting.System
                        break
                    }
                }
            }
            localStorage.setItem(LOCAL_STORAGE_THEME_KEY, theme)
            set(theme)
        })
    })

    return {
        subscribe,
        set: (value: ThemeSetting) => {
            themeSettingStore?.setValue(value)
        },
        update: (updater: Updater<ThemeSetting>) => {
            themeSettingStore?.setValue(updater(theme))
        },
    }
})()

/**
 * The current theme. If the theme is set to 'system', the theme will change
 * based on the user's system preference.
 */
export const theme = derived(themeSetting, ($themeSetting, set) => {
    if ($themeSetting === ThemeSetting.System) {
        const matchMedia = window.matchMedia('(prefers-color-scheme: dark)')
        set(matchMedia.matches ? Theme.Dark : Theme.Light)
        const listener = (event: MediaQueryListEventMap['change']): void => {
            set(event.matches ? Theme.Dark : Theme.Light)
        }
        matchMedia.addEventListener('change', listener)
        return () => matchMedia.removeEventListener('change', listener)
    }
    set($themeSetting === ThemeSetting.Dark ? Theme.Dark : Theme.Light)
    return
})

/**
 * Helper store that returns true if the current theme is light. This is
 * a common use case for components that need to know if they should render
 * light or dark content based on the current theme.
 */
export const isLightTheme = derived(theme, $theme => $theme === Theme.Light)
