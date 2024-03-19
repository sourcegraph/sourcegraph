import React, { useCallback } from 'react'

import { mdiCodeBrackets, mdiFormatLetterCase, mdiRegex } from '@mdi/js'
import classNames from 'classnames'

import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import type {
    CaseSensitivityProps,
    SearchPatternTypeMutationProps,
    SubmitSearchProps,
    SearchModeProps,
    SearchPatternTypeProps,
} from '@sourcegraph/shared/src/search'
import { findFilter, FilterKind } from '@sourcegraph/shared/src/search/query/query'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'

import { QueryInputToggle } from './QueryInputToggle'

import styles from './Toggles.module.scss'

export interface TogglesProps
    extends SearchPatternTypeProps,
        SearchPatternTypeMutationProps,
        CaseSensitivityProps,
        SearchModeProps,
        TelemetryProps,
        Partial<Pick<SubmitSearchProps, 'submitSearch'>> {
    navbarSearchQuery: string
    className?: string
    /**
     * If set to false makes all buttons non-actionable. The main use case for
     * this prop is showing the toggles in examples. This is different from
     * being disabled, because the buttons still render normally.
     */
    interactive?: boolean
    /** Comes from JSContext only set in the web app. */
    structuralSearchDisabled?: boolean
}

/**
 * The toggles displayed in the query input.
 */
export const Toggles: React.FunctionComponent<React.PropsWithChildren<TogglesProps>> = (props: TogglesProps) => {
    const {
        navbarSearchQuery,
        patternType,
        setPatternType,
        caseSensitive,
        setCaseSensitivity,
        className,
        submitSearch,
        structuralSearchDisabled,
        telemetryService,
    } = props

    const submitOnToggle = useCallback(
        (args: { newPatternType: SearchPatternType } | { newCaseSensitivity: boolean }): void => {
            submitSearch?.({
                source: 'filter',
                patternType: 'newPatternType' in args ? args.newPatternType : patternType,
                caseSensitive: 'newCaseSensitivity' in args ? args.newCaseSensitivity : caseSensitive,
            })
        },
        [caseSensitive, patternType, submitSearch]
    )

    const toggleCaseSensitivity = useCallback((): void => {
        const newCaseSensitivity = !caseSensitive
        setCaseSensitivity(newCaseSensitivity)
        submitOnToggle({ newCaseSensitivity })
    }, [caseSensitive, setCaseSensitivity, submitOnToggle])

    const toggleRegexp = useCallback((): void => {
        const newPatternType =
            patternType !== SearchPatternType.regexp ? SearchPatternType.regexp : SearchPatternType.keyword

        setPatternType(newPatternType)
        submitOnToggle({ newPatternType })
        telemetryService.log('ToggleRegexpPatternType', { currentStatus: patternType === SearchPatternType.regexp })
    }, [patternType, setPatternType, submitOnToggle, telemetryService])

    const toggleStructuralSearch = useCallback((): void => {
        const newPatternType: SearchPatternType =
            patternType !== SearchPatternType.structural ? SearchPatternType.structural : SearchPatternType.keyword

        setPatternType(newPatternType)
        submitOnToggle({ newPatternType })
    }, [patternType, setPatternType, submitOnToggle])

    return (
        <div className={classNames(className, styles.toggleContainer)}>
            <>
                <QueryInputToggle
                    title="Case sensitivity"
                    isActive={caseSensitive}
                    onToggle={toggleCaseSensitivity}
                    iconSvgPath={mdiFormatLetterCase}
                    interactive={props.interactive}
                    className={`test-case-sensitivity-toggle ${styles.caseSensitivityToggle}`}
                    disableOn={[
                        {
                            condition: findFilter(navbarSearchQuery, 'case', FilterKind.Subexpression) !== undefined,
                            reason: 'Query already contains one or more case subexpressions',
                        },
                        {
                            condition:
                                findFilter(navbarSearchQuery, 'patterntype', FilterKind.Subexpression) !== undefined,
                            reason: 'Query contains one or more patterntype subexpressions, cannot apply global case-sensitivity',
                        },
                        {
                            condition: patternType === SearchPatternType.structural,
                            reason: 'Structural search is always case sensitive',
                        },
                    ]}
                />
                <QueryInputToggle
                    title="Regular expression"
                    isActive={patternType === SearchPatternType.regexp}
                    onToggle={toggleRegexp}
                    iconSvgPath={mdiRegex}
                    interactive={props.interactive}
                    className={`test-regexp-toggle ${styles.regularExpressionToggle}`}
                    disableOn={[
                        {
                            condition:
                                findFilter(navbarSearchQuery, 'patterntype', FilterKind.Subexpression) !== undefined,
                            reason: 'Query already contains one or more patterntype subexpressions',
                        },
                    ]}
                />
                <>
                    {!structuralSearchDisabled && (
                        <QueryInputToggle
                            title="Structural search"
                            className={`test-structural-search-toggle ${styles.structuralSearchToggle}`}
                            isActive={patternType === SearchPatternType.structural}
                            onToggle={toggleStructuralSearch}
                            iconSvgPath={mdiCodeBrackets}
                            interactive={props.interactive}
                            disableOn={[
                                {
                                    condition:
                                        findFilter(navbarSearchQuery, 'patterntype', FilterKind.Subexpression) !==
                                        undefined,
                                    reason: 'Query already contains one or more patterntype subexpressions',
                                },
                            ]}
                        />
                    )}
                </>
            </>
        </div>
    )
}
