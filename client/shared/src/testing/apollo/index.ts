import { act } from '@testing-library/react'

/**
 * Wait one tick to load the next response from Apollo
 * https://www.apollographql.com/docs/react/development-testing/testing/#testing-the-success-state
 */
export const waitForNextApolloResponse = (): Promise<void> =>
    act(() => new Promise(resolve => setTimeout(resolve, 100)))

export * from './mockedTestProvider'
export * from './mockedMswProvider'
