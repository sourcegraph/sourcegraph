import { PropsWithChildren, createContext, useContext, useMemo, useState } from 'react'

import { DarkModeOutlined, LightModeOutlined } from '@mui/icons-material'
import { CssBaseline, ThemeProvider as MuiThemeProvider, PaletteMode, Theme, createTheme } from '@mui/material'

type ThemeContextProps = {
    mode: PaletteMode
    toggle: VoidFunction
    theme: Theme
}

export const Context = createContext<ThemeContextProps>({
    mode: 'light',
    toggle: () => {},
    theme: createTheme(),
})

export const ThemeProvider: React.FC<PropsWithChildren> = ({ children }) => {
    const [mode, setMode] = useState<PaletteMode>((localStorage.getItem('theme') as PaletteMode) ?? 'light')

    const theme = useMemo(() => {
        console.log('Theme changed to', mode)
        return createTheme({
            palette: {
                mode: mode,
                secondary: {
                    main: '#f3f4f6',
                    light: '#35aaaa',
                    dark: '#2e3841',
                },
            },
        })
    }, [mode])

    return (
        <Context.Provider
            value={{
                mode,
                theme,
                toggle: () => {
                    console.log('Changing mode')
                    setMode(mode => {
                        const newMode = mode === 'light' ? 'dark' : 'light'
                        localStorage.setItem('theme', newMode)
                        return newMode
                    })
                },
            }}
        >
            <MuiThemeProvider theme={theme}>
                <CssBaseline />
                {children}
            </MuiThemeProvider>
        </Context.Provider>
    )
}

export const Info: React.FC = () => {
    const context = useContext(Context)
    return context.mode === 'dark' ? (
        <DarkModeOutlined onClick={context.toggle} />
    ) : (
        <LightModeOutlined onClick={context.toggle} />
    )
}
