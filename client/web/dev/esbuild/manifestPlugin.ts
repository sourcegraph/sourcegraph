import fs from 'fs'
import path from 'path'

import type * as esbuild from 'esbuild'

/**
 * A function that maps an asset to a manifest entry. If `undefined` is returned, the asset is
 * ignored.
 */
interface ManifestEntryMapper {
    (asset: string, output: esbuild.Metafile['outputs'][string]): string | undefined
}

/**
 * The output of a manifest builder.
 * Each key is a manifest entry. The value is the path to the asset.
 */
type ManifestBuilderOutput<T extends object> = {
    [K in Extract<keyof T, string>]: string
}

/**
 * A class to build a manifest from esbuild build results.
 *
 * `T` describes the structure of the manifest and `mapper` is used to create the manifest
 * entries from the build result.
 */
export class ManifestBuilder<Input extends object> {
    /**
     * @param mapper A map of manifest keys to functions that map an asset to a manifest entry.
     */
    constructor(public readonly mapper: Record<keyof Input, ManifestEntryMapper>) {}

    /**
     * Create a manifest from esbuild build results.
     *
     * @param outdir The output directory. Asset paths will be relative to this directory.
     * @param outputs The esbuild metafile outputs
     * @param manifestBuilder A map of manifest keys to functions that map an asset to a manifest entry.
     */
    public createManifestFromBuildResult(
        outdir: string,
        outputs: esbuild.Metafile['outputs']
    ): ManifestBuilderOutput<Input> {
        const manifest: Partial<ManifestBuilderOutput<Input>> = {}

        for (const key in this.mapper) {
            const builder = this.mapper[key]
            for (const [asset, output] of Object.entries(outputs)) {
                if (!output.entryPoint) {
                    continue
                }
                const result = builder(asset, output)
                if (result) {
                    if (manifest[key]) {
                        throw new Error(`Entry for '${key}' already exists`)
                    }
                    manifest[key] = path.relative(outdir, result)
                }
            }
            if (!manifest[key]) {
                throw new Error(`no output found that matches for '${key}'`)
            }
        }

        // Type casting is OK because we've checked that all keys are present
        return manifest as ManifestBuilderOutput<Input>
    }
}

/**
 * Options for the manifest plugin.
 */
type ManifestPluginOptions<T extends Record<string, string> = Record<string, string>> = {
    /**
     * The filename to use for the manifest, relative to output directory.
     */
    manifestFilename: string

    /**
     * The builder to use to create the manifest.
     */
    builder: ManifestBuilder<T>
}

/**
 * An esbuild plugin to write a web.manifest.json file.
 *
 * `T` describes the structure of the manifest and `options.mapper` is used to create the manifest
 * entries from the build result.
 *
 * `outdir` must be specified in the esbuild build options for this plugin to work.
 */
export function manifestPlugin(options: ManifestPluginOptions): esbuild.Plugin {
    return {
        name: 'manifest',
        setup: build => {
            const origMetafile = build.initialOptions.metafile
            build.initialOptions.metafile = true

            build.onEnd(async result => {
                const outdir = build.initialOptions.outdir
                if (!outdir) {
                    throw new Error('[manifestPlugin] You must specify an `outdir`')
                }
                const outputs = result?.metafile?.outputs
                if (!origMetafile) {
                    // If we were the only consumers of the metafile, then delete it from the result to
                    // avoid unexpected behavior from other downstream consumers relying on the metafile
                    // despite not actually enabling it in the config.
                    delete result.metafile
                }

                if (!outputs) {
                    throw new Error('[manifestPlugin] No outputs found')
                }
                const manifest = options.builder.createManifestFromBuildResult(outdir, outputs)
                const manifestPath = path.join(outdir, options.manifestFilename)
                await fs.promises.writeFile(manifestPath, JSON.stringify(manifest, null, 2))
            })
        },
    }
}
