import { firstValueFrom } from 'rxjs'

import type { KeyPath } from '@sourcegraph/client-api'

import type { PlatformContext } from '../../../platform/context'
import { isSettingsValid } from '../../../settings/settings'

/**
 * An edit to apply to settings.
 */
export interface SettingsEdit {
    /** The key path to the value. */
    path: KeyPath

    /** The new value to insert at the key path. */
    value: any
}
/**
 *
 * @todo move that to the platform context itself.
 * the code below doesn't seem to nessesery (simon)
 * @todo  also it doesn't check if subjects has any elements
 *
 */
export async function updateSettings(
    platformContext: Pick<PlatformContext, 'settings' | 'updateSettings'>,
    edit: SettingsEdit
): Promise<void> {
    const { settings: data, updateSettings: update } = platformContext
    const settings = await firstValueFrom(data)
    if (!isSettingsValid(settings)) {
        throw new Error('invalid settings (internal error)')
    }
    const subject = settings.subjects.at(-1)!
    await update(subject.subject.id, edit)
}
