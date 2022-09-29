import { noop } from 'lodash'
import { from, Observable } from 'rxjs'
import { catchError, map } from 'rxjs/operators'

import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { EventSource } from '@sourcegraph/shared/src/graphql-operations'

import { displayWarning } from '../settings/displayWarnings'
import { INSTANCE_VERSION_NUMBER_KEY, LocalStorageService } from '../settings/LocalStorageService'

import { requestGraphQLFromVSCode } from './requestGraphQl'

/**
 * Gets the Sourcegraph instance version number via the GrapQL API.
 *
 * @returns An Observable that emits flattened Sourcegraph instance version number or undefined in case of an error:
 * - regular instance version format: 3.38.2
 * - insiders version format: 134683_2022-03-02_5188fes0101
 */
export const observeInstanceVersionNumber = (): Observable<string | undefined> =>
    from(requestGraphQLFromVSCode<SiteVersionResult>(siteVersionQuery, {})).pipe(
        map(dataOrThrowErrors),
        map(data => data.site.productVersion),
        catchError(error => {
            console.error('Failed to get instance version from host:', error)
            return [undefined]
        })
    )

/**
 * Parses the Sourcegraph instance version number.
 *
 * @returns Major, minor and patch version numbers if it's a regular version, or `'insiders'` if it's an insiders version.
 */
export const parseVersion = (version: string): { major: number; minor: number; patch: number } | 'insiders' => {
    const versionParts = version.split('.')
    if (versionParts.length === 3) {
        return {
            major: parseInt(versionParts[0], 10),
            minor: parseInt(versionParts[1], 10),
            patch: parseInt(versionParts[2], 10),
        }
    }
    return 'insiders'
}

/**
 * This function will return the EventSource Type based
 * on the instance version
 */
export function initializeInstanceVersionNumber(
    localStorageService: LocalStorageService,
    instanceURL: string,
    accessToken: string | undefined
): EventSource {
    // Check only if a user is trying to connect to a private instance with a valid access token provided
    if (instanceURL !== 'https://sourcegraph.com' && accessToken) {
        observeInstanceVersionNumber()
            .toPromise()
            .then(async version => {
                if (version) {
                    const parsedVersion = parseVersion(version)
                    if (
                        parsedVersion !== 'insiders' &&
                        (parsedVersion.major < 3 || (parsedVersion.major === 3 && parsedVersion.minor < 32))
                    ) {
                        displayWarning(
                            'Your Sourcegraph instance version is not fully compatible with the Sourcegraph extension. Please ask your site admin to upgrade to version 3.32.0 or above. Read more about version support in our [troubleshooting docs](https://docs.sourcegraph.com/admin/how-to/troubleshoot-sg-extension#unsupported-features-by-sourcegraph-version).'
                        ).catch(() => {})
                    }
                    await localStorageService.setValue(INSTANCE_VERSION_NUMBER_KEY, version)
                }
            })
            .catch(noop) // We handle potential errors in instanceVersionNumber observable

        const version = localStorageService.getValue(INSTANCE_VERSION_NUMBER_KEY)
        const parsedVersion = parseVersion(version)
        // instances below 3.38.0 does not support EventSource.IDEEXTENSION and should fallback to BACKEND source
        return parsedVersion === 'insiders' ||
            parsedVersion.major > 3 ||
            (parsedVersion.major === 3 && parsedVersion.minor >= 38)
            ? EventSource.IDEEXTENSION
            : EventSource.BACKEND
    }
    return EventSource.IDEEXTENSION
}

const siteVersionQuery = gql`
    query SiteProductVersion {
        site {
            productVersion
        }
    }
`
interface SiteVersionResult {
    site: {
        productVersion: string
    }
}
