import { lastValueFrom } from 'rxjs'
import { distinctUntilChanged, switchMap, take, toArray } from 'rxjs/operators'
import { describe, test } from 'vitest'

import { fromSubscribable } from '@sourcegraph/common'
import { Selection } from '@sourcegraph/extension-api-classes'

import { assertToJSON, integrationTestContext } from '../../testing/testHelpers'

describe('CodeEditor (integration)', () => {
    describe('selection', () => {
        test('observe changes', async () => {
            const { extensionAPI, extensionHostAPI } = await integrationTestContext()

            const values = lastValueFrom(
                fromSubscribable(extensionAPI.app.activeWindow!.activeViewComponentChanges).pipe(
                    switchMap(viewer =>
                        viewer && viewer.type === 'CodeEditor' ? fromSubscribable(viewer.selectionsChanges) : []
                    ),
                    distinctUntilChanged(),
                    take(3),
                    toArray()
                )
            )

            await extensionHostAPI.setEditorSelections({ viewerId: 'viewer#0' }, [new Selection(1, 2, 3, 4)])
            await extensionHostAPI.setEditorSelections({ viewerId: 'viewer#0' }, [])

            assertToJSON(
                (await values).map(selections => selections.map(selection => Selection.fromPlain(selection).toPlain())),
                [[], [new Selection(1, 2, 3, 4).toPlain()], []]
            )
        })
    })
})
