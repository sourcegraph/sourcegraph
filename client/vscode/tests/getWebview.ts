import type puppeteer from 'puppeteer'

export interface VSCodeWebviewFrames {
    sidebarFrame: puppeteer.Frame
    searchPanelFrame: puppeteer.Frame
}

/**
 * The VSCode extension currently opens two web views:
 * one is the sidebar content and the other is the search page.
 *
 * @param page The VS Code frontend page.
 * @returns Search panel and sidebar webview frames.
 */
export async function getVSCodeWebviewFrames(page: puppeteer.Page): Promise<VSCodeWebviewFrames> {
    let sidebarFrame: puppeteer.Frame | undefined
    let searchPanelFrame: puppeteer.Frame | undefined

    // Find the Sourcegraph icon in the activity bar
    // to open the Sourcegraph sidebar and active the extension,
    // which should open a search panel in the process.
    await page.waitForSelector('[aria-label="Sourcegraph"]')
    await page.click('[aria-label="Sourcegraph"]')

    // In the release of VS Code at the time this test harness was written, there was no
    // stable unique selector for the search panel webview's outermost iframe ancestor.
    // Reverse to look for panel webview first.
    const outerFrameHandles = (await page?.$$('div[id^="webview"] iframe')).reverse()

    for (const outerFrameHandle of outerFrameHandles) {
        if (sidebarFrame && searchPanelFrame) {
            break
        }

        const webview = await findSearchWebview(outerFrameHandle)
        if (webview?.type === 'panel') {
            searchPanelFrame = webview.frame
        } else if (webview?.type === 'sidebar') {
            sidebarFrame = webview.frame
        }
    }

    if (!sidebarFrame || !searchPanelFrame) {
        const missingWebviewNames: string[] = []
        if (!sidebarFrame) {
            missingWebviewNames.push('sidebar webview')
        }
        if (!searchPanelFrame) {
            missingWebviewNames.push('panel webview')
        }
        throw new Error(`Could not find ${missingWebviewNames.join(',')}`)
    }

    return {
        sidebarFrame,
        searchPanelFrame,
    }
}

async function findSearchWebview(
    outerFrameHandle: puppeteer.ElementHandle<Element>
): Promise<{ frame: puppeteer.Frame; type: 'panel' | 'sidebar' } | undefined> {
    try {
        const outerFrame = await outerFrameHandle.contentFrame()
        if (!outerFrame) {
            return
        }

        // The search web views have another iframe inside it. ¯\_(ツ)_/¯
        const frameHandle = await outerFrame.waitForSelector('iframe')
        if (frameHandle === null) {
            return
        }

        const frame = await frameHandle.contentFrame()
        if (frame === null) {
            return
        }

        const body = await frame.waitForSelector('body')
        if (body) {
            const className = await (await body.getProperty('className')).jsonValue()
            if (typeof className === 'string') {
                if (className.includes('search-sidebar')) {
                    return { frame, type: 'sidebar' }
                }
                if (className.includes('search-panel')) {
                    return { frame, type: 'panel' }
                }
            }
        }
    } catch {
        // Likely a selector timeout. Noop, we will throw if we never find the webview.
    }
    return
}
