import childProcess from 'child_process'
import { mkdtempSync } from 'fs'
import { tmpdir } from 'os'
import path from 'path'

import puppeteer, { type Page } from 'puppeteer'
import rimraf from 'rimraf'

const PORT = 29378
const verbose = process.argv.includes('-v') || process.argv.includes('--verbose')

export interface VSCodeTestDriver {
    page: Page
    dispose: (onDispose?: () => void) => void
}

export async function launchVsCode(vscodeExecutablePath: string): Promise<VSCodeTestDriver> {
    const extensionDevelopmentPath = path.join(__dirname, '..')

    const userDataDirectory = mkdtempSync(path.join(tmpdir(), 'vsce'))
    const extensionsDirectory = mkdtempSync(path.join(tmpdir(), 'vsce'))

    console.log('Starting VS Code', { verbose, vscodeExecutablePath, userDataDirectory, extensionsDirectory })

    const vsCodeProcess = childProcess.spawn(
        vscodeExecutablePath,
        [
            `--extensionDevelopmentPath=${extensionDevelopmentPath}`,
            // Load extension in web worker extension host
            // so we can intercept requests with Polly.js through
            // the Chrome DevTools protocol.
            '--extensionDevelopmentKind=web',
            `--remote-debugging-port=${PORT || 9229}`,
            `--user-data-dir=${userDataDirectory}`,
            `--extensions-dir=${extensionsDirectory}`,
            '--enable-logging',
            '--skip-release-notes',
            // https://github.com/microsoft/vscode-test/issues/120
            '--disable-updates',
            // https://github.com/microsoft/vscode/issues/84238
            '--no-sandbox',
            '--disable-gpu',
        ],
        {
            env: process.env,
            stdio: verbose ? 'inherit' : ['pipe', 'pipe', 'pipe'],
        }
    )

    const dispose: VSCodeTestDriver['dispose'] = onDispose => {
        setTimeout(() => {
            vsCodeProcess.kill()

            // eslint-disable-next-line no-void
            void delay(1000).then(() => {
                rimraf.sync(userDataDirectory)
                rimraf.sync(extensionsDirectory)

                onDispose?.()
            })
        }, 1000)
    }

    for (const event of ['SIGINT', 'SIGUSR1', 'SIGUSR2', 'SIGTERM'] as const) {
        process.on(event, () => dispose?.())
    }
    process.on('exit', () => dispose?.())
    process.on('uncaughtException', () => dispose?.())

    // Initialize Puppeteer

    const browserURL = `http://127.0.0.1:${PORT}`
    await delay(2000)
    console.log(`VSCode started in debug mode on ${browserURL}`)

    const browser = await puppeteer.connect({
        browserURL,
        defaultViewport: null, // used to bypass Chrome viewport issue, doesn't work w/ VS code.
        slowMo: 50,
    })

    await delay(1000)

    // We look for the VSCode frontend process
    const pages = await browser.pages()

    let page: null | Page = null
    for (const _page of pages) {
        const title = await _page.title()
        if (title.includes('Get Started')) {
            page = _page
        }
    }
    if (page === null) {
        throw new Error('Could not find VS Code frontend page')
    }

    return {
        page,
        dispose,
    }
}

function delay(timeout: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, timeout))
}
