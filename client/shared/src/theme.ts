import { createContext, useCallback, useContext, useSyncExternalStore } from 'react'

import { useTemporarySetting } from './settings/temporary'
import { Theme, ThemeSetting } from './theme-types'

export { Theme, ThemeSetting }

interface ThemeContextData {
    themeSetting: ThemeSetting | null
    onThemeSettingChanging?: (themeSetting: ThemeSetting) => void
}
/**
 * This context is used when we want to override some subtree of UI with
 * specific theme, widely used in storybook testing
 */
export const ThemeContext = createContext<ThemeContextData>({
    themeSetting: null,
    onThemeSettingChanging: undefined,
})

interface useThemeApi {
    /**
     * Current theme value, might be either user setting based or derived
     * on system settings, it depends on {@link themeSetting} value
     */
    theme: Theme

    /**
     * Current theme setting value, it is set by user in their user settings.
     */
    themeSetting: ThemeSetting

    /**
     * Theme setting setter handler to change the current theme setting value.
     */
    setThemeSetting: (setting: ThemeSetting) => void
}

/**
 * Provides API to read and write theme settings, it doesn't contain any
 * side effects (like theme CSS class setters, so it may used on any level
 * of components)
 */
export function useTheme(): useThemeApi {
    const { themeSetting: contextThemeSetting, onThemeSettingChanging } = useContext(ThemeContext)

    const systemTheme = useSyncExternalStore(subscribeToSystemTheme, getSystemThemeSnapshot)
    const [userThemeSetting, setUserThemeSetting] = useUserThemeSetting()

    // Prefer context value over internal value and setter
    const themeSetting = contextThemeSetting ?? userThemeSetting
    const handleSetThemeSetting = useCallback(
        (themeSetting: ThemeSetting) => {
            if (onThemeSettingChanging) {
                onThemeSettingChanging(themeSetting)
            } else {
                setUserThemeSetting(themeSetting)
            }
        },
        [setUserThemeSetting, onThemeSettingChanging]
    )

    return {
        theme: themeSetting === ThemeSetting.System ? systemTheme : (themeSetting as unknown as Theme),
        themeSetting: userThemeSetting,
        setThemeSetting: handleSetThemeSetting,
    }
}

/**
 * A small helper to detect the light there appearance, widely used in
 * brand logo, search box and other components where we can't use CSS variable
 * colors for theming
 */
export function useIsLightTheme(): boolean {
    const { theme } = useTheme()
    return theme === Theme.Light
}

type Unsubscribe = () => void

function subscribeToSystemTheme(callback: () => void): Unsubscribe {
    const matchMedia = window.matchMedia('(prefers-color-scheme: dark)')
    matchMedia.addEventListener('change', callback)

    return () => {
        matchMedia.removeEventListener('change', callback)
    }
}

function getSystemThemeSnapshot(): Theme {
    if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
        return Theme.Dark
    }

    return Theme.Light
}

function useUserThemeSetting(): [ThemeSetting, (setting: ThemeSetting) => void] {
    const [userThemeSetting, setThemeSetting] = useTemporarySetting('user.themePreference', ThemeSetting.System)

    return [readStoredThemePreference(userThemeSetting), setThemeSetting]
}

function readStoredThemePreference(value?: string): ThemeSetting {
    // Handle both old and new preference values
    switch (value?.toLowerCase()) {
        case 'true':
        case 'light': {
            return ThemeSetting.Light
        }
        case 'false':
        case 'dark': {
            return ThemeSetting.Dark
        }
        default: {
            return ThemeSetting.System
        }
    }
}
