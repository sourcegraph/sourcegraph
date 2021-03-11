import React, { useState, useCallback } from 'react'
import * as H from 'history'
import { ExtensionsControllerProps } from '../../../../../shared/src/extensions/controller'
import { ThemeProps } from '../../../../../shared/src/theme'
import { PlatformContextProps } from '../../../../../shared/src/platform/context'
import { TelemetryProps } from '../../../../../shared/src/telemetry/telemetryService'
import { BatchChangeFields } from '../../../graphql-operations'
import {
    queryChangesets as _queryChangesets,
    queryExternalChangesetWithFileDiffs as _queryExternalChangesetWithFileDiffs,
    queryChangesetCountsOverTime as _queryChangesetCountsOverTime,
} from './backend'
import classNames from 'classnames'
import SourceBranchIcon from 'mdi-react/SourceBranchIcon'
import ChartLineVariantIcon from 'mdi-react/ChartLineVariantIcon'
import { BatchChangeBurndownChart } from './BatchChangeBurndownChart'
import { BatchChangeChangesets } from './changesets/BatchChangeChangesets'
import FileDocumentIcon from 'mdi-react/FileDocumentIcon'
import { BatchSpecTab } from './BatchSpecTab'

type SelectedTab = 'changesets' | 'chart' | 'spec'

export interface BatchChangeTabsProps
    extends ExtensionsControllerProps,
        ThemeProps,
        PlatformContextProps,
        TelemetryProps {
    batchChange: BatchChangeFields
    history: H.History
    location: H.Location
    /** For testing only. */
    queryChangesets?: typeof _queryChangesets
    /** For testing only. */
    queryExternalChangesetWithFileDiffs?: typeof _queryExternalChangesetWithFileDiffs
    /** For testing only. */
    queryChangesetCountsOverTime?: typeof _queryChangesetCountsOverTime
}

export const BatchChangeTabs: React.FunctionComponent<BatchChangeTabsProps> = ({
    extensionsController,
    history,
    isLightTheme,
    location,
    platformContext,
    telemetryService,
    batchChange,
    queryChangesets,
    queryChangesetCountsOverTime,
    queryExternalChangesetWithFileDiffs,
}) => {
    const [selectedTab, setSelectedTab] = useState<SelectedTab>(() => {
        const urlParameters = new URLSearchParams(location.search)
        if (urlParameters.get('tab') === 'chart') {
            return 'chart'
        }
        if (urlParameters.get('tab') === 'spec') {
            return 'spec'
        }
        return 'changesets'
    })
    const onSelectChangesets = useCallback<React.MouseEventHandler>(
        event => {
            event.preventDefault()
            setSelectedTab('changesets')
            const urlParameters = new URLSearchParams(location.search)
            urlParameters.delete('tab')
            if (location.search !== urlParameters.toString()) {
                history.replace({ ...location, search: urlParameters.toString() })
            }
        },
        [history, location]
    )
    const onSelectChart = useCallback<React.MouseEventHandler>(
        event => {
            event.preventDefault()
            setSelectedTab('chart')
            const urlParameters = new URLSearchParams(location.search)
            urlParameters.set('tab', 'chart')
            if (location.search !== urlParameters.toString()) {
                history.replace({ ...location, search: urlParameters.toString() })
            }
        },
        [history, location]
    )
    const onSelectSpec = useCallback<React.MouseEventHandler>(
        event => {
            event.preventDefault()
            setSelectedTab('spec')
            const urlParameters = new URLSearchParams(location.search)
            urlParameters.set('tab', 'spec')
            if (location.search !== urlParameters.toString()) {
                history.replace({ ...location, search: urlParameters.toString() })
            }
        },
        [history, location]
    )
    return (
        <>
            <div className="overflow-auto mb-2">
                <ul className="nav nav-tabs d-inline-flex d-sm-flex flex-nowrap text-nowrap">
                    <li className="nav-item">
                        <a
                            href=""
                            onClick={onSelectChangesets}
                            className={classNames('nav-link', selectedTab === 'changesets' && 'active')}
                        >
                            <SourceBranchIcon className="icon-inline text-muted mr-1" /> Changesets
                        </a>
                    </li>
                    <li className="nav-item test-batches-chart-tab">
                        <a
                            href=""
                            onClick={onSelectChart}
                            className={classNames('nav-link', selectedTab === 'chart' && 'active')}
                        >
                            <ChartLineVariantIcon className="icon-inline text-muted mr-1" /> Burndown chart
                        </a>
                    </li>
                    <li className="nav-item test-batches-spec-tab">
                        <a
                            href=""
                            onClick={onSelectSpec}
                            className={classNames('nav-link', selectedTab === 'spec' && 'active')}
                        >
                            <FileDocumentIcon className="icon-inline text-muted mr-1" /> Spec
                        </a>
                    </li>
                </ul>
            </div>
            {selectedTab === 'chart' && (
                <BatchChangeBurndownChart
                    batchChangeID={batchChange.id}
                    queryChangesetCountsOverTime={queryChangesetCountsOverTime}
                    history={history}
                />
            )}
            {selectedTab === 'changesets' && (
                <BatchChangeChangesets
                    batchChangeID={batchChange.id}
                    viewerCanAdminister={batchChange.viewerCanAdminister}
                    history={history}
                    location={location}
                    isLightTheme={isLightTheme}
                    extensionsController={extensionsController}
                    platformContext={platformContext}
                    telemetryService={telemetryService}
                    queryChangesets={queryChangesets}
                    queryExternalChangesetWithFileDiffs={queryExternalChangesetWithFileDiffs}
                />
            )}
            {selectedTab === 'spec' && (
                <BatchSpecTab batchChange={batchChange} originalInput={batchChange.currentSpec.originalInput} />
            )}
        </>
    )
}
