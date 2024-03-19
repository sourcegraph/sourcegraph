// These are currently used by the web app and because the web-sveltekit app uses
// code from the web app these need to be defined here as well.
interface PageError {
    statusCode: number
    statusText: string
    error: string
    errorID: string
}

interface Window {
    pageError?: PageError
    buildInfo: BuildInfo
    context: import('@sourcegraph/web/src/jscontext').SourcegraphContext
}
