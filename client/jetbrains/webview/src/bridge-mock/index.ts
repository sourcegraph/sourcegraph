import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'

import type { Request } from '../search/js-to-java-bridge'
import type { Search } from '../search/types'

let savedSearch: Search = {
    query: 'r:github.com/sourcegraph/sourcegraph jetbrains',
    caseSensitive: false,
    patternType: SearchPatternType.literal,
    selectedSearchContextSpec: 'global',
}

const instanceURL = 'https://sourcegraph.com'

const codeDetailsNode = document.querySelector('#code-details') as HTMLPreElement
const iframeNode = document.querySelector('#webview') as HTMLIFrameElement

function callJava(request: Request): Promise<object> {
    return new Promise((resolve, reject) => {
        const requestAsString = JSON.stringify(request)
        const onSuccessCallback = (responseAsString: string): void => {
            resolve(JSON.parse(responseAsString))
        }
        const onFailureCallback = (errorCode: number, errorMessage: string): void => {
            reject(new Error(`${errorCode} - ${errorMessage}`))
        }
        console.log(`Got this request: ${requestAsString}`)
        handleRequest(request, onSuccessCallback, onFailureCallback)
    })
}

function handleRequest(
    request: Request,
    onSuccessCallback: (responseAsString: string) => void,
    onFailureCallback: (errorCode: number, errorMessage: string) => void
): void {
    const action = request.action
    switch (action) {
        case 'getConfig': {
            onSuccessCallback(
                JSON.stringify({
                    instanceURL,
                    isGlobbingEnabled: true,
                    accessToken: null,
                })
            )
            break
        }

        case 'getTheme': {
            onSuccessCallback(
                JSON.stringify({
                    isDarkTheme: true,
                    backgroundColor: 'blue',
                    buttonArc: '2px',
                    buttonColor: 'red',
                    color: 'green',
                    font: 'Times New Roman',
                    fontSize: '12px',
                    labelBackground: 'gray',
                })
            )
            break
        }

        case 'preview': {
            const { content, absoluteOffsetAndLengths } = request.arguments

            const start = absoluteOffsetAndLengths.length > 0 ? absoluteOffsetAndLengths[0][0] : 0
            const length = absoluteOffsetAndLengths.length > 0 ? absoluteOffsetAndLengths[0][1] : 0

            let htmlContent: string
            if (content === null) {
                htmlContent = '(No preview available)'
            } else {
                const decodedContent = atob(content)
                htmlContent = escapeHTML(decodedContent.slice(0, start))
                htmlContent += `<span id="code-details-highlight">${escapeHTML(
                    decodedContent.slice(start, start + length)
                )}</span>`
                htmlContent += escapeHTML(decodedContent.slice(start + length))
            }

            codeDetailsNode.innerHTML = htmlContent

            document.querySelector('#code-details-highlight')?.scrollIntoView({ block: 'center', inline: 'center' })

            onSuccessCallback('null')
            break
        }

        case 'clearPreview': {
            codeDetailsNode.textContent = ''
            onSuccessCallback('null')
            break
        }

        case 'open': {
            const { path } = request.arguments
            alert(`Opening ${path}`)
            onSuccessCallback('null')
            break
        }

        case 'saveLastSearch': {
            savedSearch = request.arguments
            onSuccessCallback('null')
            break
        }

        case 'loadLastSearch': {
            onSuccessCallback(JSON.stringify(savedSearch))
            break
        }

        case 'indicateFinishedLoading': {
            onSuccessCallback('null')
            break
        }

        case 'openSourcegraphUrl': {
            const { relativeUrl } = request.arguments
            window.open(instanceURL + relativeUrl, '_blank')
            onSuccessCallback('null')
            break
        }

        default: {
            // noinspection UnnecessaryLocalVariableJS
            const exhaustiveCheck: never = action
            onFailureCallback(2, `Unknown action: ${exhaustiveCheck as string}`)
        }
    }
}

/* Initialize app for standalone server */
iframeNode.addEventListener('load', () => {
    const iframeWindow = iframeNode.contentWindow
    if (iframeWindow !== null) {
        iframeWindow.callJava = callJava
        iframeWindow.initializeSourcegraph()
    }
})

function escapeHTML(unsafe: string): string {
    return unsafe.replace(
        // eslint-disable-next-line no-control-regex
        /[\u0000-\u002F\u003A-\u0040\u005B-\u0060\u007B-\u00FF]/g,
        // eslint-disable-next-line @typescript-eslint/restrict-plus-operands
        char => '&#' + ('000' + char.charCodeAt(0)).slice(-4) + ';'
    )
}
