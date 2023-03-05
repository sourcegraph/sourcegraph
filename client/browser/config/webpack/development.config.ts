import * as path from 'path'

import * as webpack from 'webpack'

import { getCacheConfig } from '@sourcegraph/build-config'

import { config as baseConfig, browserWorkspacePath } from './base.config'
import { generateBundleUID } from './utils'

const { plugins, entry: entries, ...base } = baseConfig

const entriesWithAutoReload = {
    ...entries,
    background: [path.join(__dirname, '../../src/browser-extension/scripts/auto-reloading.ts'), ...entries.background],
}

export const config: webpack.Configuration = {
    ...base,
    entry: process.env.AUTO_RELOAD === 'false' ? entries : entriesWithAutoReload,
    mode: 'development',
    // Use cache only in `development` mode to speed up production build.
    cache: getCacheConfig({ invalidateCacheFiles: [path.resolve(browserWorkspacePath, 'babel.config.js')] }),
    plugins: (plugins || []).concat(
        ...[
            new webpack.DefinePlugin({
                'process.env': {
                    NODE_ENV: JSON.stringify('development'),
                    BUNDLE_UID: JSON.stringify(generateBundleUID()),
                },
            }),
        ]
    ),
}
