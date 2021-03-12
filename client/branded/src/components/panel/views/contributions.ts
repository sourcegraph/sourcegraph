import { Remote } from 'comlink'
import { Unsubscribable } from 'rxjs'
import { FlatExtensionHostAPI } from '../../../../../shared/src/api/contract'
import { syncRemoteSubscription } from '../../../../../shared/src/api/util'

export function registerPanelToolbarContributions(
    extensionHostAPI: Promise<Remote<FlatExtensionHostAPI>>
): Unsubscribable {
    return syncRemoteSubscription(
        extensionHostAPI.then(extensionHostAPI =>
            extensionHostAPI.registerContributions({
                actions: [
                    {
                        id: 'panel.locations.groupByFile',
                        title: 'Group by file',
                        category: 'Locations (panel)',
                        command: 'updateConfiguration',
                        commandArguments: [
                            ['panel.locations.groupByFile'],
                            // eslint-disable-next-line no-template-curly-in-string
                            '${!config.panel.locations.groupByFile}',
                            null,
                            'json',
                        ],
                        actionItem: {
                            // eslint-disable-next-line no-template-curly-in-string
                            label: '${config.panel.locations.groupByFile && "Ungroup" || "Group"} by file',
                        },
                    },
                ],
                menus: {
                    'panel/toolbar': [
                        {
                            action: 'panel.locations.groupByFile',
                            when: 'panel.locations.hasResults && panel.activeView.hasLocations',
                        },
                    ],
                },
            })
        )
    )
}
