import { Tab, TabList, TabPanel, TabPanels, Tabs } from '@reach/tabs'
import * as H from 'history'
import CloseIcon from 'mdi-react/CloseIcon'
import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useHistory, useLocation } from 'react-router'
import { Button } from 'reactstrap'
import { Observable } from 'rxjs'
import { map } from 'rxjs/operators'
import { ActionsNavItems } from '../../../../shared/src/actions/ActionsNavItems'
import { ContributableMenu, ContributableViewContainer } from '../../../../shared/src/api/protocol/contribution'
import { ActivationProps } from '../../../../shared/src/components/activation/Activation'
import { FetchFileParameters } from '../../../../shared/src/components/CodeExcerpt'
import { Resizable } from '../../../../shared/src/components/Resizable'
import { ExtensionsControllerProps } from '../../../../shared/src/extensions/controller'
import { PlatformContextProps } from '../../../../shared/src/platform/context'
import { VersionContextProps } from '../../../../shared/src/search/util'
import { SettingsCascadeProps } from '../../../../shared/src/settings/settings'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { ThemeProps } from '../../../../shared/src/theme'
import { useObservable } from '../../../../shared/src/util/useObservable'
import { EmptyPanelView } from './views/EmptyPanelView'
import { PanelView } from './views/PanelView'

interface Props
    extends ExtensionsControllerProps,
        PlatformContextProps,
        SettingsCascadeProps,
        ActivationProps,
        TelemetryProps,
        ThemeProps,
        VersionContextProps {
    location: H.Location
    history: H.History
    repoName?: string
    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>
}

/**
 * A tab and corresponding content to display in the panel.
 */
interface PanelItem {
    id: string

    label: React.ReactFragment
    /**
     * Controls the relative order of panel items. The items are laid out from highest priority (at the beginning)
     * to lowest priority (at the end). The default is 0.
     */
    priority: number

    /** The content element to display when the tab is active. */
    element: JSX.Element

    /**
     * Whether this panel contains a list of locations (from a location provider). This value is
     * exposed to contributions as `panel.activeView.hasLocations`. It is true if there is a
     * location provider (even if the result set is empty).
     */
    hasLocations?: boolean
}

/**
 * The panel, which is a tabbed component with contextual information. Components rendering the panel should
 * generally use ResizablePanel, not Panel.
 *
 * Other components can contribute panel items to the panel.
 */

const Panel: React.FunctionComponent<Props> = props => {
    const [panels, setPanels] = useState<PanelItem[]>([])
    const [tabIndex, setTabIndex] = useState(0)
    const { hash, pathname } = useLocation()
    const history = useHistory()
    const handlePanelClose = useCallback(() => history.replace(pathname), [history, pathname])

    const items = useObservable(
        useMemo(
            () =>
                props.extensionsController.services.panelViews
                    .getPanelViews(ContributableViewContainer.Panel)
                    .pipe(map(panelViews => ({ panelViews }))),
            [props.extensionsController.services.panelViews]
        )
    )

    const handleActiveTab = useCallback(
        (index: number): void => {
            history.replace(`${pathname}${hash.split('=')[0]}=${panels[index].id}`)
        },
        [hash, history, panels, pathname]
    )

    useEffect(() => {
        setTabIndex(panels.findIndex(({ id }) => id === `${hash.split('=')[1]}`))
    }, [hash, panels])

    useEffect(() => {
        if (items?.panelViews) {
            setPanels(
                items.panelViews
                    .map(
                        (panelView): PanelItem => ({
                            label: panelView.title,
                            id: panelView.id,
                            priority: panelView.priority,
                            element: <PanelView {...props} panelView={panelView} />,
                            hasLocations: !!panelView.locationProvider,
                        })
                    )
                    .sort((a, b) => b.priority - a.priority)
            )
        }
    }, [items?.panelViews, props])

    if (!items) {
        return <EmptyPanelView />
    }

    return (
        <Tabs
            className="d-flex flex-column w-100 overflow-hidden border-top"
            index={tabIndex}
            onChange={handleActiveTab}
        >
            <div className="tablist-wrapper bg-white d-flex justify-content-between">
                <TabList>
                    {panels.map(({ label, id }) => (
                        <Tab key={id}>{label}</Tab>
                    ))}
                </TabList>
                <div className="align-items-center d-flex mr-2">
                    <ActionsNavItems
                        {...props}
                        // TODO remove references to Bootstrap from shared, get class name from prop
                        // This is okay for now because the Panel is currently only used in the webapp
                        listClass="d-flex justify-content-end list-unstyled m-0 align-items-center"
                        listItemClass="pr-4"
                        // actionItemClass="d-flex flex-nowrap"
                        actionItemIconClass="icon-inline"
                        menu={ContributableMenu.PanelToolbar}
                        scope={
                            panels[tabIndex]
                                ? {
                                      type: 'panelView',
                                      id: panels[tabIndex].id,
                                      hasLocations: Boolean(panels[tabIndex].hasLocations),
                                  }
                                : undefined
                        }
                        wrapInList={true}
                    />
                    <Button
                        onClick={handlePanelClose}
                        className="bg-transparent border-0 ml-auto p-1 position-relative"
                        title="Close panel"
                    >
                        <CloseIcon className="icon-inline" />
                    </Button>
                </div>
            </div>
            <TabPanels className="bg-white d-flex flex-1 flex-column h-100 overflow-hidden">
                {panels.map(({ id, element }) => (
                    <TabPanel className="overflow-auto is-here-man" key={id}>
                        {element}
                    </TabPanel>
                ))}
            </TabPanels>
        </Tabs>
    )
}

/** A wrapper around Panel that makes it resizable. */
export const ResizablePanel: React.FunctionComponent<Props> = props => (
    <div className="w-100 bg-white">
        <Resizable position="top" defaultSize={350} storageKey="panel-size">
            <Panel {...props} />
        </Resizable>
    </div>
)
