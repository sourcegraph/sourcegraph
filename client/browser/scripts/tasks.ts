/* eslint no-sync: warn */
import fs from 'fs'
import path from 'path'

import { omit, cloneDeep, curry } from 'lodash'
import shelljs from 'shelljs'
import signale from 'signale'
import utcVersion from 'utc-version'
import { Configuration } from 'webpack'

import manifestSpec from '../src/browser-extension/manifest.spec.json'
import schema from '../src/browser-extension/schema.json'

/**
 * If true, add <all_urls> to the permissions in the manifest.
 * This is needed for e2e and integration tests because it is not possible to accept the
 * permission prompt with puppeteer.
 */
const EXTENSION_PERMISSIONS_ALL_URLS = Boolean(
    process.env.EXTENSION_PERMISSIONS_ALL_URLS && JSON.parse(process.env.EXTENSION_PERMISSIONS_ALL_URLS)
)

export type BuildEnvironment = 'dev' | 'prod'

type Browser = 'firefox' | 'chrome' | 'safari' | 'edge'

const BUILDS_DIR = 'build'

/*
 * Use a UTC-timestamp-based as the version string, generated at build-time.
 *
 * If enabled, the version string will depend on the timestamp when building, so
 * it will vary with every build. Uses the `utc-version` module.
 *
 * To get a reproducible build, disable this and set a version manually in
 * `manifest.spec.json`.
 */
const useUtcVersion = true

export const WEBPACK_STATS_OPTIONS: Configuration['stats'] = {
    all: false,
    timings: true,
    errors: true,
    warnings: true,
    colors: true,
}

function ensurePaths(browser?: Browser): void {
    shelljs.mkdir('-p', 'build/dist')
    shelljs.mkdir('-p', 'build/bundles')
    if (browser) {
        shelljs.mkdir('-p', `build/${browser}`)
    } else {
        shelljs.mkdir('-p', 'build/chrome')
        shelljs.mkdir('-p', 'build/edge')
        shelljs.mkdir('-p', 'build/firefox')
        shelljs.mkdir('-p', 'build/safari')
    }
}

/**
 * Create the Safari extension app Xcode project and build it.
 */
function buildSafariExtensionApp(): void {
    const safariWebExtensionConverterOptions = [
        'build/safari',
        '--project-location',
        'build/',
        '--app-name',
        '"Sourcegraph for Safari"',
        '--bundle-identifier',
        '"com.sourcegraph.Sourcegraph-for-Safari"',
        '--copy-resources',
        '--swift',
        '--force',
        '--no-open',
    ]

    const xcodebuildOptions = [
        '-quiet',
        '-project',
        '"./build/Sourcegraph for Safari/Sourcegraph for Safari.xcodeproj"',
        'build',
    ]

    shelljs.echo('y').exec(`xcrun safari-web-extension-converter ${safariWebExtensionConverterOptions.join(' ')}`)
    shelljs.exec(`xcodebuild ${xcodebuildOptions.join(' ')}`)
    shelljs.mv('./build/Sourcegraph for Safari/build/Release/Sourcegraph for Safari.app', './build/bundles')
}

export function copyAssets(): void {
    signale.await('Copy assets')
    const directory = 'build/dist'
    shelljs.rm('-rf', directory)
    shelljs.mkdir('-p', directory)
    shelljs.cp('-R', 'assets/*', directory)
    shelljs.cp('-R', 'src/browser-extension/pages/*', directory)
    signale.success('Assets copied')
}

function copyExtensionAssets(toDirectory: string): void {
    shelljs.mkdir('-p', `${toDirectory}/js`, `${toDirectory}/css`, `${toDirectory}/img`)
    shelljs.cp('build/dist/js/*.bundle.js', `${toDirectory}/js`)
    shelljs.cp('build/dist/css/*.bundle.css', `${toDirectory}/css`)
    shelljs.cp('-R', 'build/dist/img/*', `${toDirectory}/img`)
    shelljs.cp('build/dist/*.html', toDirectory)
}

/**
 * When building with inline (bundled) Sourcegraph extensions, copy the built Sourcegraph extensions into the output.
 * They will be available as `web_accessible_resources`.
 *
 * The pre-requisite step is to first clone, build, and copy into `build/extensions`.
 */
export function copyInlineExtensions(toDirectory: string): void {
    shelljs.cp('-R', 'build/extensions', toDirectory)
}

export function copyIntegrationAssets(): void {
    shelljs.mkdir('-p', 'build/integration/scripts')
    shelljs.mkdir('-p', 'build/integration/css')
    shelljs.cp('build/dist/js/phabricator.bundle.js', 'build/integration/scripts')
    shelljs.cp('build/dist/js/integration.bundle.js', 'build/integration/scripts')
    shelljs.cp('build/dist/css/style.bundle.css', 'build/integration/css')
    shelljs.cp('build/dist/css/inject.bundle.css', 'build/integration/css')
    // Copy to the ui/assets directory so that these files can be served by
    // the webapp.
    shelljs.mkdir('-p', '../../ui/assets/extension')
    shelljs.cp('-r', 'build/integration/*', '../../ui/assets/extension')
}

const BROWSER_TITLES = {
    firefox: 'Firefox',
    chrome: 'Chrome',
    edge: 'Edge',
    safari: 'Safari',
} as const

/**
 * Names of the zipped bundles for the browsers that use a zipped bundle.
 */
const BROWSER_BUNDLE_ZIPS: Partial<Record<Browser, string>> = {
    firefox: 'firefox-bundle.xpi',
    chrome: 'chrome-bundle.zip',
    edge: 'edge-bundle.zip',
}

/**
 * Fields to exclude from the manifest, for each browser.
 */
const BROWSER_BLOCKLIST = {
    chrome: ['applications'] as const,
    edge: ['applications'] as const,
    firefox: ['key'] as const,
    safari: [] as const,
}

function writeSchema(writeDirectory: string): void {
    fs.writeFileSync(`${writeDirectory}/schema.json`, JSON.stringify(schema, null, 4))
}

const version = process.env.BROWSER_EXTENSION_VERSION || utcVersion()

function writeManifest(environment: BuildEnvironment, browser: Browser, writeDirectory: string): void {
    const extensionInfo = cloneDeep(manifestSpec)
    const manifest = {
        ...omit(extensionInfo, ['dev', 'prod', ...BROWSER_BLOCKLIST[browser]]),
        ...omit(extensionInfo[environment], BROWSER_BLOCKLIST[browser]),
    }

    if (EXTENSION_PERMISSIONS_ALL_URLS) {
        manifest.permissions!.push('<all_urls>')
        /** Set key to make extension id deterministic */
        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        // @ts-ignore
        manifest.key = manifestSpec.dev.key
        signale.info('Adding <all_urls> to permissions because of env var setting')
    }

    if (browser === 'firefox') {
        manifest.permissions!.push('<all_urls>')
        delete manifest.storage
    }

    if (browser === 'safari') {
        // If any modifications need to be done to the manifest for Safari, they
        // can be done here.
        if (manifest.description!.length > 112) {
            throw new Error('Manifest description field cannot be longer than 112 characters. (Safari)')
        }
    }

    // Add the inline extensions to web accessible resources
    manifest.web_accessible_resources = manifest.web_accessible_resources || []
    manifest.web_accessible_resources.push('extensions/*')

    delete manifest.$schema

    if (environment === 'prod' && useUtcVersion) {
        manifest.version = version
    }

    fs.writeFileSync(`${writeDirectory}/manifest.json`, JSON.stringify(manifest, null, 4))
}

const buildForBrowser = curry((browser: Browser, environment: BuildEnvironment): (() => void) => () => {
    // Allow only building for specific browser targets.
    // Useful in local dev for faster builds.
    if (process.env.TARGETS && !process.env.TARGETS.includes(browser)) {
        return
    }

    const title = BROWSER_TITLES[browser]
    signale.await(`Building the ${title} ${environment} bundle`)

    ensurePaths(browser)

    const buildDirectory = path.resolve(process.cwd(), `${BUILDS_DIR}/${browser}`)

    writeManifest(environment, browser, buildDirectory)
    writeSchema(buildDirectory)

    copyExtensionAssets(buildDirectory)
    copyInlineExtensions(buildDirectory)

    // Create a bundle by zipping the web extension directory.
    const browserBundleZip = BROWSER_BUNDLE_ZIPS[browser]
    if (browserBundleZip) {
        const zipDestination = path.resolve(process.cwd(), `${BUILDS_DIR}/bundles/${browserBundleZip}`)
        if (zipDestination) {
            shelljs.mkdir('-p', `./${BUILDS_DIR}/bundles`)
            shelljs.exec(`cd ${buildDirectory} && zip -q -r ${zipDestination} *`)
        }
    }

    // Safari-specific build step
    if (browser === 'safari') {
        buildSafariExtensionApp()
    }

    signale.success(`Done building the ${title} ${environment} bundle`)
})

export const buildFirefox = buildForBrowser('firefox')
export const buildChrome = buildForBrowser('chrome')
export const buildEdge = buildForBrowser('edge')
export const buildSafari = buildForBrowser('safari')
