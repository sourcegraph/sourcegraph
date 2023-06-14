/* eslint-disable no-sync */
import childProcess from 'child_process'

import * as semver from 'semver'

// eslint-disable-next-line  @typescript-eslint/no-require-imports, @typescript-eslint/no-var-requires
const { version } = require('../package.json')

/**
 * This script is used by the CI to publish the extension to the VS Code Marketplace.
 * Stable release is triggered when a commit has been made to the `cody/release` branch
 * Nightly release is triggered by CI nightly and is built from the `main` branch
 *
 * NOTE: Release numbers for Stable Build and Nightly Build are different.
 * Release numbers for Stable Build must have an even minor number.
 * Release numbers for Nightly Build must have an odd minor number and release as pre-release.
 * All release types are triggered by the CI and should not be run locally.
 * Release numbers for Nightly Build are automatically generated by the CI and should not be manually changed.
 * Release numbers for Stable Build are manually updated in package.json and should not be changed by the CI.
 *
 * Refer to our CONTRIBUTION docs to learn more about our release process.
 */

/**
 * Build and publish the extension with the updated package name using the tokens stored in the
 * pipeline to run commands in pnpm and allows all events to activate the extension.
 *
 * releaseType avilable in CI: stable, nightly
 */
const releaseType = process.env.CODY_RELEASE_TYPE

// Tokens are stored in CI pipeline
const tokens = {
    vscode: releaseType === 'dry-run' ? 'dry-run' : process.env.VSCODE_MARKETPLACE_TOKEN,
    openvsx: releaseType === 'dry-run' ? 'dry-run' : process.env.VSCODE_OPENVSX_TOKEN,
}

// Assume this is for testing purpose if tokens are not found
const hasTokens = tokens.vscode !== undefined && tokens.openvsx !== undefined
if (!hasTokens) {
    throw new Error('Cannot publish extension without tokens.')
}

// Set the version number for today's nightly build.
// The minor number should be the current minor number plus 1.
// The patch number should be today's date while major and minor should reminds the same as package.json version.
// Example: 1.0.0 in package.json -> 1.1.today's date -> 1.1.20210101
// Get today's date for nightly build. Example: 2021-01-01 = 20210101
const today = new Date().toISOString().slice(0, 10).replace(/-/g, '')
const currentVersion = semver.valid(version)
if (!currentVersion) {
    throw new Error('Cannot get the current version number from package.json')
}
if (semver.minor(currentVersion) % 2 !== 0) {
    throw new Error('Current minor number for stable release must be an even number: ' + currentVersion)
}
const tonightVersion = semver.inc(currentVersion, 'minor')?.replace(/\.\d+$/, `.${today}`)
if (!tonightVersion) {
    throw new Error("Could not populate the current version number for tonight's build.")
}

export const commands = {
    // Get the latest release version number of the last release from VS Code Marketplace
    vscode_info: 'vsce show sourcegraph.cody-ai --json',
    // Stable: publish to VS Code Marketplace
    vscode_package: 'pnpm run vsce:package',
    vscode_publish: 'vsce publish --packagePath dist/cody.vsix --pat $VSCODE_MARKETPLACE_TOKEN',
    // Nightly release: publish to VS Code Marketplace with today's date as patch number
    vscode_package_nightly: `pnpm --silent build && vsce package ${tonightVersion} --pre-release --no-dependencies -o dist/cody.vsix`,
    vscode_nightly: 'vsce publish --pre-release --packagePath dist/cody.vsix --pat $VSCODE_MARKETPLACE_TOKEN',
    // To publish to the open-vsx registry
    openvsx_publish: 'npx ovsx publish dist/cody.vsix --pat $VSCODE_OPENVSX_TOKEN',
}

// Build and bundle the extension
childProcess.execSync('pnpm run download-rg', { stdio: 'inherit' })
childProcess.execSync(releaseType === 'nightly' ? commands.vscode_package_nightly : commands.vscode_package, {
    stdio: 'inherit',
})

// Run the publish commands based on the release type
switch (releaseType) {
    case 'nightly':
        // if minor is not an odd number, throw an error
        if (
            semver.minor(tonightVersion) - semver.minor(currentVersion) !== 1 ||
            semver.minor(tonightVersion) % 2 === 0
        ) {
            throw new Error('Cannot publish nightly build with an even minor number: ' + tonightVersion)
        }
        // check if tonightVersion is a valid semv version number
        if (!tonightVersion || !semver.valid(tonightVersion) || semver.valid(tonightVersion) === currentVersion) {
            throw new Error('Cannot publish nightly build with an invalid version number: ' + tonightVersion)
        }
        // Publish to VS Code Marketplace with today's date as patch number
        childProcess.execSync(commands.vscode_nightly, { stdio: 'inherit' })
        break
    case 'stable':
        // Publish to VS Code Marketplace as the version number listed in package.json
        childProcess.execSync(commands.vscode_publish, { stdio: 'inherit' })
        // Publish to Open VSX Marketplace
        childProcess.execSync(commands.openvsx_publish, { stdio: 'inherit' })
        break
    case 'dry-run':
        console.info(`Current version: ${currentVersion}.`)
        console.info(`Pre-release version for tonight's build: ${tonightVersion}.`)
        if (!semver.valid(tonightVersion) || semver.minor(tonightVersion) % 2 === 0) {
            throw new Error('The nightly build will fail due to invalid version number.')
        }
        break
    default:
        throw new Error(`Invalid release type: ${releaseType}`)
}

console.log(releaseType === 'dry-run' ? 'Dry run completed.' : 'The extension has been published successfully.')
