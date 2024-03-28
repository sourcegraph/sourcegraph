import { describe, expect, test } from 'vitest'

import { getFirstNameAndLastInitial, extractPRNumber, truncateIfNeeded, extractCommitMessage } from './utils'

describe('getFirstNameAndLastInitial', () => {
    const tests: {
        title: string
        input: string
        expected: string
    }[] = [
        {
            title: 'returns first name only if no last name',
            input: 'John',
            expected: 'John',
        },
        {
            title: 'returns first name and last initial',
            input: 'John Smith',
            expected: 'John S.',
        },
        {
            title: 'capitalizes correctly',
            input: 'john',
            expected: 'John',
        },
        {
            title: 'capitalizes correctly',
            input: 'john smith',
            expected: 'John S.',
        },
        {
            title: 'capitalizes correctly',
            input: 'John smith',
            expected: 'John S.',
        },
        {
            title: 'capitalizes correctly',
            input: 'jOhN sMiTH',
            expected: 'John S.',
        },
    ]

    for (const tc of tests) {
        test(tc.title, () => {
            let got = getFirstNameAndLastInitial(tc.input)
            expect(got).toBe(tc.expected)
        })
    }
})

describe('extractPRNumber', () => {
    const tests: {
        title: string
        input: string
        expected: string | null
    }[] = [
        {
            title: 'returns PR number if commit message contains one',
            input: 'this commit fixes stuff (#34353)',
            expected: '#34353',
        },
        {
            title: 'returns null if no PR in commit message',
            input: 'this commit fixes stuff',
            expected: null,
        },
    ]

    for (const tc of tests) {
        test(tc.title, () => {
            let got = extractPRNumber(tc.input)
            expect(got).toBe(tc.expected)
        })
    }
})

describe('truncateIfNeeded', () => {
    const tests: {
        title: string
        input: string
        expected: string
    }[] = [
        {
            title: 'does not truncate if msg <= 23 chars',
            input: 'this commit fixes bugs',
            expected: 'this commit fixes bugs',
        },
        {
            title: 'truncates when necessary',
            input: 'this commit truncates stuff',
            expected: 'this commit truncates s...',
        },
    ]

    for (const tc of tests) {
        test(tc.title, () => {
            let got = truncateIfNeeded(tc.input)
            expect(got).toBe(tc.expected)
        })
    }
})

describe('extractCommitMessage', () => {
    const tests: {
        title: string
        input: string
        expected: string | null
    }[] = [
        {
            title: 'extract commit message when necessary',
            input: 'this commit fixes bugs (#54333)',
            expected: 'this commit fixes bugs',
        },
        {
            title: 'leave message as is when necessary',
            input: 'this commit fixes bugs',
            expected: 'this commit fixes bugs',
        },
    ]

    for (const tc of tests) {
        test(tc.title, () => {
            let got = extractCommitMessage(tc.input)
            expect(got).toBe(tc.expected)
        })
    }
})
