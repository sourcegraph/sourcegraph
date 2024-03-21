/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */

import childProcess from 'child_process'
import fs from 'fs'

import * as semver from 'semver'

import { version } from '../package.json'

/**
 * This script updates the version number and changelog for release purpose
 */
// Get the current package.json and changelog files
const originalPackageJson = fs.readFileSync('package.json').toString()
const originalChangelogFile = fs.readFileSync('CHANGELOG.md').toString()
// Only proceed the release steps if a valid release type is provided
const vsceReleaseType = String(process.env.VSCE_RELEASE_TYPE).toLowerCase() as semver.ReleaseType
const isValidType = ['major', 'minor', 'patch', 'prerelease'].includes(vsceReleaseType)
if (isValidType) {
    /**
     * Generate the next version number for release purpose
     * prerelease is treated as minor update because the prerelease
     * tag in semver is not supported by VS Code
     * ref: https://code.visualstudio.com/api/working-with-extensions/publishing-extension#prerelease-extensions
     */
    const releaseType: semver.ReleaseType = vsceReleaseType === 'prerelease' ? 'minor' : vsceReleaseType
    // Get the latest release version nubmer of the last release from VS Code Marketplace using the vsce cli tool
    // We use this to compare the current package version number with the current release version number - they should be the same
    const response = childProcess.execSync('vsce show sourcegraph.sourcegraph --json').toString()
    const latestVersion: string = JSON.parse(response).versions[0].version
    if (!semver.valid(latestVersion)) {
        throw new Error(
            'Failed to connect to VSCE to retreive the latest extension version number. Make sure you have vsce cli tool installed.'
        )
    }
    if (version !== latestVersion) {
        throw new Error('The current version number is not align with the version number of the latest release.')
    }
    // Increase minor version number by twice for minor release because ODD minor number is for pre-release
    const nextVersion =
        vsceReleaseType === 'minor'
            ? semver.inc(semver.inc(latestVersion, releaseType)!, releaseType)!
            : semver.inc(latestVersion, releaseType)!
    // commit message for the release, eg. vsce: minor release v1.0.1
    if (nextVersion && nextVersion !== latestVersion) {
        try {
            // Update version number in package.json
            const packageJson = originalPackageJson.replace(`"version": "${version}"`, `"version": "${nextVersion}"`)
            fs.writeFileSync('package.json', packageJson)
            // Update Changelog with the new version number
            const changelogFile = originalChangelogFile.replace(
                'Unreleased',
                `Unreleased\n\n### Changes\n\n### Fixes\n\n## ${nextVersion}`
            )
            fs.writeFileSync('CHANGELOG.md', changelogFile)
            // Commit and push
            const releaseCommitMessage = `vsce: ${releaseType} release v${nextVersion}`
            childProcess.execSync(`git add . && git commit -m "${releaseCommitMessage}" && git push -u origin HEAD`, {
                stdio: 'inherit',
            })
        } catch (error) {
            console.error(`Publish VSCE with ${releaseType} failed:`, error)
            // Make sure any changes made are reverted if an error has occured
            fs.writeFileSync('package.json', originalPackageJson)
            fs.writeFileSync('CHANGELOG.md', originalChangelogFile)
        }
    } else {
        throw new Error('Version number is invalid.')
    }
} else {
    throw new Error(`Release type is invalid:  ${vsceReleaseType}`)
}
