import { describe, expect, test } from 'vitest'

import { SettingsProvider } from '@sourcegraph/shared/src/settings/settings'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import { Notices } from './Notices'

describe('Notices', () => {
    test('shows notices for location', () =>
        expect(
            renderWithBrandedContext(
                <SettingsProvider
                    settingsCascade={{
                        subjects: [],
                        final: {
                            notices: [
                                { message: 'a', location: 'home' },
                                { message: 'a', location: 'home', dismissible: true },
                                { message: 'b', location: 'top' },
                                { message: 'a message with a variant', location: 'top', variant: 'note' },
                                {
                                    message: 'a message with style overrides',
                                    location: 'top',
                                    variant: 'success',
                                    styleOverrides: {
                                        backgroundColor: '#00f0ff',
                                        textCentered: true,
                                    },
                                },
                            ],
                        },
                    }}
                >
                    <Notices location="home" />
                    <Notices location="top" />
                </SettingsProvider>
            ).asFragment()
        ).toMatchSnapshot())

    test('no notices', () =>
        expect(
            renderWithBrandedContext(
                <SettingsProvider settingsCascade={{ subjects: [], final: { notices: undefined } }}>
                    <Notices location="home" />
                </SettingsProvider>
            ).asFragment()
        ).toMatchSnapshot())
})
