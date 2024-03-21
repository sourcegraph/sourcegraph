import { action } from '@storybook/addon-actions'
import type { Decorator, Meta, StoryFn } from '@storybook/react'
import type * as H from 'history'

import { subtypeOf } from '@sourcegraph/common'
import { BrandedStory } from '@sourcegraph/wildcard/src/stories'

import { NOOP_TELEMETRY_SERVICE } from '../telemetry/telemetryService'

import { ActionItem, type ActionItemComponentProps, type ActionItemProps } from './ActionItem'

const EXTENSIONS_CONTROLLER: ActionItemComponentProps['extensionsController'] = {
    executeCommand: () => new Promise(resolve => setTimeout(resolve, 750)),
}

const LOCATION: H.Location = { hash: '', pathname: '/', search: '', state: undefined }

const ICON_URL =
    'data:image/svg+xml,' +
    encodeURIComponent(
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">🚀</text></svg>'
    )

const onDidExecute = action('onDidExecute')

const commonProps = subtypeOf<Partial<ActionItemProps>>()({
    location: LOCATION,
    extensionsController: EXTENSIONS_CONTROLLER,
    telemetryService: NOOP_TELEMETRY_SERVICE,
    iconClassName: 'icon-inline',
    active: true,
})

const decorator: Decorator = story => <BrandedStory>{() => <div className="p-4">{story()}</div>}</BrandedStory>

const config: Meta = {
    title: 'shared/ActionItem',
    decorators: [decorator],
}
export default config

export const NoopAction: StoryFn = () => (
    <ActionItem
        {...commonProps}
        action={{ id: 'a', command: undefined, actionItem: { label: 'Hello' } }}
        variant="actionItem"
    />
)

NoopAction.storyName = 'Noop action'

export const CommandAction: StoryFn = () => (
    <ActionItem
        {...commonProps}
        action={{ id: 'a', command: 'c', title: 'Hello', iconURL: ICON_URL }}
        telemetryService={NOOP_TELEMETRY_SERVICE}
        disabledDuringExecution={true}
        showLoadingSpinnerDuringExecution={true}
        onDidExecute={onDidExecute}
    />
)

CommandAction.storyName = 'Command action'
CommandAction.parameters = {
    chromatic: {
        enableDarkMode: true,
        disableSnapshot: false,
    },
}

export const LinkAction: StoryFn = () => (
    <ActionItem
        {...commonProps}
        action={{
            id: 'a',
            command: 'open',
            commandArguments: ['javascript:alert("link clicked")'],
            actionItem: { label: 'Hello' },
        }}
        variant="actionItem"
        onDidExecute={onDidExecute}
    />
)

LinkAction.storyName = 'Link action'

export const Executing: StoryFn = () => {
    class ActionItemExecuting extends ActionItem {
        constructor(props: ActionItem['props']) {
            super(props)
            // eslint-disable-next-line react/no-this-in-sfc
            this.state.actionOrError = 'loading'
        }
    }
    return (
        <ActionItemExecuting
            {...commonProps}
            action={{ id: 'a', command: 'c', title: 'Hello', iconURL: ICON_URL }}
            disabledDuringExecution={true}
            showLoadingSpinnerDuringExecution={true}
        />
    )
}

export const _Error: StoryFn = () => {
    class ActionItemWithError extends ActionItem {
        constructor(props: ActionItem['props']) {
            super(props)
            // eslint-disable-next-line react/no-this-in-sfc
            this.state.actionOrError = new Error('e')
        }
    }
    return (
        <ActionItemWithError
            {...commonProps}
            action={{ id: 'a', command: 'c', title: 'Hello', iconURL: ICON_URL }}
            disabledDuringExecution={true}
            showLoadingSpinnerDuringExecution={true}
        />
    )
}
