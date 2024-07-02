import type { Meta, StoryFn, Decorator } from '@storybook/react'

import { Text, H2, Button, ButtonLink } from '@sourcegraph/wildcard'

import { WebStory } from '../../../src/components/WebStory'

import { CodyAlert } from './CodyAlert'
import { ProIcon } from './CodyIcon'

const decorator: Decorator = story => <div className="p-3 container">{story()}</div>

const config: Meta = {
    title: 'web/cody/PLG/CodyAlert',
    decorators: [decorator],
}

export default config

export const CodyProBanner: StoryFn = () => (
    <WebStory>
        {props => (
            <CodyAlert variant="purple" badge="CodyPro">
                <div>
                    <H2>
                        Get unlimited help with <span>Cody Pro</span>
                    </H2>
                    <div className="d-flex justify-content-between align-items-center">
                        <div className="w-100">
                            <ul className="pl-4 mb-0">
                                <li>Unlimited autocompletions</li>
                                <li>Unlimited chat messages</li>
                            </ul>
                        </div>
                        <div>
                            <ButtonLink to="/" variant="primary" size="sm">
                                <div className="textNowrap">
                                    <ProIcon className="mr-1" /> Upgrade now
                                </div>
                            </ButtonLink>
                        </div>
                    </div>
                </div>
            </CodyAlert>
        )}
    </WebStory>
)

export const AlertWithActions: StoryFn = () => (
    <WebStory>
        {props => (
            <CodyAlert variant="purple" title="Card with display card">
                <Text>
                    You've been invited to a new Cody Pro team by rob@biglike.com <br />
                    This will terminate your current Cody Pro plan, and place you on the new Cody Pro team. You will not
                    lose access to your Cody Pro benefits.
                </Text>
                <div className="mt-3">
                    <Button variant="primary" disabled={true} className="mr-3">
                        Accept
                    </Button>
                    <Button variant="link">Decline</Button>
                </div>
            </CodyAlert>
        )}
    </WebStory>
)

export const PurpulePro: StoryFn = () => (
    <WebStory>
        {props => (
            <CodyAlert variant="purple" badge={'CodyPro'} title="Card with display card">
                <Text>A success message</Text>
            </CodyAlert>
        )}
    </WebStory>
)

export const Purple: StoryFn = () => (
    <WebStory>
        {props => (
            <CodyAlert variant="purple" title="A Tile">
                <Text>Purple success message</Text>
            </CodyAlert>
        )}
    </WebStory>
)

export const Green: StoryFn = () => (
    <WebStory>
        {props => (
            <CodyAlert variant="green" title="A Tile">
                <Text>Success message</Text>
            </CodyAlert>
        )}
    </WebStory>
)

export const GreenPro: StoryFn = () => (
    <WebStory>
        {props => (
            <CodyAlert variant="green" badge={'CodyPro'} title="A Tile">
                <Text>Success message</Text>
            </CodyAlert>
        )}
    </WebStory>
)

export const Error: StoryFn = () => (
    <WebStory>
        {props => (
            <CodyAlert variant="error" badge={'Alert'} title="A Tile">
                <Text>A success message</Text>
            </CodyAlert>
        )}
    </WebStory>
)
