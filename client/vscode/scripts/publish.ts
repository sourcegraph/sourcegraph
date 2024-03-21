/* eslint-disable @typescript-eslint/no-unsafe-member-access */

import childProcess from 'child_process'
import fs from 'fs'

import * as semver from 'semver'

import { version } from '../package.json'

/**
 * This script is used by the CI to publish the extension to the VS Code Marketplace
 * It is triggered when a commit has been made to the vsce release branch
 * Refer to our CONTRIBUTION docs to learn more about our release process
 */
// Get current package.json
const originalPackageJson = fs.readFileSync('package.json').toString()
/**
 * Build and publish the extension with the updated package name
 * using the tokens stored in the pipeline to run commands in pnpm
 * and allows all events to activate the extension
 */
const isPreRelease = semver.minor(version) % 2 !== 0 ? '--pre-release' : ''
// Tokens are stored in CI pipeline
const tokens = {
    vscode: process.env.VSCODE_MARKETPLACE_TOKEN,
    openvsx: process.env.VSCODE_OPENVSX_TOKEN,
}
// Assume this is for testing purpose if tokens are not found
const hasTokens = tokens.vscode !== undefined && tokens.openvsx !== undefined
const commands = {
    vscode_info: 'vsce show sourcegraph.sourcegraph --json',
    // To publish to VS Code Marketplace
    vscode_publish: `vsce publish ${isPreRelease} --pat $VSCODE_MARKETPLACE_TOKEN --no-dependencies --allow-star-activation`,
    // To package the extension without publishing
    vscode_package: `vsce package ${isPreRelease} --no-dependencies --allow-star-activation`,
    // To publish to the open-vsx registry
    openvsx_publish: 'npx --yes ovsx publish --no-dependencies -p $VSCODE_OPENVSX_TOKEN',
}
// Publish the extension with the correct extension name "sourcegraph"
try {
    childProcess.execSync('pnpm build-inline-extensions && pnpm build', { stdio: 'inherit' })
    // Get the latest release version nubmer of the last release from VS Code Marketplace using the vsce cli tool
    const response = childProcess.execSync(commands.vscode_info).toString()
    /*
        `vscode_info` command output is extension info prepended by command name and arguments, e.g.
        $ /Users/taras/projects/sourcegraph/node_modules/.bin/vsce show sourcegraph.sourcegraph --json
        {
            ...json
        }
        We cut everything before the json object so that we can parse it as json.
    */
    const trimmedResponse = response.slice(Math.max(0, response.indexOf('{')))
    const latestVersion: string = JSON.parse(trimmedResponse).versions[0].version
    if (hasTokens && (version === latestVersion || !semver.valid(latestVersion) || !semver.valid(version))) {
        throw new Error(
            version === latestVersion
                ? 'Cannot release extension with the same version number.'
                : `invalid version number: ${latestVersion} & ${version}`
        )
    }
    // Update package name from @sourcegraph/vscode to sourcegraph in package.json
    const packageJson = originalPackageJson.replace('@sourcegraph/vscode', 'sourcegraph')
    fs.writeFileSync('package.json', packageJson)
    if (hasTokens) {
        // Run the publish commands
        childProcess.execSync(commands.vscode_publish, { stdio: 'inherit' })
        childProcess.execSync(commands.openvsx_publish, { stdio: 'inherit' })
    } else {
        // Use vsce package command instead without publishing the extension for testing
        childProcess.execSync(commands.vscode_package, { stdio: 'inherit' })
    }
    console.log(`The extension has been ${hasTokens ? 'published' : 'packaged'} successfully.`)
} catch (error) {
    console.error('Failed to publish VSCE:', error)
    console.error('You may not run this script locally to publish the extension.')
} finally {
    // Revert changes made to package.json
    fs.writeFileSync('package.json', originalPackageJson)
}
