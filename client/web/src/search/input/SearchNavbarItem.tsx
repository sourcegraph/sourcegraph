import { Shortcut } from '@slimsag/react-shortcuts'
import * as H from 'history'
import React, { useCallback, useState, useEffect } from 'react'

import { Form } from '@sourcegraph/branded/src/components/Form'
import { SearchBox } from '@sourcegraph/branded/src/search/input/SearchBox'
import { ActivationProps } from '@sourcegraph/shared/src/components/activation/Activation'
import { KEYBOARD_SHORTCUT_FUZZY_FINDER } from '@sourcegraph/shared/src/keyboardShortcuts/keyboardShortcuts'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { CaseSensitivityProps, PatternTypeProps, SearchContextInputProps } from '@sourcegraph/shared/src/search'
import { SubmitSearchParameters } from '@sourcegraph/shared/src/search/helpers'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'
import { FuzzyFinder } from '@sourcegraph/web/src/components/fuzzyFinder/FuzzyFinder'

import { OnboardingTourProps, parseSearchURLQuery } from '..'
import { AuthenticatedUser } from '../../auth'
import { useGlobalStore } from '../../stores/global'
import { getExperimentalFeatures } from '../../util/get-experimental-features'

interface Props
    extends ActivationProps,
        PatternTypeProps,
        CaseSensitivityProps,
        SettingsCascadeProps,
        ThemeProps,
        SearchContextInputProps,
        OnboardingTourProps,
        TelemetryProps,
        PlatformContextProps<'requestGraphQL'> {
    authenticatedUser: AuthenticatedUser | null
    location: H.Location
    history: H.History
    isSourcegraphDotCom: boolean
    globbing: boolean
    isSearchAutoFocusRequired?: boolean
    isRepositoryRelatedPage?: boolean
}

/**
 * The search item in the navbar
 */
export const SearchNavbarItem: React.FunctionComponent<Props> = (props: Props) => {
    const autoFocus = props.isSearchAutoFocusRequired ?? true
    // This uses the same logic as in Layout.tsx until we have a better solution
    // or remove the search help button
    const isSearchPage = props.location.pathname === '/search' && Boolean(parseSearchURLQuery(props.location.search))
    const [isFuzzyFinderVisible, setIsFuzzyFinderVisible] = useState(false)
    const { queryState, setQueryState, submitSearch } = useGlobalStore()

    const submitSearchOnChange = useCallback(
        (parameters: Partial<SubmitSearchParameters> = {}) => {
            submitSearch({
                history: props.history,
                patternType: props.patternType,
                caseSensitive: props.caseSensitive,
                source: 'nav',
                activation: props.activation,
                selectedSearchContextSpec: props.selectedSearchContextSpec,
                ...parameters,
            })
        },
        [
            submitSearch,
            props.history,
            props.patternType,
            props.caseSensitive,
            props.activation,
            props.selectedSearchContextSpec,
        ]
    )

    const onSubmit = useCallback(
        (event?: React.FormEvent): void => {
            event?.preventDefault()
            submitSearchOnChange()
        },
        [submitSearchOnChange]
    )

    useEffect(() => {
        if (isSearchPage && isFuzzyFinderVisible) {
            setIsFuzzyFinderVisible(false)
        }
    }, [isSearchPage, isFuzzyFinderVisible])

    const { fuzzyFinder, fuzzyFinderCaseInsensitiveFileCountThreshold } = getExperimentalFeatures(
        props.settingsCascade.final
    )

    return (
        <Form
            className="search--navbar-item d-flex align-items-flex-start flex-grow-1 flex-shrink-past-contents"
            onSubmit={onSubmit}
        >
            <SearchBox
                {...props}
                queryState={queryState}
                onChange={setQueryState}
                onSubmit={onSubmit}
                submitSearchOnToggle={submitSearchOnChange}
                submitSearchOnSearchContextChange={submitSearchOnChange}
                autoFocus={autoFocus}
                hideHelpButton={isSearchPage}
                onHandleFuzzyFinder={setIsFuzzyFinderVisible}
                isExternalServicesUserModeAll={window.context.externalServicesUserMode === 'all'}
            />
            <Shortcut
                {...KEYBOARD_SHORTCUT_FUZZY_FINDER.keybindings[0]}
                onMatch={() => {
                    setIsFuzzyFinderVisible(true)
                    const input = document.querySelector<HTMLInputElement>('#fuzzy-modal-input')
                    input?.focus()
                    input?.select()
                }}
            />
            {props.isRepositoryRelatedPage && fuzzyFinder && (
                <FuzzyFinder
                    caseInsensitiveFileCountThreshold={fuzzyFinderCaseInsensitiveFileCountThreshold}
                    setIsVisible={bool => setIsFuzzyFinderVisible(bool)}
                    isVisible={isFuzzyFinderVisible}
                    telemetryService={props.telemetryService}
                />
            )}
        </Form>
    )
}
