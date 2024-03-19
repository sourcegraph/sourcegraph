import path from 'path'

import { context as esbuildContext } from 'esbuild'
import express from 'express'
import { createProxyMiddleware } from 'http-proxy-middleware'
import signale from 'signale'

import { STATIC_ASSETS_PATH } from '@sourcegraph/build-config'

import {
    DEV_SERVER_LISTEN_ADDR,
    DEV_SERVER_PROXY_TARGET_ADDR,
    ENVIRONMENT_CONFIG,
    HTTPS_WEB_SERVER_URL,
    printSuccessBanner,
} from '../utils'

import { esbuildBuildOptions } from './config'
import { assetPathPrefix } from './webmanifest'

export const esbuildDevelopmentServer = async (
    listenAddress: { host: string; port: number },
    configureProxy: (app: express.Application) => void
): Promise<void> => {
    const start = performance.now()

    const ctx = await esbuildContext(esbuildBuildOptions(ENVIRONMENT_CONFIG))

    await ctx.watch()

    // Start esbuild's server on a random local port.
    const { host: esbuildHost, port: esbuildPort } = await ctx.serve({
        host: 'localhost',
        servedir: STATIC_ASSETS_PATH,
    })

    // Start a proxy at :3080. Asset requests (underneath /.assets/) go to esbuild; all other
    // requests go to the upstream.
    const proxyApp = express()
    proxyApp.use(
        assetPathPrefix,
        createProxyMiddleware({
            target: { protocol: 'http:', host: esbuildHost, port: esbuildPort },
            pathRewrite: { [`^${assetPathPrefix}`]: '' },
            onProxyRes: (_proxyResponse, request, response) => {
                // Cache chunks because their filename includes a hash of the content.
                const isCacheableChunk = path.basename(request.url).startsWith('chunk-')
                if (isCacheableChunk) {
                    response.setHeader('Cache-Control', 'max-age=3600')
                }
            },
            logLevel: 'error',
        })
    )
    configureProxy(proxyApp)

    const proxyServer = proxyApp.listen(listenAddress)
    // eslint-disable-next-line @typescript-eslint/return-await
    return await new Promise<void>((_resolve, reject) => {
        proxyServer.once('listening', async () => {
            await pingUntilReady({ url: HTTPS_WEB_SERVER_URL })
            signale.success(`esbuild server is ready after ${Math.round(performance.now() - start)}ms`)
            printSuccessBanner(['✱ Sourcegraph is really ready now!', `Click here: ${HTTPS_WEB_SERVER_URL}/search`])
        })
        proxyServer.once('error', error => reject(error))
    })
}

interface PingOptions {
    url: string
    backoffMillis?: number
    factor?: number
    maxBackoffMillis?: number
}

function pingUntilReady({
    url,
    backoffMillis = 500,
    factor = 1.5,
    maxBackoffMillis = 10000,
}: PingOptions): Promise<void> {
    const ping = (backoff: number) => (resolve: () => void) => {
        fetch(url)
            .then(response => {
                if (response.ok) {
                    resolve()
                } else {
                    throw new Error(`${url} produced ${response.status} ${response.statusText}`)
                }
            })
            .catch(error => {
                if (backoff === maxBackoffMillis) {
                    // Only log errors if we have reached the maximum backoff
                    // to avoid spamming the console with transient errors.
                    console.log(error)
                }
            })
            .then(sleep(backoff))
            .then(() => ping(Math.min(backoff * factor, maxBackoffMillis))(resolve))
    }

    return new Promise(ping(backoffMillis))
}

function sleep(ms: number): () => Promise<void> {
    return () => new Promise(resolve => setTimeout(resolve, ms))
}

if (require.main === module) {
    async function main(args: string[]): Promise<void> {
        if (args.length !== 0) {
            throw new Error('Usage: (no options)')
        }
        await esbuildDevelopmentServer(DEV_SERVER_LISTEN_ADDR, app => {
            app.use(
                '/',
                createProxyMiddleware({
                    target: {
                        protocol: 'http:',
                        host: DEV_SERVER_PROXY_TARGET_ADDR.host,
                        port: DEV_SERVER_PROXY_TARGET_ADDR.port,
                    },
                    logLevel: 'error',
                })
            )
        })
    }
    // eslint-disable-next-line unicorn/prefer-top-level-await
    main(process.argv.slice(2)).catch(error => {
        console.error(error)
        process.exit(1)
    })
}
