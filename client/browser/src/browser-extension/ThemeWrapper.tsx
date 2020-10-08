import React, { useEffect, useMemo, useState } from 'react'
import { ThemeProps } from '../../../shared/src/theme'

/**
 * Wrapper for the browser extension that listens to changes of the OS theme.
 */
export function ThemeWrapper({
    children: Children,
}: {
    children: (props: ThemeProps) => JSX.Element | null
}): JSX.Element | null {
    const darkThemeMediaList = useMemo(() => window.matchMedia('(prefers-color-scheme: dark)'), [])
    const [isLightTheme, setIsLightTheme] = useState(!darkThemeMediaList.matches)

    useEffect(() => {
        darkThemeMediaList.addListener(event => setIsLightTheme(!event.matches))
    }, [darkThemeMediaList])

    useEffect(() => {
        document.body.classList.toggle('theme-light', isLightTheme)
        document.body.classList.toggle('theme-dark', !isLightTheme)
    }, [isLightTheme])

    return <Children isLightTheme={isLightTheme} />
}
