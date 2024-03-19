import React, { useContext, useState, useCallback, useMemo } from 'react'

import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'

import { useCodyChat, type CodyChatStore, codyChatStoreMock } from '../useCodyChat'

import { useSidebarSize } from './useSidebarSize'

interface CodySidebarStore extends CodyChatStore {
    readonly isSidebarOpen: boolean
    readonly inputNeedsFocus: boolean
    setIsSidebarOpen: (isOpen: boolean) => void
    setFocusProvided: () => void
    setSidebarSize: (size: number) => void
}

const CodySidebarContext = React.createContext<CodySidebarStore | null>({
    ...codyChatStoreMock,
    isSidebarOpen: false,
    inputNeedsFocus: false,
    setSidebarSize: () => {},
    setIsSidebarOpen: () => {},
    setFocusProvided: () => {},
})

interface ICodySidebarStoreProviderProps extends TelemetryV2Props {
    children?: React.ReactNode
    authenticatedUser: AuthenticatedUser | null
}

export const CodySidebarStoreProvider: React.FC<ICodySidebarStoreProviderProps> = ({
    authenticatedUser,
    children,
    telemetryRecorder,
}) => {
    const [isSidebarOpen, setIsSidebarOpenState] = useTemporarySetting('cody.showSidebar', false)
    const [inputNeedsFocus, setInputNeedsFocus] = useState(false)
    const { setSidebarSize } = useSidebarSize()

    const setFocusProvided = useCallback(() => {
        setInputNeedsFocus(false)
    }, [setInputNeedsFocus])

    const setIsSidebarOpen = useCallback(
        (open: boolean) => {
            setIsSidebarOpenState(open)
            setInputNeedsFocus(true)
        },
        [setIsSidebarOpenState, setInputNeedsFocus]
    )

    const onEvent = useCallback(() => setIsSidebarOpen(true), [setIsSidebarOpen])

    const codyChatStore = useCodyChat({ userID: authenticatedUser?.id, onEvent, telemetryRecorder })

    const state = useMemo<CodySidebarStore>(
        () => ({
            ...codyChatStore,
            isSidebarOpen: isSidebarOpen ?? false,
            inputNeedsFocus,
            setIsSidebarOpen,
            setFocusProvided,
            setSidebarSize,
        }),
        [codyChatStore, isSidebarOpen, setIsSidebarOpen, setFocusProvided, setSidebarSize, inputNeedsFocus]
    )

    // dirty fix because CodyRecipesWidget is rendered inside a different React DOM tree.
    const global = window as any
    global.codySidebarStore = state

    return <CodySidebarContext.Provider value={state}>{children}</CodySidebarContext.Provider>
}

export const useCodySidebar = (): CodySidebarStore => useContext(CodySidebarContext) as CodySidebarStore

export const CODY_SIDEBAR_SIZES = { default: 350, max: 1200, min: 250 }

export { useSidebarSize }
