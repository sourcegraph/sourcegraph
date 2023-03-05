import { FC, Suspense, useEffect, useLayoutEffect, useMemo } from 'react'

import { BrowserRouter, Routes, Route } from 'react-router-dom'

import { useTheme, Theme, ThemeSetting } from '@sourcegraph/shared/src/theme'
import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'
import { Alert, LoadingSpinner, setLinkComponent, WildcardTheme, WildcardThemeContext } from '@sourcegraph/wildcard'

import '../../SourcegraphWebApp.scss'

import { useGlobalContributions } from '../../contributions'
import { createPlatformContext } from '../../platform/context'

import { OpenNewTabAnchorLink } from './OpenNewTabAnchorLink'

import styles from './EmbeddedWebApp.module.scss'

// Since we intend to embed the EmbeddedWebApp component within an iframe,
// we want to open all links in a new tab instead of the current iframe window.
// Otherwise, we would get an error that we tried to access a non-embed route from within the iframe.
setLinkComponent(OpenNewTabAnchorLink)

const WILDCARD_THEME: WildcardTheme = {
    isBranded: true,
}

const EmbeddedNotebookPage = lazyComponent(
    () => import('../../notebooks/notebookPage/EmbeddedNotebookPage'),
    'EmbeddedNotebookPage'
)

const EMPTY_SETTINGS_CASCADE = { final: {}, subjects: [] }

export const EmbeddedWebApp: FC = () => {
    const { theme, setThemeSetting } = useTheme()

    useLayoutEffect(() => {
        const isLightTheme = theme === Theme.Light

        document.documentElement.classList.add('theme')
        document.documentElement.classList.toggle('theme-light', isLightTheme)
        document.documentElement.classList.toggle('theme-dark', !isLightTheme)
    }, [theme])

    useEffect(() => {
        const query = new URLSearchParams(window.location.search)
        const theme = query.get('theme')
        setThemeSetting(
            theme === 'dark' ? ThemeSetting.Dark : theme === 'light' ? ThemeSetting.Light : ThemeSetting.System
        )
    }, [setThemeSetting])

    const platformContext = useMemo(() => createPlatformContext(), [])

    useGlobalContributions()

    // 🚨 SECURITY: The `EmbeddedWebApp` is intended to be embedded into 3rd party sites where we do not have total control.
    // That is why it is essential to be mindful when adding new routes that may be vulnerable to clickjacking or similar exploits.
    // It is crucial not to embed any components that an attacker could hijack and use to leak personal information (e.g., the sign-in page).
    // The embedded components should be limited to displaying read-only, publicly accessible content.
    //
    // IMPORTANT: Please consult with the security team if you are unsure whether your changes could introduce security exploits.
    return (
        <BrowserRouter>
            <WildcardThemeContext.Provider value={WILDCARD_THEME}>
                <div className={styles.body}>
                    <Suspense
                        fallback={
                            <div className="d-flex justify-content-center p-3">
                                <LoadingSpinner />
                            </div>
                        }
                    >
                        <Routes>
                            <Route
                                path="/embed/notebooks/:notebookId"
                                element={
                                    <EmbeddedNotebookPage
                                        searchContextsEnabled={true}
                                        isSourcegraphDotCom={window.context.sourcegraphDotComMode}
                                        authenticatedUser={null}
                                        settingsCascade={EMPTY_SETTINGS_CASCADE}
                                        platformContext={platformContext}
                                    />
                                }
                            />
                            Ï
                            <Route
                                path="*"
                                element={
                                    <Alert variant="danger">
                                        Invalid embedding route, please check the embedding URL.
                                    </Alert>
                                }
                            />
                        </Routes>
                    </Suspense>
                </div>
            </WildcardThemeContext.Provider>
        </BrowserRouter>
    )
}
