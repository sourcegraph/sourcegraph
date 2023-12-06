import { type FC, type PropsWithChildren, useMemo } from 'react'

import { ApolloClient, ApolloProvider } from '@apollo/client'

import { generateCache } from '../../backend/apolloCache'

/**
 * A provider that sets up an Apollo client with a mocked backend. The mock backed is provided by MSW
 * and needs to be set up in the test file via {@link setupMockServer} from
 * `@sourcegraph/shared/src/testing/graphql/vitest`.
 */
export const AutomockGraphQLProvider: FC<PropsWithChildren<{}>> = ({ children }) => {
    const client = useMemo(
        () =>
            new ApolloClient({
                uri: 'http://0.0.0.0:8080/graphql',
                cache: generateCache(),
            }),
        []
    )
    return <ApolloProvider client={client}>{children}</ApolloProvider>
}
