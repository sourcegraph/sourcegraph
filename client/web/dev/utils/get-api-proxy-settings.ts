import type * as http from 'http'
import * as zlib from 'zlib'

import { type Options, responseInterceptor } from 'http-proxy-middleware'

import { ENVIRONMENT_CONFIG, HTTPS_WEB_SERVER_URL } from './environment-config'
import { STREAMING_ENDPOINTS } from './should-compress-response'

// One of the API routes: "/-/sign-in".
const PROXY_ROUTES = ['/.api', '/search/stream', '/-', '/.auth']

interface GetAPIProxySettingsOptions {
    apiURL: string
    /**
     * If provided, the server will proxy requests to index.html
     * and inject the `window.context` defined there into the local template.
     */
    getLocalIndexHTML?: (jsContextScript?: string) => string
}

interface ProxySettings extends Options {
    proxyRoutes: string[]
}

export function getAPIProxySettings(options: GetAPIProxySettingsOptions): ProxySettings {
    const { apiURL, getLocalIndexHTML } = options

    return {
        // Enable index.html proxy if `getLocalIndexHTML` is provided.
        proxyRoutes: [...PROXY_ROUTES, ...(getLocalIndexHTML ? [''] : [])],
        target: apiURL,
        // Do not SSL certificate.
        secure: false,
        // Change the origin of the host header to the target URL.
        changeOrigin: true,
        // Rewrite domain of `set-cookie` headers for all cookies received.
        cookieDomainRewrite: '',
        // Prevent automatic call of res.end() in `onProxyRes`. It is handled by `responseInterceptor`.
        selfHandleResponse: true,
        // eslint-disable-next-line @typescript-eslint/require-await
        onProxyRes: conditionalResponseInterceptor(STREAMING_ENDPOINTS, async (responseBuffer, proxyRes) => {
            // Propagate cookies to enable authentication on the remote server.
            if (proxyRes.headers['set-cookie']) {
                // Remove `Secure` and `SameSite` from `set-cookie` headers.
                const cookies = proxyRes.headers['set-cookie'].map(cookie =>
                    cookie.replace(/; secure/gi, '').replace(/; samesite=.+/gi, '')
                )

                proxyRes.headers['set-cookie'] = cookies
            }

            // Extract remote `window.context` from the HTML response and inject it into
            // the index.html generated by `getLocalIndexHTML`.
            if (
                getLocalIndexHTML &&
                // router.go is not up to date with client routes and still serves index.html with 404
                (proxyRes.statusCode === 200 || proxyRes.statusCode === 404) &&
                proxyRes.headers['content-type'] &&
                proxyRes.headers['content-type'].includes('text/html')
            ) {
                const remoteIndexHTML = responseBuffer.toString('utf8')

                return getLocalIndexHTML(getRemoteJsContextScript(remoteIndexHTML))
            }

            return responseBuffer
        }),
        onProxyReq: proxyRequest => {
            // Not really clear why, but the `changeOrigin: true` setting does NOT add the correct
            // Origin header to requests sent to k8s.sgdev.org, which e.g. breaks sign in and more. So
            // we add it ourselves.
            proxyRequest.setHeader('Origin', apiURL)
        },
        // TODO: share with `client/web/gulpfile.js`
        // Avoid crashing on "read ECONNRESET".
        onError: () => undefined,
        // Don't log proxy errors, these usually just contain
        // ECONNRESET errors caused by the browser cancelling
        // requests. This should not be needed to actually debug something.
        logLevel: 'silent',
        onProxyReqWs: (_proxyRequest, _request, socket) =>
            socket.on('error', error => console.error('WebSocket proxy error:', error)),
    }
}

const jsContextChanges = `
    // Changes to remote 'window.context' required for local development.
    Object.assign(window.context, {
        // Only username/password auth-provider provider is supported with the standalone server.
        authProviders: window.context.authProviders.filter(provider => provider.isBuiltin),

        // Sync externalURL with the development environment config.
        externalURL: '${HTTPS_WEB_SERVER_URL}',

        // Enable local testing of OpenTelemtry endpoints.
        openTelemetry: {
            endpoint: '${ENVIRONMENT_CONFIG.CLIENT_OTEL_EXPORTER_OTLP_ENDPOINT}',
        },

        // Do not send errors to Sentry from the development environment.
        sentryDSN: null,

        siteGQLID: 'TestGQLSiteID',
        siteID: 'TestSiteID',
        version: 'web-standalone',
    })
`

function getRemoteJsContextScript(remoteIndexHTML: string): string {
    const remoteJsContextStart = remoteIndexHTML.indexOf('window.context = {')
    const remoteJsContextEnd = remoteIndexHTML.indexOf('</script>', remoteJsContextStart)

    return remoteIndexHTML.slice(remoteJsContextStart, remoteJsContextEnd) + jsContextChanges
}

type Interceptor = (
    buffer: Buffer,
    proxyRes: http.IncomingMessage,
    req: http.IncomingMessage,
    res: http.ServerResponse
) => Promise<Buffer | string>

function conditionalResponseInterceptor(
    ignoredRoutes: string[],
    interceptor: Interceptor
): (proxyRes: http.IncomingMessage, req: http.IncomingMessage, res: http.ServerResponse) => Promise<void> {
    const unconditionalResponseInterceptor = responseInterceptor(interceptor)

    return async function proxyResResponseInterceptor(
        proxyRes: http.IncomingMessage,
        req: http.IncomingMessage,
        res: http.ServerResponse
    ): Promise<void> {
        let shouldStream = false
        for (const route of ignoredRoutes) {
            if (req.url?.startsWith(route)) {
                shouldStream = true
            }
        }

        if (shouldStream) {
            return new Promise(resolve => {
                res.setHeader('content-type', 'text/event-stream')
                const _proxyRes = decompress(proxyRes, proxyRes.headers['content-encoding'])

                _proxyRes.on('data', (chunk: any) => res.write(chunk))
                _proxyRes.on('end', () => {
                    res.end()
                    resolve()
                })
                _proxyRes.on('error', () => {
                    res.end()
                    resolve()
                })
            })
        }

        return unconditionalResponseInterceptor(proxyRes, req, res)
    }
}

function decompress<TReq extends http.IncomingMessage = http.IncomingMessage>(
    proxyRes: TReq,
    contentEncoding?: string
): TReq | zlib.Gunzip | zlib.Inflate | zlib.BrotliDecompress {
    let _proxyRes: TReq | zlib.Gunzip | zlib.Inflate | zlib.BrotliDecompress = proxyRes
    let decompress

    switch (contentEncoding) {
        case 'gzip':
            decompress = zlib.createGunzip()
            break
        case 'br':
            decompress = zlib.createBrotliDecompress()
            break
        case 'deflate':
            decompress = zlib.createInflate()
            break
        default:
            break
    }

    if (decompress) {
        _proxyRes.pipe(decompress)
        _proxyRes = decompress
    }

    return _proxyRes
}
