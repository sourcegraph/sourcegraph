// This is the entry point for the web app

// Order is important here
// Don't remove the empty lines between these imports

// prettier-ignore-start
import '@sourcegraph/shared/src/polyfills'
// prettier-ignore-end

import '../initBuildInfo'
import '../monitoring/initMonitoring'

import { createRoot } from 'react-dom/client'

import { logger } from '@sourcegraph/common'
import { setLinkComponent } from '@sourcegraph/wildcard'

import { initAppShell } from '../storm/app-shell-init'
import { WebNextAwareLink } from '../sveltekit/WebNextAwareLink'

import { EnterpriseWebApp } from './EnterpriseWebApp'

const appShellPromise = initAppShell()

setLinkComponent(WebNextAwareLink)

// It's important to have a root component in a separate file to create a react-refresh boundary and avoid page reload.
window.addEventListener('DOMContentLoaded', async () => {
    const root = createRoot(document.querySelector('#root')!)

    try {
        const { graphqlClient, temporarySettingsStorage } = await appShellPromise

        root.render(
            <EnterpriseWebApp
                graphqlClient={graphqlClient}
                temporarySettingsStorage={temporarySettingsStorage}
                telemetryRecorder={window.context.telemetryRecorder}
            />
        )
    } catch (error) {
        logger.error('Failed to initialize the app shell', error)
    }
})

if (process.env.NODE_ENV === 'development') {
    new EventSource('/.assets/esbuild').addEventListener('change', () => {
        location.reload()
    })
}
