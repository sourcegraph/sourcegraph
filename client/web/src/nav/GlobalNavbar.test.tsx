import React from 'react'

import { describe, expect, test, vi, afterAll } from 'vitest'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'
import {
    mockFetchSearchContexts,
    mockGetUserSearchContextNamespaces,
} from '@sourcegraph/shared/src/testing/searchContexts/testHelpers'
import { NOOP_SETTINGS_CASCADE } from '@sourcegraph/shared/src/testing/searchTestHelpers'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import { GlobalNavbar } from './GlobalNavbar'

vi.mock('../search/input/SearchNavbarItem', () => ({ SearchNavbarItem: () => 'SearchNavbarItem' }))
vi.mock('../components/branding/BrandLogo', () => ({ BrandLogo: () => 'BrandLogo' }))

const PROPS: React.ComponentProps<typeof GlobalNavbar> = {
    authenticatedUser: null,
    isSourcegraphDotCom: false,
    platformContext: {} as any,
    settingsCascade: NOOP_SETTINGS_CASCADE,
    batchChangesEnabled: false,
    batchChangesExecutionEnabled: false,
    batchChangesWebhookLogsEnabled: false,
    telemetryService: {} as any,
    telemetryRecorder: noOpTelemetryRecorder,
    showSearchBox: true,
    selectedSearchContextSpec: '',
    setSelectedSearchContextSpec: () => undefined,
    branding: undefined,
    routes: [],
    searchContextsEnabled: true,
    fetchSearchContexts: mockFetchSearchContexts,
    getUserSearchContextNamespaces: mockGetUserSearchContextNamespaces,
    showKeyboardShortcutsHelp: () => undefined,
    setFuzzyFinderIsVisible: () => undefined,
    notebooksEnabled: true,
    codeMonitoringEnabled: true,
    ownEnabled: true,
    showFeedbackModal: () => undefined,
}

describe('GlobalNavbar', () => {
    afterAll(() => {
        vi.restoreAllMocks()
    })

    test('default', () => {
        vi.mock('../util/license', () => ({
            isCodeSearchOnlyLicense: () => false,
            isCodeSearchPlusCodyLicense: () => true,
            isCodyOnlyLicense: () => false,
        }))

        const { asFragment } = renderWithBrandedContext(
            <MockedTestProvider>
                <GlobalNavbar {...PROPS} />
            </MockedTestProvider>
        )
        expect(asFragment()).toMatchSnapshot()
    })

    test('cody only license', () => {
        vi.mock('../util/license', () => ({
            isCodeSearchOnlyLicense: () => false,
            isCodeSearchPlusCodyLicense: () => false,
            isCodyOnlyLicense: () => true,
        }))

        const { asFragment } = renderWithBrandedContext(
            <MockedTestProvider>
                <GlobalNavbar {...PROPS} />
            </MockedTestProvider>
        )
        expect(asFragment()).toMatchSnapshot()
    })

    test('code search only license', () => {
        vi.mock('../util/license', () => ({
            isCodeSearchOnlyLicense: () => true,
            isCodeSearchPlusCodyLicense: () => false,
            isCodyOnlyLicense: () => false,
        }))

        const { asFragment } = renderWithBrandedContext(
            <MockedTestProvider>
                <GlobalNavbar {...PROPS} />
            </MockedTestProvider>
        )
        expect(asFragment()).toMatchSnapshot()
    })
})
