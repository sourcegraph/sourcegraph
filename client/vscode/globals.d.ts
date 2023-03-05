declare module '*.scss' {
    const cssModule: string
    export default cssModule
}
declare module '*.css' {
    const cssModule: string
    export default cssModule
}

/**
 * Set by shared/dev/jest-environment.js
 */
declare var jsdom: import('jsdom').JSDOM
