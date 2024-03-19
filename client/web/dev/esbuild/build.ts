import { writeFileSync } from 'fs'

import * as esbuild from 'esbuild'

import { ENVIRONMENT_CONFIG } from '../utils'

import { esbuildBuildOptions } from './config'

export async function build(): Promise<void> {
    const buildOptions = esbuildBuildOptions(ENVIRONMENT_CONFIG)

    if (!buildOptions.outdir) {
        throw new Error('no outdir')
    }

    const metafile = process.env.ESBUILD_METAFILE
    const options: esbuild.BuildOptions = {
        ...buildOptions,
        metafile: Boolean(metafile),
    }
    const result = await esbuild.build(options)
    if (metafile) {
        writeFileSync(metafile, JSON.stringify(result.metafile), 'utf-8')
    }
    if (process.env.WATCH) {
        const ctx = await esbuild.context(options)
        await ctx.watch()
        await new Promise(() => {}) // wait forever
    }
}

if (require.main === module) {
    async function main(args: string[]): Promise<void> {
        if (args.length !== 0) {
            throw new Error('Usage: (no options)')
        }
        await build()
    }
    // eslint-disable-next-line unicorn/prefer-top-level-await
    main(process.argv.slice(2)).catch(error => {
        console.error(error)
        process.exit(1)
    })
}
