import { Meta, Story } from '@storybook/react'

import { BrandedStory } from '../../stories'

import { Progress } from './Progress'

const config: Meta = {
    title: 'wildcard/Progress',
    component: Progress,
    decorators: [story => <BrandedStory>{() => story()}</BrandedStory>],
}

export default config

export const ProgressGallery: Story = () => (
    <div
        style={{
            width: 400,
            display: 'flex',
            gap: 50,
            padding: 50,
            flexDirection: 'column',
            alignItems: 'center',
            justifyItems: 'center',
        }}
    >
        <Progress value={0} />
        <Progress value={30} />
        <Progress value={50} />
    </div>
)
