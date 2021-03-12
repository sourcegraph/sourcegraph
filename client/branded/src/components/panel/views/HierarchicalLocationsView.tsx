import { Location } from '@sourcegraph/extension-api-types'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import SourceRepositoryIcon from 'mdi-react/SourceRepositoryIcon'
import * as H from 'history'
import * as React from 'react'
import { Observable, of, Subject, Subscription } from 'rxjs'
import { catchError, distinctUntilChanged, endWith, map, startWith, switchMap, tap } from 'rxjs/operators'
import { FetchFileParameters } from '../../../../../shared/src/components/CodeExcerpt'
import { RepoLink } from '../../../../../shared/src/components/RepoLink'
import { Resizable } from '../../../../../shared/src/components/Resizable'
import { ExtensionsControllerProps } from '../../../../../shared/src/extensions/controller'
import { SettingsCascadeProps } from '../../../../../shared/src/settings/settings'
import { asError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { parseRepoURI } from '../../../../../shared/src/util/url'
import { registerPanelToolbarContributions } from './contributions'
import { FileLocations, FileLocationsError, FileLocationsNotFound } from './FileLocations'
import { groupLocations } from './locations'
import { MaybeLoadingResult } from '@sourcegraph/codeintellify'
import { VersionContextProps } from '../../../../../shared/src/search/util'

/** The maximum number of results we'll receive from a provider before we truncate and display a banner. */
const MAXIMUM_LOCATION_RESULTS = 500

export interface HierarchicalLocationsViewProps
    extends SettingsCascadeProps,
        VersionContextProps,
        ExtensionsControllerProps<'extHostAPI'> {
    location: H.Location
    /**
     * The observable that emits the locations.
     */
    locations: Observable<MaybeLoadingResult<Location[]>>

    /**
     * In the grouping (i.e., by repository and, optionally, then by file), this is the URI of the first group.
     * Usually this is set to the URI to the root of the repository that is currently being viewed to ensure that
     * it is listed first.
     */
    defaultGroup: string

    /** Called when an item in the tree is selected. */
    onSelectTree?: () => void

    /** Called when a location is selected. */
    onSelectLocation?: () => void

    className?: string

    isLightTheme: boolean

    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>
}

interface State {
    /**
     * Locations (inside files identified by LSP-style git:// URIs) to display,
     * loading, or an error if they failed to load.
     *
     * Locations may be truncated if the result set is too large.
     */
    locationsOrError: MaybeLoadingResult<{ locations: Location[]; isTruncated: boolean } | ErrorLike>

    selectedGroups?: string[]
}

/**
 * Displays a multi-column view to drill down (by repository, file, etc.) to a list of locations in files.
 */
export class HierarchicalLocationsView extends React.PureComponent<HierarchicalLocationsViewProps, State> {
    public state: State = { locationsOrError: { isLoading: true, result: { locations: [], isTruncated: false } } }

    private componentUpdates = new Subject<HierarchicalLocationsViewProps>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        const locationProvidersChanges = this.componentUpdates.pipe(
            map(({ locations }) => locations),
            distinctUntilChanged()
        )

        this.subscriptions.add(
            locationProvidersChanges
                .pipe(
                    switchMap(locationProviderResults =>
                        locationProviderResults.pipe(
                            // Truncate the result set if it is too large,
                            // to avoid crashing the UI. A banner will be displayed to the user
                            // when this is the case.
                            map(({ isLoading, result: locations }) => {
                                const isTruncated = locations.length > MAXIMUM_LOCATION_RESULTS
                                return {
                                    isLoading,
                                    result: {
                                        locations: isTruncated
                                            ? locations.slice(0, MAXIMUM_LOCATION_RESULTS)
                                            : locations,
                                        isTruncated,
                                    },
                                }
                            }),
                            catchError((error): [State['locationsOrError']] => [
                                { isLoading: false, result: asError(error) },
                            ]),
                            startWith({
                                result: { locations: [], isTruncated: false },
                                isLoading: true,
                            }),
                            tap(({ result }) => {
                                const hasResults = !isErrorLike(result) && result.locations.length > 0
                                this.props.extensionsController.extHostAPI
                                    .then(extHostAPI =>
                                        extHostAPI.updateContext({
                                            'panel.locations.hasResults': hasResults,
                                        })
                                    )
                                    .catch(() => {
                                        // noop
                                    })
                            }),
                            endWith({ isLoading: false })
                        )
                    )
                )
                .subscribe(locationsOrError =>
                    this.setState(previous => ({
                        locationsOrError: {
                            ...previous.locationsOrError,
                            ...locationsOrError,
                        },
                    }))
                )
        )

        this.subscriptions.add(registerPanelToolbarContributions(this.props.extensionsController.extHostAPI))

        this.componentUpdates.next(this.props)
    }

    public componentDidUpdate(): void {
        this.componentUpdates.next(this.props)
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        if (isErrorLike(this.state.locationsOrError.result)) {
            return <FileLocationsError error={this.state.locationsOrError.result} />
        }
        if (this.state.locationsOrError.isLoading && this.state.locationsOrError.result.locations.length === 0) {
            return <LoadingSpinner className="icon-inline m-1 test-loading-spinner" />
        }
        if (this.state.locationsOrError.result.locations.length === 0) {
            return <FileLocationsNotFound />
        }

        const GROUPS: {
            name: string
            defaultSize: number
            key: (location: Location) => string | undefined
        }[] = [
            {
                name: 'repo',
                defaultSize: 175,
                key: location => parseRepoURI(location.uri).repoName,
            },
        ]
        const groupByFile =
            this.props.settingsCascade.final &&
            !isErrorLike(this.props.settingsCascade.final) &&
            this.props.settingsCascade.final['panel.locations.groupByFile']
        if (groupByFile) {
            GROUPS.push({
                name: 'file',
                defaultSize: 200,
                key: location => parseRepoURI(location.uri).filePath,
            })
        }

        const { groups, selectedGroups, visibleLocations } = groupLocations<Location, string>(
            this.state.locationsOrError.result.locations,
            this.state.selectedGroups || null,
            GROUPS.map(({ key }) => key),
            { uri: this.props.defaultGroup }
        )

        const groupsToDisplay = GROUPS.map(({ name, key, defaultSize }, index) => {
            const group = { name, key, defaultSize }
            if (!groups[index]) {
                // No groups exist at this level. Don't display anything.
                return null
            }
            if (groups[index].length > 1) {
                // Always display when there is more than 1 group.
                return group
            }
            if (groups[index].length === 1) {
                if (selectedGroups[index] !== groups[index][0].key) {
                    // When the only group is not the currently selected group, show it. This occurs when the
                    // references list changes after the user made an initial selection. The group must be shown so
                    // that the user can update their selection to the only available group; otherwise they would
                    // be stuck viewing the (zero) results from the previously selected group that no longer
                    // exists.
                    return group
                }
                if (key({ uri: this.props.defaultGroup }) !== selectedGroups[index]) {
                    // When the only group is other than the default group, show it. This is important because it
                    // often indicates that the match comes from another repository. If it isn't shown, the user
                    // would likely assume the match is from the current repository.
                    return group
                }
            }
            if (groupByFile && name === 'file') {
                // Always display the file groups when group-by-file is enabled.
                return group
            }
            return null
        })

        return (
            <div className="hierarchical-locations-wrapper">
                {this.state.locationsOrError.result.isTruncated && (
                    <div className="alert alert-warning py-1 px-3 m-2 text-nowrap text-center">
                        <small>
                            <strong>Large result set</strong> - only showing the first {MAXIMUM_LOCATION_RESULTS}{' '}
                            results.
                        </small>
                    </div>
                )}
                <div className={`hierarchical-locations-view ${this.props.className || ''}`}>
                    {selectedGroups &&
                        groupsToDisplay.map(
                            (group, index) =>
                                group && (
                                    <Resizable
                                        key={index}
                                        className="hierarchical-locations-view__resizable"
                                        handlePosition="right"
                                        storageKey={`hierarchical-locations-view-resizable:${group.name}`}
                                        defaultSize={group.defaultSize}
                                        element={
                                            <div className="list-group list-group-flush hierarchical-locations-view__list test-hierarchical-locations-view-list">
                                                {groups[index].map((group, innerIndex) => (
                                                    <span
                                                        key={innerIndex}
                                                        className={`list-group-item hierarchical-locations-view__item ${
                                                            selectedGroups[index] === group.key ? 'active' : ''
                                                        }`}
                                                        onClick={event =>
                                                            this.onSelectTree(event, selectedGroups, index, group.key)
                                                        }
                                                    >
                                                        <span
                                                            className="hierarchical-locations-view__item-name"
                                                            title={group.key}
                                                        >
                                                            <span className="hierarchical-locations-view__item-name-text">
                                                                <RepoLink to={null} repoName={group.key} />
                                                            </span>
                                                        </span>
                                                        <span className="badge badge-secondary badge-pill hierarchical-locations-view__item-badge">
                                                            {group.count}
                                                        </span>
                                                    </span>
                                                ))}
                                                {this.state.locationsOrError.isLoading && (
                                                    <LoadingSpinner className="icon-inline m-2 flex-shrink-0 test-loading-spinner" />
                                                )}
                                            </div>
                                        }
                                    />
                                )
                        )}
                    <FileLocations
                        className="hierarchical-locations-view__content"
                        location={this.props.location}
                        locations={of(visibleLocations)}
                        onSelect={this.props.onSelectLocation}
                        icon={SourceRepositoryIcon}
                        isLightTheme={this.props.isLightTheme}
                        fetchHighlightedFileLineRanges={this.props.fetchHighlightedFileLineRanges}
                        settingsCascade={this.props.settingsCascade}
                        versionContext={this.props.versionContext}
                    />
                </div>
            </div>
        )
    }

    private onSelectTree = (
        event: React.MouseEvent<HTMLElement>,
        selectedGroups: string[],
        index: number,
        group: string
    ): void => {
        event.preventDefault()
        this.setState({ selectedGroups: selectedGroups.slice(0, index).concat(group) })
        if (this.props.onSelectTree) {
            this.props.onSelectTree()
        }
    }
}
