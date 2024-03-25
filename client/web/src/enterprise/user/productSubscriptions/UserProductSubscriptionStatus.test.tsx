import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import { UserProductSubscriptionStatus } from './UserProductSubscriptionStatus'

vi.mock('mdi-react/KeyIcon', () => () => 'KeyIcon')
vi.mock('mdi-react/InformationIcon', () => () => 'InformationIcon')
vi.mock('../../../components/CopyableText', () => ({ CopyableText: () => 'CopyableText' }))

describe('UserProductSubscriptionStatus', () => {
    test('toggle', () => {
        const { asFragment } = renderWithBrandedContext(
            <UserProductSubscriptionStatus
                subscriptionName="L-123"
                productNameWithBrand="P"
                userCount={123}
                expiresAt={23456}
                licenseKey="lk"
                telemetryRecorder={noOpTelemetryRecorder}
            />
        )
        expect(asFragment()).toMatchSnapshot('license key hidden')

        // Click "Reveal license key" button.
        const button = screen.getByRole('button')
        userEvent.click(button)
        expect(asFragment()).toMatchSnapshot('license key revealed')
    })
})
