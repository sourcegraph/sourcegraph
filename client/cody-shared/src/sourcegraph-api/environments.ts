export const DOTCOM_URL = new URL('https://sourcegraph.com')
export const INTERNAL_S2_URL = new URL('https://sourcegraph.sourcegraph.com/')
export const LOCAL_APP_URL = new URL('http://localhost:3080')

export function isLocalApp(url: string): boolean {
    try {
        return new URL(url).origin === LOCAL_APP_URL.origin
    } catch {
        return false
    }
}

export function isDotCom(url: string): boolean {
    try {
        return new URL(url).origin === DOTCOM_URL.origin
    } catch {
        return false
    }
}

export function isInternalUser(url: string): boolean {
    try {
        return new URL(url).origin === INTERNAL_S2_URL.origin
    } catch {
        return false
    }
}
