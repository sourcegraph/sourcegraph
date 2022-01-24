// We want to polyfill first.
import '../../shared/polyfills'

import React from 'react'
import { render } from 'react-dom'

import { WildcardThemeProvider } from '../../shared/components/WildcardThemeProvider'
import { AfterInstallPageContent } from '../after-install-page/AfterInstallPageContent'
import { ThemeWrapper } from '../ThemeWrapper'

const AfterInstallPage: React.FunctionComponent = () => (
    <ThemeWrapper>
        {({ isLightTheme }) => (
            <WildcardThemeProvider>
                <AfterInstallPageContent isLightTheme={isLightTheme} />
            </WildcardThemeProvider>
        )}
    </ThemeWrapper>
)

document.addEventListener('DOMContentLoaded', () => render(<AfterInstallPage />, document.querySelector('#root')))
