import { faker } from '@faker-js/faker'
import { range } from 'lodash'

import type { Commit } from '$lib/Commit.gql'
import { type HighlightedFileVariables, type HighlightedFileResult } from '$lib/graphql-operations'
import type { HistoryPanel_HistoryConnection } from '$lib/repo/HistoryPanel.gql'

/**
 * Initializes faker's randomness generator with a fixed seed, for
 * deterministic results.
 */
export function seed(seed?: number): number {
    return faker.seed(seed)
}

export function createSignature() {
    const firstName = faker.person.firstName()
    const lastName = faker.person.lastName()
    const displayName = faker.internet.displayName({ firstName, lastName })

    return {
        person: {
            displayName,
            name: faker.person.fullName({ firstName, lastName }),
            avatarURL: faker.internet.avatar(),
            user: {
                displayName,
                id: faker.string.uuid(),
                url: faker.internet.url(),
                username: faker.internet.userName({ firstName, lastName }),
            },
            email: faker.internet.email(),
        },
        date: faker.date.past().toISOString(),
    }
}

export function createGitCommit(initial?: Partial<Commit>): Commit {
    const oid = faker.git.commitSha()

    return {
        id: faker.string.uuid(),
        abbreviatedOID: oid.slice(0, 7),
        subject: faker.git.commitMessage(),
        body: faker.lorem.paragraph(),
        author: createSignature(),
        committer: faker.helpers.maybe(createSignature) ?? null,
        canonicalURL: faker.internet.url(),
        ...initial,
    }
}

export function createHistoryResults(count: number, pageSize: number): HistoryPanel_HistoryConnection[] {
    return Array.from({ length: count }, (_, index) => ({
        nodes: faker.helpers.uniqueArray(createGitCommit, pageSize),
        pageInfo: {
            hasNextPage: index < count - 1,
            endCursor: String((index + 1) * pageSize),
        },
    }))
}

const MAX_LINE_LENGTH = 100

function colorize(line: string): string {
    return line
        .split(' ')
        .map(word => faker.helpers.maybe(() => `<span style="color: ${faker.color.rgb()}">${word}</span>`) ?? word)
        .join(' ')
}

export function createHighlightedFileResult(ranges: HighlightedFileVariables['ranges']): HighlightedFileResult {
    return {
        repository: {
            id: faker.string.uuid(),
            commit: {
                id: faker.string.uuid(),
                blob: {
                    canonicalURL: faker.internet.url(),
                    isDirectory: false,
                    highlight: {
                        aborted: false,
                        lineRanges: ranges.map(({ startLine, endLine }) =>
                            range(startLine, endLine).map(
                                line =>
                                    `<tr><td class="line" data-line="${line}"></td><td class="code annotated-selection-match">${colorize(
                                        loremLine(MAX_LINE_LENGTH)
                                    )}</td></tr>`
                            )
                        ),
                    },
                },
            },
        },
    }
}

function loremLine(minLength: number): string {
    let content = ''
    do {
        content += faker.lorem.sentence() + ' '
    } while (content.length < minLength)

    return content
}
