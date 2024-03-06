import fs from 'fs'
import path from 'path'

import type * as esbuild from 'esbuild'

import { NODE_MODULES_PATH } from '@sourcegraph/build-config'

const fontsDir = 'output/chtml/fonts/woff-v2'
const texScript = 'tex-mml-chtml.js'

/** nodeModule resolves a path in the downloaded mathjax-full node module directory. */
const nodeModule = (file?: string): string => path.resolve(NODE_MODULES_PATH, 'mathjax-full', 'es5', file || '')

/** esbuild plugin to copy Mathjax minified scripts and TeX fonts. */
export const mathjaxPlugin: esbuild.Plugin = {
    name: 'mathjax',
    setup: build => {
        const { outdir } = build.initialOptions
        if (!outdir) {
            throw new Error('[mathjaxPlugin] No outdir found')
        }

        /** dest resolves a path in the mathjax output directory. */
        const dest = (file?: string): string => path.resolve(outdir, 'mathjax', file || '')
        const dir = dest(),
            script = dest(texScript),
            fonts = dest(fontsDir)

        build.onEnd(_ => {
            if (!fs.existsSync(dir)) {
                fs.mkdirSync(dir)
            }

            if (!fs.existsSync(script)) {
                fs.copyFileSync(nodeModule(texScript), script)
            }

            if (!fs.existsSync(fonts)) {
                fs.cpSync(nodeModule(fontsDir), fonts, { recursive: true })
            }
        })
    },
}
