import { subDays } from 'date-fns'
import { ListBatchChange } from '../../../graphql-operations'

export const now = new Date()

export const nodes: Record<string, ListBatchChange> = {
    'Open batch change': {
        id: 'test',
        url: '/users/alice/batch-change/test',
        name: 'Awesome batch',
        description: `# What this does

This is my thorough explanation. And it can also get very long, in that case the UI doesn't break though, which is good. And one more line to finally be longer than the viewport.`,
        createdAt: subDays(now, 5).toISOString(),
        closedAt: null,
        changesetsStats: {
            open: 10,
            closed: 0,
            merged: 5,
        },
        namespace: {
            namespaceName: 'alice',
            url: '/users/alice',
        },
    },
    'No description': {
        id: 'test2',
        url: '/users/alice/batch-changes/test2',
        name: 'Awesome batch',
        description: null,
        createdAt: subDays(now, 5).toISOString(),
        closedAt: null,
        changesetsStats: {
            open: 10,
            closed: 0,
            merged: 5,
        },
        namespace: {
            namespaceName: 'alice',
            url: '/users/alice',
        },
    },
    'Closed batch change': {
        id: 'test3',
        url: '/users/alice/batch-changes/test3',
        name: 'Awesome batch',
        description: `# My batch

        This is my thorough explanation.`,
        createdAt: subDays(now, 5).toISOString(),
        closedAt: subDays(now, 3).toISOString(),
        changesetsStats: {
            open: 0,
            closed: 10,
            merged: 5,
        },
        namespace: {
            namespaceName: 'alice',
            url: '/users/alice',
        },
    },
}
