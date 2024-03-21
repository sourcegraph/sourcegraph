import { render } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import type { AuthProvider } from '../../../jscontext'

import { ExternalAccountsSignIn } from './ExternalAccountsSignIn'
import type { UserExternalAccount } from './UserSettingsSecurityPage'

const mockAccounts: UserExternalAccount[] = [
    {
        id: '1',
        serviceID: '123',
        serviceType: 'github',
        publicAccountData: {
            displayName: 'account1',
            login: 'account1',
            url: 'https://example.com/account1',
        },
        clientID: '123',
    },
    {
        id: '2',
        serviceID: '123',
        serviceType: 'github',
        publicAccountData: {
            displayName: 'account2',
            login: 'account2',
            url: 'https://example.com/account2',
        },
        clientID: '123',
    },
]

const mockAuthProviders: AuthProvider[] = [
    {
        serviceType: 'github',
        displayName: 'GitHub',
        serviceID: '123',
        clientID: '123',
        isBuiltin: false,
        authenticationURL: 'https://example.com',
        noSignIn: false,
    },
]

describe('ExternalAccountsSignIn', () => {
    test('renders multiple accounts correctly', () => {
        const cmp = render(
            <ExternalAccountsSignIn
                accounts={mockAccounts}
                authProviders={mockAuthProviders}
                onDidRemove={() => {}}
                onDidAdd={() => {}}
                onDidError={() => {}}
            />
        )
        expect(cmp.asFragment()).toMatchSnapshot()
    })
})
