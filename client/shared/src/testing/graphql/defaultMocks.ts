import { fakerEN as faker } from '@faker-js/faker'
import { capitalize } from 'lodash'

import type { TypeMocks } from '../../graphql-types'

export const defaultMocks: TypeMocks = {
    // GraphQLs Int type is 32 bit so we need to set the max value.
    Int: () => faker.number.int(2 ** 31),
    String: () => faker.string.alpha(10),
    ID: () => faker.string.uuid(),
    Boolean: () => faker.helpers.arrayElement([true, false]),

    // Our own scalar types
    JSONCString: () => '{}',
    DateTime: () => faker.date.past().toISOString(),
    GitObjectID: () => faker.git.commitSha(),
    BigInt: () => String(faker.number.int(2 ** 53)),

    // Ensure some simple business logic

    // In Ranges the start line is always smaller than the end line.
    Range: () => {
        const line = faker.number.int(1000)
        return {
            start: {
                line,
                character: faker.number.int(100),
            },
            end: {
                line: line + faker.number.int(50),
                character: faker.number.int(100),
            },
        }
    },

    Person: () => {
        const name = faker.internet.userName()
        return {
            name,
            displayName: faker.helpers.maybe(() => faker.internet.displayName()) ?? '',
            email: faker.helpers.maybe(() => faker.internet.email()) ?? '',
            avatarURL: faker.helpers.maybe(() => faker.image.avatar()) ?? null,
            user: {
                username: name,
            },
        }
    },

    Signature: () => ({
        // 'date' is typed as a string field but it will always be a date value.
        date: faker.date.past().toISOString(),
    }),

    User: () => ({
        username: faker.internet.userName(),
    }),

    Team: () => ({
        name: `${faker.company.buzzNoun()}`,
        displayName: `Team ${capitalize(faker.company.buzzNoun())}`,
        url: faker.internet.url(),
        avatarURL: faker.helpers.maybe(() => faker.image.avatar()) ?? null,
    }),

    UserEmail: () => ({
        email: faker.internet.email(),
    }),

    SettingsCascade: () => ({
        final: '{}', // JSON value
    }),
}
