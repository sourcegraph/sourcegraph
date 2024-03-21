import { createMemoryHistory } from 'history'

import { MarkupKind } from '@sourcegraph/extension-api-classes'

import type { ActionItemAction } from '../actions/ActionItem'
import type { MarkupContent, Badged, AggregableBadge } from '../codeintel/legacy-extensions/api'
import { EMPTY_SETTINGS_CASCADE, type SettingsCascadeProps } from '../settings/settings'
import { NOOP_TELEMETRY_SERVICE } from '../telemetry/telemetryService'

import type { HoverOverlayProps } from './HoverOverlay'

const history = createMemoryHistory()
const NOOP_EXTENSIONS_CONTROLLER = { executeCommand: () => Promise.resolve() }

export const commonProps = (): HoverOverlayProps & SettingsCascadeProps => ({
    location: history.location,
    telemetryService: NOOP_TELEMETRY_SERVICE,
    extensionsController: NOOP_EXTENSIONS_CONTROLLER,
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
    linkURL: 'https://sourcegraph.com/docs/code_navigation/explanations/precise_code_navigation',
    hoverMessage: 'Sample hover message',
}

export const FIXTURE_ACTIONS: ActionItemAction[] = [
    {
        action: {
            id: 'goToDefinition.preloaded',
            title: 'Go to definition',
            command: 'open',
            commandArguments: ['/github.com/sourcegraph/codeintellify/-/blob/src/hoverifier.ts#L57:1'],
        },
        active: true,
    },
    {
        action: {
            id: 'findReferences',
            title: 'Find references',
            command: 'open',
            commandArguments: ['/github.com/sourcegraph/codeintellify/-/blob/src/hoverifier.ts?tab=references#L57:18'],
        },
        active: true,
    },
]
