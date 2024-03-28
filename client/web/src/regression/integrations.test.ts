import { describe, test } from 'mocha'
import { lastValueFrom, merge } from 'rxjs'
import { fromFetch } from 'rxjs/fetch'
import { catchError } from 'rxjs/operators'

import { checkOk } from '@sourcegraph/http-client'
import { getConfig } from '@sourcegraph/shared/src/testing/config'

describe('Native integrations regression test suite', () => {
    const { sourcegraphBaseUrl } = getConfig('sourcegraphBaseUrl')
    test('Native integration assets are served by the instance', async () => {
        const assets = [
            '/.assets/extension/scripts/integration.bundle.js',
            '/.assets/extension/scripts/phabricator.bundle.js',
            '/.assets/extension/scripts/extensionHostWorker.bundle.js',
            '/.assets/extension/css/app.bundle.css',
            '/.assets/extension/css/contentPage.main.bundle.css',
            '/.assets/extension/extensionHostFrame.html',
        ]
        await lastValueFrom(
            merge(
                ...assets.map(asset =>
                    fromFetch(new URL(asset, sourcegraphBaseUrl).href, {
                        selector: response => [checkOk(response)],
                    }).pipe(
                        catchError(() => {
                            throw new Error('Error fetching native integration asset: ' + asset)
                        })
                    )
                )
            )
        )
    })
})
