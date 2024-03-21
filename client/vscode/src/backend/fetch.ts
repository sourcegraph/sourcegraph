import type net from 'net'

import type { ClientRequest, RequestOptions } from 'agent-base'
import HttpProxyAgent from 'http-proxy-agent'
import HttpsProxyAgent from 'https-proxy-agent'
import fetch, { Headers } from 'node-fetch'
import vscode from 'vscode'

export { fetch, Headers }
export type { BodyInit, Response, HeadersInit } from 'node-fetch'

interface HttpsProxyAgentInterface {
    callback(req: ClientRequest, opts: RequestOptions): Promise<net.Socket>
}

type HttpsProxyAgentConstructor = new (
    options: string | { [key: string]: number | boolean | string | null }
) => HttpsProxyAgentInterface

export function getProxyAgent(): ((url: URL | string) => HttpsProxyAgentInterface | undefined) | undefined {
    const proxyProtocol = vscode.workspace.getConfiguration('sourcegraph').get<string>('proxyProtocol')
    const proxyHost = vscode.workspace.getConfiguration('sourcegraph').get<string>('proxyHost')
    const proxyPort = vscode.workspace.getConfiguration('sourcegraph').get<number>('proxyPort')
    const proxyPath = vscode.workspace.getConfiguration('sourcegraph').get<string>('proxyPath')

    // Quit if we're in the browser—we don't need proxying there.
    if (HttpsProxyAgent === null) {
        return undefined
    }

    if (proxyHost && !proxyPort) {
        console.error('proxyHost is set but proxyPort is not. These two settings must be set together.')
        return undefined
    }

    if (proxyPort && !proxyHost) {
        console.error('proxyPort is set but proxyHost is not. These two settings must be set together.')
        return undefined
    }

    if (proxyHost || proxyPort || proxyPath) {
        return (url: URL | string) => {
            const protocol = getProtocol(url)
            if (protocol === undefined) {
                return undefined
            }
            const ProxyAgent = (protocol === 'http'
                ? HttpProxyAgent
                : HttpsProxyAgent) as unknown as HttpsProxyAgentConstructor
            return new ProxyAgent({
                protocol: proxyProtocol === 'http' || proxyProtocol === 'https' ? proxyProtocol : 'https',
                ...(proxyHost ? { host: proxyHost } : null),
                ...(proxyPort ? { port: proxyPort } : null),
                ...(proxyPath ? { path: proxyPath } : null),
            })
        }
    }

    return undefined
}

function getProtocol(url: URL | string): string | undefined {
    if (typeof url === 'string') {
        return url.startsWith('http:') ? 'http' : 'https'
    }

    if (url instanceof URL) {
        return url.protocol === 'http:' ? 'http' : 'https'
    }

    return undefined
}
