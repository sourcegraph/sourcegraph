import { type FC, useCallback, useState } from 'react'

import { mdiChevronDoubleRight, mdiChevronDoubleLeft } from '@mdi/js'
import classNames from 'classnames'

import type { Scalars } from '@sourcegraph/shared/src/graphql-operations'
import { useKeyboardShortcut } from '@sourcegraph/shared/src/keyboardShortcuts/useKeyboardShortcut'
import { Shortcut } from '@sourcegraph/shared/src/react-shortcuts'
import { type SettingsCascadeProps, useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import type { RepoFile } from '@sourcegraph/shared/src/util/url'
import {
    Button,
    useLocalStorage,
    useMatchMedia,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Icon,
    Panel,
    Tooltip,
} from '@sourcegraph/wildcard'

import settingsSchemaJSON from '../../../../schema/settings.schema.json'
import type { AuthenticatedUser } from '../auth'
import { useFeatureFlag } from '../featureFlags/useFeatureFlag'
import { GettingStartedTour } from '../tour/GettingStartedTour'
import { useShowOnboardingTour } from '../tour/hooks'

import { RepoRevisionSidebarFileTree } from './RepoRevisionSidebarFileTree'
import { RepoRevisionSidebarSymbols } from './RepoRevisionSidebarSymbols'

import styles from './RepoRevisionSidebar.module.scss'

interface RepoRevisionSidebarProps extends RepoFile, TelemetryProps, TelemetryV2Props, SettingsCascadeProps {
    repoID?: Scalars['ID']
    isDir: boolean
    defaultBranch: string
    className: string
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
}

const SIZE_STORAGE_KEY = 'repo-revision-sidebar'
const TABS_KEY = 'repo-revision-sidebar-last-tab'
const SIDEBAR_KEY = 'repo-revision-sidebar-toggle'
/**
 * The sidebar for a specific repo revision that shows the list of files and directories.
 */
export const RepoRevisionSidebar: FC<RepoRevisionSidebarProps> = props => {
    const [persistedTabIndex, setPersistedTabIndex] = useLocalStorage(TABS_KEY, 0)
    const [persistedIsVisible, setPersistedIsVisible] = useLocalStorage(
        SIDEBAR_KEY,
        settingsSchemaJSON.properties.fileSidebarVisibleByDefault.default
    )
    const showOnboardingTour = useShowOnboardingTour({
        authenticatedUser: props.authenticatedUser,
        isSourcegraphDotCom: props.isSourcegraphDotCom,
    })

    const isWideScreen = useMatchMedia('(min-width: 768px)', false)
    const [isVisible, setIsVisible] = useState(persistedIsVisible && isWideScreen)

    const showFullTreeContextEnabled = useExperimentalFeatures(features => features.showFullTreeContext)
    const [initialFilePath, setInitialFilePath] = useState<string>(showFullTreeContextEnabled ? '' : props.filePath)
    const [initialFilePathIsDir, setInitialFilePathIsDir] = useState<boolean>(props.isDir)
    const onExpandParent = useCallback((parent: string) => {
        setInitialFilePath(parent)
        setInitialFilePathIsDir(true)
    }, [])

    const handleSidebarToggle = useCallback(
        (value: boolean) => {
            props.telemetryService.log('FileTreeViewClicked', {
                action: 'click',
                label: 'expand / collapse file tree view',
            })
            props.telemetryRecorder.recordEvent('blobSidebar.fileTreeView', 'click')
            setPersistedIsVisible(value)
            setIsVisible(value)
        },
        [setPersistedIsVisible, props.telemetryService, props.telemetryRecorder]
    )
    const handleSymbolClick = useCallback(() => {
        props.telemetryService.log('SymbolTreeViewClicked')
        props.telemetryRecorder.recordEvent('blobSidebar.symbolTreeView', 'click')
    }, [props.telemetryService, props.telemetryRecorder])

    const [enableBlobPageSwitchAreasShortcuts] = useFeatureFlag('blob-page-switch-areas-shortcuts')
    const focusFileTreeShortcut = useKeyboardShortcut('focusFileTree')
    const focusSymbolsShortcut = useKeyboardShortcut('focusSymbols')
    const [fileTreeFocusKey, setFileTreeFocusKey] = useState('')
    const [symbolsFocusKey, setSymbolsFocusKey] = useState('')

    return (
        <>
            {isVisible ? (
                <Panel
                    defaultSize={256}
                    minSize={150}
                    position="left"
                    storageKey={SIZE_STORAGE_KEY}
                    ariaLabel="File sidebar"
                >
                    <div className="d-flex flex-column h-100 w-100">
                        {showOnboardingTour && (
                            <GettingStartedTour
                                className="mr-3"
                                telemetryService={props.telemetryService}
                                authenticatedUser={props.authenticatedUser}
                            />
                        )}
                        <Tabs
                            className="w-100 test-repo-revision-sidebar h-25 d-flex flex-column flex-grow-1"
                            index={persistedTabIndex}
                            onChange={setPersistedTabIndex}
                            lazy={true}
                            // The individual tabs should keep their state when switching around (e.g. scroll
                            // position, which tree is expanded)
                            behavior="memoize"
                        >
                            <TabList
                                wrapperClassName="mr-3"
                                actions={
                                    <Tooltip content="Hide sidebar" placement="right">
                                        <Button
                                            aria-label="Hide sidebar"
                                            onClick={() => handleSidebarToggle(false)}
                                            className="bg-transparent border-0 ml-auto p-1 position-relative focus-behaviour"
                                        >
                                            <Icon
                                                className={styles.closeIcon}
                                                aria-hidden={true}
                                                svgPath={mdiChevronDoubleLeft}
                                            />
                                        </Button>
                                    </Tooltip>
                                }
                            >
                                <Tab data-tab-content="files">
                                    <span className="tablist-wrapper--tab-label">Files</span>
                                </Tab>
                                <Tab data-tab-content="symbols">
                                    <span className="tablist-wrapper--tab-label">Symbols</span>
                                </Tab>
                            </TabList>
                            <div
                                className={classNames('flex w-100 overflow-auto explorer pr-2', styles.tabpanels)}
                                tabIndex={-1}
                            >
                                {/* TODO: See if we can render more here, instead of waiting for these props */}
                                {props.repoID && props.commitID && (
                                    <TabPanels className="h-100">
                                        <TabPanel>
                                            <RepoRevisionSidebarFileTree
                                                key={initialFilePath}
                                                focusKey={fileTreeFocusKey}
                                                onExpandParent={onExpandParent}
                                                repoName={props.repoName}
                                                revision={props.revision}
                                                commitID={props.commitID}
                                                initialFilePath={initialFilePath}
                                                initialFilePathIsDirectory={initialFilePathIsDir}
                                                filePath={props.filePath}
                                                filePathIsDirectory={props.isDir}
                                                telemetryService={props.telemetryService}
                                                telemetryRecorder={props.telemetryRecorder}
                                            />
                                        </TabPanel>
                                        <TabPanel>
                                            <RepoRevisionSidebarSymbols
                                                key="symbols"
                                                focusKey={symbolsFocusKey}
                                                repoID={props.repoID}
                                                revision={props.revision}
                                                activePath={props.filePath}
                                                onHandleSymbolClick={handleSymbolClick}
                                            />
                                        </TabPanel>
                                    </TabPanels>
                                )}
                            </div>
                        </Tabs>
                    </div>
                </Panel>
            ) : (
                <Tooltip content="Show sidebar">
                    <Button
                        aria-label="Show sidebar"
                        variant="icon"
                        className={classNames(
                            'position-absolute border-top border-bottom border-right mt-4',
                            styles.toggle
                        )}
                        onClick={() => handleSidebarToggle(true)}
                    >
                        <Icon aria-hidden={true} svgPath={mdiChevronDoubleRight} />
                    </Button>
                </Tooltip>
            )}

            {enableBlobPageSwitchAreasShortcuts && (
                <>
                    {focusFileTreeShortcut?.keybindings.map((keybinding, index) => (
                        <Shortcut
                            key={index}
                            {...keybinding}
                            allowDefault={true}
                            onMatch={() => {
                                handleSidebarToggle(true)
                                setPersistedTabIndex(0)
                                setFileTreeFocusKey(Date.now().toString())
                            }}
                        />
                    ))}
                    {focusSymbolsShortcut?.keybindings.map((keybinding, index) => (
                        <Shortcut
                            key={index}
                            {...keybinding}
                            allowDefault={true}
                            onMatch={() => {
                                handleSidebarToggle(true)
                                setPersistedTabIndex(1)
                                setSymbolsFocusKey(Date.now().toString())
                            }}
                        />
                    ))}
                </>
            )}
        </>
    )
}
