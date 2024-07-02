import { type FC, useEffect, useState } from 'react'

import classNames from 'classnames'

import { Toggle } from '@sourcegraph/branded/src/components/Toggle'
import { QueryExamples } from '@sourcegraph/branded/src/search-ui/components/QueryExamples'
import type { QueryState } from '@sourcegraph/shared/src/search'
import { getGlobalSearchContextFilter } from '@sourcegraph/shared/src/search/query/query'
import { appendContextFilter, omitFilter } from '@sourcegraph/shared/src/search/query/transformer'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Label, Tooltip, useLocalStorage } from '@sourcegraph/wildcard'

import { BrandLogo } from '../../../components/branding/BrandLogo'
import { useFeatureFlag } from '../../../featureFlags/useFeatureFlag'
import { SearchPatternType } from '../../../graphql-operations'
import { useLegacyContext_onlyInStormRoutes } from '../../../LegacyRouteContext'
import { useV2QueryInput } from '../../../search/useV2QueryInput'
import { useNavbarQueryState } from '../../../stores'
import { GettingStartedTour } from '../../../tour/GettingStartedTour'
import { useShowOnboardingTour } from '../../../tour/hooks'

import { AddCodeHostWidget } from './AddCodeHostWidget'
import { KeywordSearchCtaSection } from './KeywordSearchCtaSection'
import { SearchPageFooter } from './SearchPageFooter'
import { SearchPageInput } from './SearchPageInput'

import styles from './SearchPageContent.module.scss'

interface SearchPageContentProps {
    shouldShowAddCodeHostWidget?: boolean
    isSourcegraphDotCom: boolean
}

export const SearchPageContent: FC<SearchPageContentProps> = props => {
    const { shouldShowAddCodeHostWidget } = props

    const { telemetryService, selectedSearchContextSpec, isSourcegraphDotCom, authenticatedUser, platformContext } =
        useLegacyContext_onlyInStormRoutes()
    const { telemetryRecorder } = platformContext

    const isLightTheme = useIsLightTheme()
    const [v2QueryInput] = useV2QueryInput()

    /** The value entered by the user in the query input */
    const [queryState, setQueryState] = useState<QueryState>({
        query: '',
    })

    useEffect(() => {
        telemetryService.logViewEvent('Home')
        telemetryRecorder.recordEvent('home', 'view')
    }, [telemetryService, telemetryRecorder])
    useEffect(() => {
        // TODO (#48103): Remove/simplify when new search input is released
        // Because the current and the new search input handle the context: selector differently
        // we need properly "translate" the queries when switching between the both versions
        if (selectedSearchContextSpec) {
            setQueryState(state => {
                if (v2QueryInput) {
                    return { query: appendContextFilter(state.query, selectedSearchContextSpec) }
                }
                const contextFilter = getGlobalSearchContextFilter(state.query)?.filter
                if (contextFilter) {
                    return { query: omitFilter(state.query, contextFilter) }
                }
                return state
            })
        }
    }, [v2QueryInput, selectedSearchContextSpec])

    const defaultSimpleSearchToggle = true
    const [simpleSearch, setSimpleSearch] = useLocalStorage('simple.search.toggle', defaultSimpleSearchToggle)
    const [simpleSearchEnabled] = useFeatureFlag('enable-simple-search', false)

    const showOnboardingTour = useShowOnboardingTour({ authenticatedUser, isSourcegraphDotCom })
    const patternType = useNavbarQueryState.getState().searchPatternType

    return (
        <div className={classNames('d-flex flex-column align-items-center px-3', styles.searchPage)}>
            <BrandLogo className={styles.logo} isLightTheme={isLightTheme} variant="logo" />
            {isSourcegraphDotCom && (
                <div className="text-muted mt-3 mr-sm-2 pr-2 text-center">
                    Code search and an AI assistant with the context of the code graph.
                </div>
            )}

            <div className={styles.searchContainer}>
                {simpleSearchEnabled && (
                    <div className="mb-2">
                        <Label htmlFor="simpleSearchToggle" className="mr-2">
                            Simple search
                        </Label>
                        <Toggle
                            id="simpleSearchToggle"
                            value={simpleSearch}
                            onToggle={val => {
                                const arg = { state: val }
                                telemetryService.log('SimpleSearchToggle', arg, arg)
                                telemetryRecorder.recordEvent('home.simpleSearch', 'toggle', {
                                    metadata: { enabled: val ? 1 : 0 },
                                })
                                setSimpleSearch(val)
                            }}
                        />
                    </div>
                )}

                {shouldShowAddCodeHostWidget ? (
                    <>
                        <Tooltip
                            content="Sourcegraph is not fully functional until a code host is set up"
                            placement="top"
                        >
                            <div className={styles.translucent}>
                                <SearchPageInput
                                    simpleSearch={false}
                                    queryState={queryState}
                                    setQueryState={setQueryState}
                                />
                            </div>
                        </Tooltip>
                        <AddCodeHostWidget className="mb-4" />
                    </>
                ) : (
                    <>
                        <SearchPageInput
                            simpleSearch={simpleSearch && simpleSearchEnabled}
                            queryState={queryState}
                            setQueryState={setQueryState}
                        />
                        {authenticatedUser && showOnboardingTour && (
                            <GettingStartedTour
                                className="mt-5"
                                telemetryService={telemetryService}
                                telemetryRecorder={telemetryRecorder}
                                variant="horizontal"
                                authenticatedUser={authenticatedUser}
                            />
                        )}
                        {patternType === SearchPatternType.keyword ? <KeywordSearchCtaSection /> : <></>}
                    </>
                )}
            </div>
            {(!simpleSearchEnabled || !simpleSearch) && (
                <div className={classNames(styles.panelsContainer)}>
                    {(!!authenticatedUser || isSourcegraphDotCom) && (
                        <QueryExamples
                            selectedSearchContextSpec={selectedSearchContextSpec}
                            telemetryService={telemetryService}
                            telemetryRecorder={telemetryRecorder}
                            isSourcegraphDotCom={isSourcegraphDotCom}
                            patternType={patternType}
                        />
                    )}
                </div>
            )}
            <SearchPageFooter />
        </div>
    )
}

interface ShouldShowAddCodeHostWidgetOptions {
    isAddCodeHostWidgetEnabled?: boolean
    isSiteAdmin?: boolean
    externalServicesCount?: number
}

export function getShouldShowAddCodeHostWidget({
    isAddCodeHostWidgetEnabled,
    isSiteAdmin,
    externalServicesCount,
}: ShouldShowAddCodeHostWidgetOptions): boolean {
    return !!isAddCodeHostWidgetEnabled && !!isSiteAdmin && externalServicesCount === 0
}
