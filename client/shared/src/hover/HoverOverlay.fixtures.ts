import { createMemoryHistory } from 'history'
import { of } from 'rxjs'

import { MarkupKind } from '@sourcegraph/extension-api-classes'

import { ActionItemAction } from '../actions/ActionItem'
import type { MarkupContent, Badged, AggregableBadge } from '../codeintel/legacy-extensions/api'
import { PlatformContext } from '../platform/context'
import { EMPTY_SETTINGS_CASCADE, SettingsCascadeProps } from '../settings/settings'
import { NOOP_TELEMETRY_SERVICE } from '../telemetry/telemetryService'

import { HoverOverlayProps } from './HoverOverlay'

const history = createMemoryHistory()
const NOOP_PLATFORM_CONTEXT: Pick<PlatformContext, 'settings'> = {
    settings: of({ final: {}, subjects: [] }),
}

export const commonProps = (): HoverOverlayProps & SettingsCascadeProps => ({
    location: history.location,
    telemetryService: NOOP_TELEMETRY_SERVICE,
    platformContext: NOOP_PLATFORM_CONTEXT,
    overlayPosition: { top: 16, left: 16 },
    settingsCascade: EMPTY_SETTINGS_CASCADE,
})

export const FIXTURE_CONTENT: Badged<MarkupContent> = {
    value:
        '```go\nfunc RegisterMiddlewares(m ...*Middleware)\n```\n\n' +
        '---\n\nRegisterMiddlewares registers additional authentication middlewares. Currently this is used to register enterprise-only SSO middleware. This should only be called from an init function.\n',
    kind: MarkupKind.Markdown,
}

export const FIXTURE_SEMANTIC_BADGE: AggregableBadge = {
    text: 'semantic',
    linkURL: 'https://docs.sourcegraph.com/code_navigation/explanations/precise_code_navigation',
    hoverMessage: 'Sample hover message',
}

export const FIXTURE_ACTIONS: ActionItemAction[] = [
    {
        action: {
            id: 'goToDefinition',
            title: 'Go to definition',
            command: 'open',
            commandArguments: ['/github.com/sourcegraph/codeintellify/-/blob/src/hoverifier.ts?subtree=true#L57:1'],
        },
        active: true,
    },
    {
        action: {
            id: 'findReferences',
            title: 'Find references',
            command: 'open',
            commandArguments: [
                '/github.com/sourcegraph/codeintellify/-/blob/src/hoverifier.ts?subtree=true#L57:18&tab=references',
            ],
        },
        active: true,
    },
]
