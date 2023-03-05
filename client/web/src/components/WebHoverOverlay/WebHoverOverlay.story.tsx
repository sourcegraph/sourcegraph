import { DecoratorFn, Meta, Story } from '@storybook/react'

import { registerHighlightContributions } from '@sourcegraph/common'
import {
    commonProps,
    FIXTURE_ACTIONS,
    FIXTURE_SEMANTIC_BADGE,
    FIXTURE_CONTENT,
} from '@sourcegraph/shared/src/hover/HoverOverlay.fixtures'

import { WebStory } from '../WebStory'

import { WebHoverOverlay } from './WebHoverOverlay'
import {
    FIXTURE_CONTENT_LONG_CODE,
    FIXTURE_CONTENT_LONG_TEXT_ONLY,
    FIXTURE_CONTENT_MARKDOWN,
    FIXTURE_PARTIAL_BADGE,
} from './WebHoverOverlay.fixtures'

import styles from './WebHoverOverlay.story.module.scss'

registerHighlightContributions()

const decorator: DecoratorFn = story => <WebStory>{() => story()}</WebStory>

const config: Meta = {
    title: 'web/WebHoverOverlay',
    parameters: {
        design: {
            type: 'figma',
            url: 'https://www.figma.com/file/NIsN34NH7lPu04olBzddTw/Design-Refresh-Systemization-source-of-truth?node-id=2877%3A35469',
        },
        chromatic: {
            enableDarkMode: true,
            disableSnapshot: false,
        },
    },
    decorators: [decorator],
}

export default config

export const Loading: Story = () => (
    <WebHoverOverlay {...commonProps()} hoverOrError="loading" actionsOrError={FIXTURE_ACTIONS} />
)

export const _Error: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={
            new Error(
                'Something terrible happened: Eiusmod voluptate deserunt in sint cillum pariatur laborum eiusmod.'
            )
        }
        actionsOrError={FIXTURE_ACTIONS}
    />
)

_Error.storyName = 'Error'

export const NoHoverInformation: Story = () => (
    <WebHoverOverlay {...commonProps()} hoverOrError={null} actionsOrError={FIXTURE_ACTIONS} />
)

NoHoverInformation.storyName = 'No hover information'

export const CommonContentWithoutActions: Story = () => (
    <WebHoverOverlay {...commonProps()} hoverOrError={{ contents: [FIXTURE_CONTENT] }} />
)

CommonContentWithoutActions.storyName = 'Common content without actions'

export const CommonContentWithActions: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={{
            contents: [FIXTURE_CONTENT],
        }}
        actionsOrError={FIXTURE_ACTIONS}
    />
)

CommonContentWithActions.storyName = 'Common content with actions'

export const AggregatedBadges: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={{
            contents: [FIXTURE_CONTENT],
            aggregatedBadges: [FIXTURE_SEMANTIC_BADGE],
        }}
        actionsOrError={FIXTURE_ACTIONS}
    />
)

export const LongCode: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={{
            contents: [FIXTURE_CONTENT_LONG_CODE],
            aggregatedBadges: [FIXTURE_SEMANTIC_BADGE],
        }}
        actionsOrError={FIXTURE_ACTIONS}
    />
)

LongCode.storyName = 'Long code'

export const LongTextOnly: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={{
            contents: [FIXTURE_CONTENT_LONG_TEXT_ONLY],
        }}
        actionsOrError={FIXTURE_ACTIONS}
    />
)

LongTextOnly.storyName = 'Long text only'

export const LongMarkdownWithDiv: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={{
            contents: [FIXTURE_CONTENT_MARKDOWN],
        }}
        actionsOrError={FIXTURE_ACTIONS}
    />
)

LongMarkdownWithDiv.storyName = 'Long markdown with <div>'

export const MultipleMarkupContents: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={{
            contents: [FIXTURE_CONTENT, FIXTURE_CONTENT, FIXTURE_CONTENT],
            aggregatedBadges: [FIXTURE_SEMANTIC_BADGE],
        }}
        actionsOrError={FIXTURE_ACTIONS}
    />
)

MultipleMarkupContents.storyName = 'Multiple MarkupContents'

export const WithLongMarkdownTextIcon: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={{
            contents: [FIXTURE_CONTENT],
            aggregatedBadges: [FIXTURE_PARTIAL_BADGE, FIXTURE_SEMANTIC_BADGE],
        }}
        actionsOrError={FIXTURE_ACTIONS}
    />
)

WithLongMarkdownTextIcon.storyName = 'With long markdown text and icon.'

export const MultipleMarkupContentsWithBadges: Story = () => (
    <div className={styles.container}>
        <WebHoverOverlay
            {...commonProps()}
            hoverOrError={{
                contents: [FIXTURE_CONTENT, FIXTURE_CONTENT, FIXTURE_CONTENT],
                aggregatedBadges: [FIXTURE_SEMANTIC_BADGE],
            }}
            actionsOrError={FIXTURE_ACTIONS}
        />
    </div>
)

MultipleMarkupContentsWithBadges.storyName = 'Multiple MarkupContents with badges'

export const WithCloseButton: Story = () => (
    <WebHoverOverlay
        {...commonProps()}
        hoverOrError={{
            contents: [FIXTURE_CONTENT, FIXTURE_CONTENT, FIXTURE_CONTENT],
            aggregatedBadges: [FIXTURE_SEMANTIC_BADGE],
        }}
        actionsOrError={FIXTURE_ACTIONS}
        pinOptions={{ showCloseButton: true }}
    />
)

WithCloseButton.storyName = 'With close button'
