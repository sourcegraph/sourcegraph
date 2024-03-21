// This fake reuses the built-in "fetch" in browsers.

// eslint-disable-next-line import/no-default-export
export default fetch
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
export const Headers = globalThis.Headers
