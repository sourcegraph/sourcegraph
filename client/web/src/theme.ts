import { useMemo } from 'react'

import { observeSystemIsLightTheme, ThemeProps } from '@sourcegraph/shared/src/theme'
import { useObservable } from '@sourcegraph/shared/src/util/useObservable'

import { useGlobalStore } from './stores/global'

/**
 * The user preference for the theme.
 * These values are stored in local storage.
 */
export enum ThemePreference {
    Light = 'light',
    Dark = 'dark',
    System = 'system',
}

/**
 * Props that can be extended by any component's Props which needs to manipulate the theme preferences.
 */
export interface ThemePreferenceProps {
    themePreference: ThemePreference
    onThemePreferenceChange: (theme: ThemePreference) => void
}

export interface ThemeState {
    /**
     * Parsed from local storage theme preference value.
     */
    themePreference: ThemePreference

    /**
     * Calculated theme preference. It value takes system preference
     * value into account if parsed value is equal to 'system'
     */
    enhancedThemePreference: ThemePreference.Light | ThemePreference.Dark

    setThemePreference: (theme: ThemePreference) => void
}

export const useThemeState = (): ThemeState => {
    // React to system-wide theme change.
    const { observable: systemIsLightThemeObservable, initialValue: systemIsLightThemeInitialValue } = useMemo(
        () => observeSystemIsLightTheme(window),
        []
    )
    const systemIsLightTheme = useObservable(systemIsLightThemeObservable) ?? systemIsLightThemeInitialValue

    const [themePreference, setThemePreference] = useGlobalStore(state => [state.theme, state.setTheme])
    const enhancedThemePreference =
        themePreference === ThemePreference.System
            ? systemIsLightTheme
                ? ThemePreference.Light
                : ThemePreference.Dark
            : themePreference

    return {
        themePreference,
        enhancedThemePreference,
        setThemePreference,
    }
}

/**
 * A React hook for getting and setting the theme.
 */
export const useTheme = (): ThemeProps & ThemePreferenceProps => {
    const { themePreference, enhancedThemePreference, setThemePreference } = useThemeState()
    const isLightTheme = enhancedThemePreference === ThemePreference.Light

    useMemo(() => {
        document.documentElement.classList.toggle('theme-light', isLightTheme)
        document.documentElement.classList.toggle('theme-dark', !isLightTheme)
    }, [isLightTheme])

    return {
        isLightTheme,
        themePreference,
        onThemePreferenceChange: setThemePreference,
    }
}
