import type { FC } from 'react'

import type Mocha from 'mocha'
import { RouterProvider, createMemoryRouter, type MemoryRouterProps } from 'react-router-dom'

import { EMPTY_SETTINGS_CASCADE, SettingsProvider } from '@sourcegraph/shared/src/settings/settings'
import { MockedStoryProvider, type MockedStoryProviderProps } from '@sourcegraph/shared/src/stories'
import { NOOP_TELEMETRY_SERVICE, type TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { AutomockGraphQLProvider } from '@sourcegraph/shared/src/testing/apollo'
import { ThemeContext, ThemeSetting } from '@sourcegraph/shared/src/theme'
import { WildcardThemeContext } from '@sourcegraph/wildcard'
import { usePrependStyles, useStorybookTheme } from '@sourcegraph/wildcard/src/stories'

import type { SourcegraphContext } from '../jscontext'
import { type LegacyLayoutRouteContext, LegacyRouteContext } from '../LegacyRouteContext'

import { type BreadcrumbSetters, type BreadcrumbsProps, useBreadcrumbs } from './Breadcrumbs'
import { legacyLayoutRouteContextMock } from './legacyLayoutRouteContext.mock'

import webStyles from '../SourcegraphWebApp.scss'

// With `StoryStoreV7` stories are isolated and window value is not shared between them.
// Global variables should be updated for every story individually.
if (!window.context) {
    window.context = {} as SourcegraphContext & Mocha.SuiteFunction
}

export type WebStoryChildrenProps = BreadcrumbSetters &
    BreadcrumbsProps &
    TelemetryProps & {
        isLightTheme: boolean
    }

export interface WebStoryProps extends BaseWebStoryProps, Pick<MockedStoryProviderProps, 'mocks' | 'useStrictMocking'> {
    automock?: boolean
}

/**
 * Wrapper component for webapp Storybook stories that provides light theme and react-router props.
 * Takes a render function as children that gets called with the props.
 */
export const WebStory: FC<WebStoryProps> = ({ mocks, useStrictMocking, automock, ...props }) =>
    automock ? (
        <AutomockGraphQLProvider>
            <BaseWebStory {...props} />
        </AutomockGraphQLProvider>
    ) : (
        <MockedStoryProvider mocks={mocks} useStrictMocking={useStrictMocking}>
            <BaseWebStory {...props} />
        </MockedStoryProvider>
    )

/**
 * Wrapper component for webapp Storybook stories that provides light theme and react-router props.
 * Takes a render function as children that gets called with the props.
 */
export interface BaseWebStoryProps extends Omit<MemoryRouterProps, 'children'> {
    children: FC<WebStoryChildrenProps>
    path?: string
    legacyLayoutContext?: Partial<LegacyLayoutRouteContext>
}

export const BaseWebStory: FC<WebStoryProps> = ({
    children: Children,
    path = '*',
    initialEntries = ['/'],
    initialIndex = 1,
    legacyLayoutContext = {},
}) => {
    const isLightTheme = useStorybookTheme()
    const breadcrumbSetters = useBreadcrumbs()

    usePrependStyles('web-styles', webStyles)

    const routes = [
        {
            path,
            element: (
                <Children
                    {...breadcrumbSetters}
                    isLightTheme={isLightTheme}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                />
            ),
        },
    ]

    const router = createMemoryRouter(routes, {
        initialEntries,
        initialIndex,
    })

    return (
        <WildcardThemeContext.Provider value={{ isBranded: true }}>
            <LegacyRouteContext.Provider value={{ ...legacyLayoutRouteContextMock, ...legacyLayoutContext }}>
                <SettingsProvider settingsCascade={EMPTY_SETTINGS_CASCADE}>
                    <ThemeContext.Provider
                        value={{ themeSetting: isLightTheme ? ThemeSetting.Light : ThemeSetting.Dark }}
                    >
                        <RouterProvider router={router} />
                    </ThemeContext.Provider>
                </SettingsProvider>
            </LegacyRouteContext.Provider>
        </WildcardThemeContext.Provider>
    )
}
