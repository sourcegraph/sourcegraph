interface PageError {
    statusCode: number
    statusText: string
    error: string
    errorID: string
}

interface BuildInfo {
    commitSHA?: string
    version?: string
}

interface Window {
    pageError?: PageError
    buildInfo: BuildInfo
    context: import('./jscontext').SourcegraphContext
}

declare module '*.scss' {
    const cssModule: string
    export default cssModule
}
declare module '*.yaml' {
    const yamlModule: string
    export default yamlModule
}
declare module '*.yml' {
    const ymlModule: string
    export default ymlModule
}
