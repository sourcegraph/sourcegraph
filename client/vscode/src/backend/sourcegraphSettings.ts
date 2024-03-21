import { type Observable, of, ReplaySubject, Subject } from 'rxjs'
import { catchError, map, switchMap, throttleTime } from 'rxjs/operators'
import type * as vscode from 'vscode'

import { createAggregateError } from '@sourcegraph/common'
import { viewerSettingsQuery } from '@sourcegraph/shared/src/backend/settings'
import type { ViewerSettingsResult, ViewerSettingsVariables } from '@sourcegraph/shared/src/graphql-operations'
import {
    EMPTY_SETTINGS_CASCADE,
    gqlToCascade,
    type Settings,
    type SettingsCascadeOrError,
} from '@sourcegraph/shared/src/settings/settings'

import { requestGraphQLFromVSCode } from './requestGraphQl'

export function initializeSourcegraphSettings({ context }: { context: vscode.ExtensionContext }): {
    settings: Observable<SettingsCascadeOrError<Settings>>
    refreshSettings: () => void
} {
    const settings = new ReplaySubject<SettingsCascadeOrError<Settings>>(1)

    const refreshes = new Subject<void>()

    // Throttle refreshes for one hour.
    const ONE_HOUR_MS = 60 * 60 * 1000

    const subscription = refreshes
        .pipe(
            throttleTime(ONE_HOUR_MS, undefined, { leading: true, trailing: true }),
            switchMap(() =>
                requestGraphQLFromVSCode<ViewerSettingsResult, ViewerSettingsVariables>(viewerSettingsQuery, {})
            ),
            map(({ data, errors }) => {
                if (!data?.viewerSettings) {
                    throw createAggregateError(errors)
                }

                return gqlToCascade(data.viewerSettings)
            }),
            catchError(() => of(EMPTY_SETTINGS_CASCADE))
        )
        .subscribe(settingsCascade => {
            settings.next(settingsCascade)
        })
    context.subscriptions.push({ dispose: () => subscription.unsubscribe() })

    // Initial settings
    refreshes.next()

    return {
        settings: settings.asObservable(),
        refreshSettings: () => {
            refreshes.next()
        },
    }
}
