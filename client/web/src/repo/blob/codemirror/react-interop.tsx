import React, { useEffect, useState } from 'react'

import { type ApolloClient, ApolloProvider } from '@apollo/client'
import { BrowserRouter, type NavigateFunction, useLocation } from 'react-router-dom'

import { WildcardThemeContext } from '@sourcegraph/wildcard'

interface CodeMirrorContainerProps {
    graphQLClient: ApolloClient<any>
    navigate: NavigateFunction
    onMount?: () => void
    onRender?: () => void
}

/**
 * Creates the necessary context for React components to be rendered inside
 * CodeMirror.
 */
export const CodeMirrorContainer: React.FunctionComponent<React.PropsWithChildren<CodeMirrorContainerProps>> = ({
    graphQLClient,
    navigate,
    onMount,
    onRender,
    children,
}) => {
    useEffect(() => onRender?.())
    // This should only be called once when the component is mounted
    // eslint-disable-next-line react-hooks/exhaustive-deps
    useEffect(() => onMount?.(), [])

    return (
        <ApolloProvider client={graphQLClient}>
            <WildcardThemeContext.Provider value={{ isBranded: true }}>
                <BrowserRouter>
                    {children}
                    <SyncInnerRouterWithParent navigate={navigate} />
                </BrowserRouter>
            </WildcardThemeContext.Provider>
        </ApolloProvider>
    )
}

const SyncInnerRouterWithParent: React.FC<{ navigate: NavigateFunction }> = ({ navigate }) => {
    const initialLocation = useState(useLocation())[0]
    const location = useLocation()
    useEffect(() => {
        if (
            location.hash === initialLocation.hash &&
            location.pathname === initialLocation.pathname &&
            location.search === initialLocation.search
        ) {
            return
        }
        navigate(location, { replace: true })
    }, [location, navigate, initialLocation])
    return null
}
