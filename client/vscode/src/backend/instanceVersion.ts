import { noop } from 'lodash'
import { from, type Observable } from 'rxjs'
import { catchError, map } from 'rxjs/operators'

import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { EventSource } from '@sourcegraph/shared/src/graphql-operations'

import { displayWarning } from '../settings/displayWarnings'
import { INSTANCE_VERSION_NUMBER_KEY, type LocalStorageService } from '../settings/LocalStorageService'

import { requestGraphQLFromVSCode } from './requestGraphQl'

/**
 * Gets the Sourcegraph instance version number via the GrapQL API.
 *
 * @returns An Observable that emits flattened Sourcegraph instance version number or undefined in case of an error:
 * - regular instance version format: 3.38.2
 * - insiders version format: 134683_2022-03-02_5188fes0101
 */
export const observeInstanceVersionNumber = (
    accessToken: string,
    endpointURL: string
): Observable<string | undefined> =>
    from(requestGraphQLFromVSCode<SiteVersionResult>(siteVersionQuery, {}, accessToken, endpointURL)).pipe(
        map(dataOrThrowErrors),
        map(data => data.site.productVersion),
        catchError(error => {
            console.error('Failed to get instance version from host:', error)
            return [undefined]
        })
    )

interface RegularVersion {
    major: number
    minor: number
}

type Version = RegularVersion | 'insiders'

/**
 * Parses the Sourcegraph instance version number.
 *
 * @returns Major and minor version numbers if it's a regular version, or `'insiders'` if it's an insiders version.
 */
const parseVersion = (version: string): Version => {
    const versionParts = version.split('.')
    if (versionParts.length === 3) {
        return {
            major: parseInt(versionParts[0], 10),
            minor: parseInt(versionParts[1], 10),
        }
    }
    return 'insiders'
}

/**
 * Checks if the Sourcegraph instance version is older than the given version.
 * */
export const isOlderThan = (instanceVersion: string, comparedVersion: RegularVersion): boolean => {
    const version = parseVersion(instanceVersion)
    return (
        version !== 'insiders' &&
        (version.major < comparedVersion.major ||
            (version.major === comparedVersion.major && version.minor < comparedVersion.minor))
    )
}

/**
 * This function will return the EventSource Type based
 * on the instance version
 */
export function initializeInstanceVersionNumber(
    localStorageService: LocalStorageService,
    initialAccessToken: string | undefined,
    initialInstanceURL: string
): EventSource {
    // Check only if a user is trying to connect to a private instance with a valid access token provided
    if (initialAccessToken && initialAccessToken !== undefined) {
        observeInstanceVersionNumber(initialAccessToken, initialInstanceURL)
            .toPromise()
            .then(async version => {
                if (version) {
                    if (isOlderThan(version, { major: 3, minor: 32 })) {
                        displayWarning(
                            'Your Sourcegraph instance version is not fully compatible with the Sourcegraph extension. Please ask your site admin to upgrade to version 3.32.0 or above. Read more about version support in our [troubleshooting docs](https://docs.sourcegraph.com/admin/how-to/troubleshoot-sg-extension#unsupported-features-by-sourcegraph-version).'
                        ).catch(() => {})
                    }
                    await localStorageService.setValue(INSTANCE_VERSION_NUMBER_KEY, version)
                }
            })
            .catch(noop) // We handle potential errors in instanceVersionNumber observable

        const version = localStorageService.getValue(INSTANCE_VERSION_NUMBER_KEY)
        // instances below 3.38.0 does not support EventSource.IDEEXTENSION and should fallback to BACKEND source
        return version && isOlderThan(version, { major: 3, minor: 38 }) ? EventSource.BACKEND : EventSource.IDEEXTENSION
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
