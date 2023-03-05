import path from 'path'

import CssMinimizerWebpackPlugin from 'css-minimizer-webpack-plugin'
import MiniCssExtractPlugin from 'mini-css-extract-plugin'
import webpack, { optimize } from 'webpack'

import {
    ROOT_PATH,
    getBabelLoader,
    getCSSLoaders,
    getProvidePlugin,
    getTerserPlugin,
    getCSSModulesLoader,
    getBasicCSSLoader,
} from '@sourcegraph/build-config'
import { subtypeOf } from '@sourcegraph/common'

export const browserWorkspacePath = path.resolve(ROOT_PATH, 'client/browser')
const browserSourcePath = path.resolve(browserWorkspacePath, 'src')
const contentEntry = path.resolve(browserSourcePath, 'config/content.entry.js')
const backgroundEntry = path.resolve(browserSourcePath, 'config/background.entry.js')
const optionsEntry = path.resolve(browserSourcePath, 'config/options.entry.js')
const pageEntry = path.resolve(browserSourcePath, 'config/page.entry.js')
const extensionEntry = path.resolve(browserSourcePath, 'config/extension.entry.js')

export const config = subtypeOf<webpack.Configuration>()({
    target: 'browserslist',
    entry: {
        // Browser extension
        background: [
            extensionEntry,
            backgroundEntry,
            path.resolve(browserSourcePath, 'browser-extension/scripts/backgroundPage.main.ts'),
        ],
        inject: [
            extensionEntry,
            contentEntry,
            path.resolve(browserSourcePath, 'browser-extension/scripts/contentPage.main.ts'),
        ],
        options: [
            extensionEntry,
            optionsEntry,
            path.resolve(browserSourcePath, 'browser-extension/scripts/optionsPage.main.tsx'),
        ],
        'after-install': path.resolve(browserSourcePath, 'browser-extension/scripts/afterInstallPage.main.tsx'),

        // Common native integration entry point (Gitlab, Bitbucket)
        integration: [pageEntry, path.resolve(browserSourcePath, 'native-integration/integration.main.ts')],
        // Phabricator-only native integration entry point
        phabricator: [pageEntry, path.resolve(browserSourcePath, 'native-integration/phabricator/integration.main.ts')],

        // Styles
        style: path.join(browserSourcePath, 'app.scss'),
        'branded-style': path.join(browserSourcePath, 'branded.scss'),
    },
    output: {
        path: path.join(browserWorkspacePath, 'build/dist/js'),
        filename: '[name].bundle.js',
        chunkFilename: '[id].chunk.js',
    },
    devtool: 'inline-cheap-module-source-map',
    optimization: {
        minimizer: [getTerserPlugin() as webpack.WebpackPluginInstance, new CssMinimizerWebpackPlugin()],
    },

    plugins: [
        new MiniCssExtractPlugin({ filename: '../css/[name].bundle.css' }),
        // Code splitting doesn't make sense/work in the browser extension, but we still want to use dynamic import()
        new optimize.LimitChunkCountPlugin({ maxChunks: 1 }),
        getProvidePlugin(),
    ],
    resolve: {
        extensions: ['.ts', '.tsx', '.js'],
        alias: {
            path: require.resolve('path-browserify'),
        },
    },
    module: {
        rules: [
            {
                test: /\.[jt]sx?$/,
                use: [getBabelLoader()],
            },
            {
                // SCSS rule for our own and third-party styles
                test: /\.(css|sass|scss)$/,
                exclude: /\.module\.(sass|scss)$/,
                use: getCSSLoaders(MiniCssExtractPlugin.loader, getBasicCSSLoader()),
            },
            {
                test: /\.(css|sass|scss)$/,
                include: /\.module\.(sass|scss)$/,
                use: getCSSLoaders(MiniCssExtractPlugin.loader, getCSSModulesLoader({ sourceMap: false, url: false })),
            },
            {
                test: /\.svg$/i,
                type: 'asset/inline',
            },
        ],
    },
})
