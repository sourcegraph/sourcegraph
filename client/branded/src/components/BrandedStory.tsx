import React from 'react'
import { MemoryRouter, MemoryRouterProps } from 'react-router'

import { ThemeProps } from '@sourcegraph/shared/src/theme'
import { MockedStoryProvider, MockedStoryProviderProps } from '@sourcegraph/storybook/src/apollo/MockedStoryProvider'
import { usePrependStyles } from '@sourcegraph/storybook/src/hooks/usePrependStyles'
import { useTheme } from '@sourcegraph/storybook/src/hooks/useTheme'

import brandedStyles from '../global-styles/index.scss'

import { Tooltip } from './tooltip/Tooltip'

export interface BrandedProps extends MemoryRouterProps, Pick<MockedStoryProviderProps, 'mocks' | 'useStrictMocks'> {
    children: React.FunctionComponent<ThemeProps>
    styles?: string
}

/**
 * Wrapper component for branded Storybook stories that provides light theme and react-router props.
 * Takes a render function as children that gets called with the props.
 */
export const BrandedStory: React.FunctionComponent<BrandedProps> = ({
    children: Children,
    styles = brandedStyles,
    mocks,
    useStrictMocks,
    ...memoryRouterProps
}) => {
    const isLightTheme = useTheme()
    usePrependStyles('branded-story-styles', styles)

    return (
        <MockedStoryProvider mocks={mocks} useStrictMocks={useStrictMocks}>
            <MemoryRouter {...memoryRouterProps}>
                <Tooltip />
                <Children isLightTheme={isLightTheme} />
            </MemoryRouter>
        </MockedStoryProvider>
    )
}
