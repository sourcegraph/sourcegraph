import React, { useCallback, useMemo } from 'react'

import { mdiLink } from '@mdi/js'
import classNames from 'classnames'

import { FilterKind, findFilter } from '@sourcegraph/shared/src/search/query/query'
import { Button, Icon, Tooltip } from '@sourcegraph/wildcard'

import type { SearchPatternType } from '../../../graphql-operations'
import type { WebviewPageProps } from '../../platform/context'

import { ButtonDropdownCta, type ButtonDropdownCtaProps } from './ButtonDropdownCta'
import { BookmarkRadialGradientIcon, CodeMonitoringLogo } from './icons'

import styles from './SearchResultsInfoBar.module.scss'

// Debt: this is a fork of the web <SearchResultsInfobar>.
export interface SearchResultsInfoBarProps
    extends Pick<WebviewPageProps, 'extensionCoreAPI' | 'platformContext' | 'authenticatedUser' | 'instanceURL'> {
    stats: JSX.Element

    onShareResultsClick: () => void
    fullQuery: string
    patternType: SearchPatternType

    // Expand all feature
    allExpanded: boolean
    onExpandAllResultsToggle: () => void
}

interface ExperimentalActionButtonProps extends ButtonDropdownCtaProps {
    showExperimentalVersion: boolean
    nonExperimentalLinkTo?: string
    disabled?: boolean
    onNonExperimentalLinkClick?: () => void
    className?: string
}

const ExperimentalActionButton: React.FunctionComponent<
    React.PropsWithChildren<ExperimentalActionButtonProps>
> = props => {
    if (props.showExperimentalVersion) {
        return <ButtonDropdownCta {...props} />
    }
    return (
        <Button
            variant="secondary"
            outline={true}
            size="sm"
            onClick={props.onNonExperimentalLinkClick}
            disabled={props.disabled}
        >
            {props.button}
        </Button>
    )
}

export const SearchResultsInfoBar: React.FunctionComponent<
    React.PropsWithChildren<SearchResultsInfoBarProps>
> = props => {
    const {
        extensionCoreAPI,
        platformContext,
        authenticatedUser,
        onShareResultsClick,
        stats,
        instanceURL,
        fullQuery,
        patternType,
    } = props

    const showActionButtonExperimentalVersion = !authenticatedUser

    const onCreateCodeMonitorButtonClick = useCallback(
        (event?: React.FormEvent): void => {
            event?.preventDefault()
            platformContext.telemetryService.log('VSCECreateCodeMonitorClick')

            const searchParameters = new URLSearchParams()
            searchParameters.set('q', fullQuery)
            searchParameters.set('trigger-query', `${fullQuery} patternType:${patternType}`)
            const createMonitorURL = new URL(`/code-monitoring/new?${searchParameters.toString()}`, instanceURL)
            extensionCoreAPI.openLink(createMonitorURL.href).catch(() => {
                console.error('Error opening create code monitor link')
            })
        },
        [platformContext.telemetryService, extensionCoreAPI, fullQuery, instanceURL, patternType]
    )

    const canCreateMonitorFromQuery = useMemo(() => {
        if (!fullQuery) {
            return false
        }
        const globalTypeFilterInQuery = findFilter(fullQuery, 'type', FilterKind.Global)
        const globalTypeFilterValue = globalTypeFilterInQuery?.value ? globalTypeFilterInQuery.value.value : undefined
        return globalTypeFilterValue === 'diff' || globalTypeFilterValue === 'commit'
    }, [fullQuery])

    const createCodeMonitorButton = useMemo(() => {
        const searchParameters = new URLSearchParams()
        searchParameters.set('q', fullQuery)
        searchParameters.set('trigger-query', `${fullQuery} patternType:${patternType}`)
        return (
            <li className={classNames('mr-2', styles.navItem)}>
                <Tooltip
                    content={
                        !canCreateMonitorFromQuery
                            ? 'Code monitors only support type:diff or type:commit searches.'
                            : undefined
                    }
                >
                    <ExperimentalActionButton
                        extensionCoreAPI={extensionCoreAPI}
                        showExperimentalVersion={showActionButtonExperimentalVersion}
                        onNonExperimentalLinkClick={onCreateCodeMonitorButtonClick}
                        className="test-save-search-link"
                        button={
                            <>
                                <Icon aria-hidden={true} className="mr-1" as={CodeMonitoringLogo} />
                                Monitor
                            </>
                        }
                        icon={<BookmarkRadialGradientIcon />}
                        title="Monitor code for changes"
                        copyText="Create a monitor and get notified when your code changes. Free for registered users."
                        source="CodeMonitor"
                        viewEventName="VSCECodeMonitorCTAShown"
                        returnTo={`/code-monitoring/new?${searchParameters.toString()}`}
                        telemetryService={platformContext.telemetryService}
                        disabled={!canCreateMonitorFromQuery}
                        instanceURL={instanceURL}
                    />
                </Tooltip>
            </li>
        )
    }, [
        fullQuery,
        patternType,
        extensionCoreAPI,
        showActionButtonExperimentalVersion,
        onCreateCodeMonitorButtonClick,
        canCreateMonitorFromQuery,
        platformContext.telemetryService,
        instanceURL,
    ])

    const ShareLinkButton = useMemo(
        () => (
            <Tooltip content="Share results link">
                <li className={classNames('mr-2', styles.navItem)}>
                    <Button variant="secondary" outline={true} size="sm" onClick={onShareResultsClick}>
                        <Icon aria-hidden={true} className="mr-1" svgPath={mdiLink} />
                        Share
                    </Button>
                </li>
            </Tooltip>
        ),
        [onShareResultsClick]
    )

    return (
        <div className={classNames('flex-grow-1 my-2', styles.searchResultsInfoBar)} data-testid="results-info-bar">
            <div className={styles.row}>
                {stats}
                <div className={styles.expander} />
                <ul className="nav align-items-center">
                    {createCodeMonitorButton}
                    {ShareLinkButton}
                </ul>
            </div>
        </div>
    )
}
