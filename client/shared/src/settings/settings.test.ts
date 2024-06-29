import { describe, expect, test } from 'vitest'

import { createAggregateError, isErrorLike } from '@sourcegraph/common'

import {
    gqlToCascade,
    merge,
    mergeSettings,
    type CustomMergeFunctions,
    type Settings,
    type SettingsCascade,
    type SettingsSubject,
} from './settings'

const FIXTURE_ORG: SettingsSubject = {
    __typename: 'Org',
    name: 'n',
    displayName: 'n',
    id: 'a',
    viewerCanAdminister: true,
    latestSettings: { id: 1, contents: '{"a":1}' },
    settingsURL: null,
}

const FIXTURE_USER: SettingsSubject = {
    __typename: 'User',
    username: 'n',
    displayName: 'n',
    id: 'b',
    viewerCanAdminister: true,
    latestSettings: { id: 2, contents: '{"b":2}' },
    settingsURL: null,
}

const FIXTURE_USER_WITH_SETTINGS_ERROR: SettingsSubject = {
    ...FIXTURE_USER,
    id: 'c',
    latestSettings: { id: 3, contents: '.' },
}

const SETTINGS_ERROR_FOR_FIXTURE_USER = createAggregateError([
    new Error('parse error (code: 1, error: InvalidSymbol, offset: 0, length: 1)'),
    new Error('parse error (code: 4, error: ValueExpected, offset: 1, length: 0)'),
])

describe('gqlToCascade', () => {
    test('converts a value', () => {
        const expected: SettingsCascade = {
            subjects: [
                { subject: FIXTURE_ORG, settings: { a: 1 }, lastID: 1 },
                { subject: FIXTURE_USER, settings: { b: 2 }, lastID: 2 },
            ],
            final: { a: 1, b: 2 },
        }
        expect(
            gqlToCascade({
                subjects: [FIXTURE_ORG, FIXTURE_USER],
            })
        ).toEqual(expected)
    })
    test('preserves errors', () => {
        const value = gqlToCascade({
            subjects: [FIXTURE_ORG, FIXTURE_USER_WITH_SETTINGS_ERROR, FIXTURE_USER],
        })
        expect(isErrorLike(value.final) && value.final.message).toBe(SETTINGS_ERROR_FOR_FIXTURE_USER.message)
        expect(
            value.subjects &&
                !isErrorLike(value.subjects) &&
                isErrorLike(value.subjects[1].settings) &&
                value.subjects[1].settings.message
        ).toBe(SETTINGS_ERROR_FOR_FIXTURE_USER.message)
    })
})

describe('mergeSettings', () => {
    test('handles an empty array', () => expect(mergeSettings([])).toBe(null))
    test('merges multiple values', () =>
        expect(mergeSettings<{ a?: number; b?: number } & Settings>([{ a: 1 }, { b: 2 }, { a: 3 }])).toEqual({
            a: 3,
            b: 2,
        }))
    test('deeply merges extensions property', () =>
        expect(
            mergeSettings<{ a?: { [key: string]: boolean }; b?: { [key: string]: boolean } } & Settings>([
                { extensions: { 'sourcegraph/cpp': true, 'sourcegraph/go': true, 'sourcegraph/typescript': true } },
                { extensions: { 'sourcegraph/cpp': false, 'sourcegraph/go': true, 'sourcegraph/typescript': true } },
                { extensions: { 'sourcegraph/go': false, 'sourcegraph/typescript': true } },
            ])
        ).toEqual({
            extensions: { 'sourcegraph/cpp': false, 'sourcegraph/go': false, 'sourcegraph/typescript': true },
        }))
    test('merges experimentalFeatures property', () =>
        expect(
            mergeSettings<Settings & { experimentalFeatures: { a?: boolean; b?: boolean; c?: boolean } }>([
                { experimentalFeatures: { a: true, b: true } },
                { experimentalFeatures: { b: false, c: true } },
            ])
        ).toEqual({
            experimentalFeatures: { a: true, b: false, c: true },
        }))
    test('merges search.scopes property', () =>
        expect(
            mergeSettings<
                {
                    a?: { [key: string]: { [key: string]: string }[] }
                    b?: { [key: string]: { [key: string]: string }[] }
                } & Settings
            >([
                { 'search.scopes': [{ name: 'test repos', value: 'repo:test' }] },
                { 'search.scopes': [{ name: 'sourcegraph repos', value: 'repo:sourcegraph' }] },
            ])
        ).toEqual({
            'search.scopes': [
                { name: 'test repos', value: 'repo:test' },
                { name: 'sourcegraph repos', value: 'repo:sourcegraph' },
            ],
        }))
    test('merges quicklinks property', () =>
        expect(
            mergeSettings<
                {
                    a?: { [key: string]: { [key: string]: string }[] }
                    b?: { [key: string]: { [key: string]: string }[] }
                } & Settings
            >([
                { quicklinks: [{ name: 'main repo', url: '/github.com/org/main-repo' }] },
                { quicklinks: [{ name: 'About Sourcegraph', url: 'https://docs.internal/about-sourcegraph' }] },
                {
                    quicklinks: [
                        { name: 'mycorp extensions', url: 'https://sourcegraph.com/extensions?query=mycorp%2F' },
                    ],
                },
            ])
        ).toEqual({
            quicklinks: [
                { name: 'main repo', url: '/github.com/org/main-repo' },
                { name: 'About Sourcegraph', url: 'https://docs.internal/about-sourcegraph' },
                { name: 'mycorp extensions', url: 'https://sourcegraph.com/extensions?query=mycorp%2F' },
            ],
        }))
    test('merges notices property', () =>
        expect(
            mergeSettings<{ a?: { [key: string]: string }; b?: { [key: string]: string } } & Settings>([
                {
                    notices: [
                        {
                            dismissible: false,
                            location: 'home',
                            message: 'global notice',
                        },
                    ],
                },
                {
                    notices: [
                        {
                            dismissible: false,
                            location: 'top',
                            message: 'org notice',
                        },
                    ],
                },
                {
                    notices: [
                        {
                            dismissible: false,
                            location: 'top',
                            message: 'user notice',
                        },
                    ],
                },
            ])
        ).toEqual({
            notices: [
                {
                    dismissible: false,
                    location: 'home',
                    message: 'global notice',
                },
                {
                    dismissible: false,
                    location: 'top',
                    message: 'org notice',
                },

                {
                    dismissible: false,
                    location: 'top',
                    message: 'user notice',
                },
            ],
        }))
    test('merges search.savedQueries property', () =>
        expect(
            mergeSettings<{ a?: { [key: string]: string }; b?: { [key: string]: string } } & Settings>([
                {
                    'search.savedQueries': [
                        {
                            key: '1',
                            description: 'global saved query',
                            query: 'type:diff global',
                        },
                    ],
                },
                {
                    'search.savedQueries': [
                        {
                            key: '2',
                            description: 'org saved query',
                            query: 'type:diff org',
                        },
                    ],
                },
                {
                    'search.savedQueries': [
                        {
                            key: '3',
                            description: 'user saved query',
                            query: 'type:diff user',
                        },
                    ],
                },
            ])
        ).toEqual({
            'search.savedQueries': [
                {
                    key: '1',
                    description: 'global saved query',
                    query: 'type:diff global',
                },
                {
                    key: '2',
                    description: 'org saved query',
                    query: 'type:diff org',
                },
                {
                    key: '3',
                    description: 'user saved query',
                    query: 'type:diff user',
                },
            ],
        }))
})

describe('merge', () => {
    function assertMerged(base: any, add: any, expected: any, custom?: CustomMergeFunctions): void {
        merge(base, add, custom)
        expect(base).toEqual(expected)
    }

    test('merges with empty', () => {
        assertMerged({ a: 1 }, {}, { a: 1 })
        assertMerged({}, { a: 1 }, { a: 1 })
    })
    test('merges top-level objects deeply', () => assertMerged({ a: 1 }, { b: 2 }, { a: 1, b: 2 }))
    test('does not merge nested objects deeply', () => assertMerged({ a: { b: 1 } }, { a: { c: 2 } }, { a: { c: 2 } }))
    test('overwrites arrays', () => assertMerged({ a: [1] }, { a: [2] }, { a: [2] }))
    test('uses custom merge functions', () =>
        assertMerged({ a: [1] }, { a: [2] }, { a: [1, 2] }, { a: (base, add) => [...base, ...add] }))
})
