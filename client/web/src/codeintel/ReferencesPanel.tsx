import React, { KeyboardEvent, MouseEvent, useCallback, useEffect, useLayoutEffect, useMemo, useState } from 'react'

import { mdiArrowCollapseRight, mdiChevronDown, mdiChevronUp, mdiFilterOutline, mdiOpenInNew } from '@mdi/js'
import classNames from 'classnames'
import * as H from 'history'
import { capitalize, uniqBy } from 'lodash'
import { useNavigate, useLocation } from 'react-router-dom'
import { Observable, of } from 'rxjs'
import { map } from 'rxjs/operators'

import { CodeExcerpt, onClickCodeExcerptHref } from '@sourcegraph/branded'
import { HoveredToken } from '@sourcegraph/codeintellify'
import {
    addLineRangeQueryParameter,
    ErrorLike,
    formatSearchParameters,
    logger,
    lprToRange,
    pluralize,
    toPositionOrRangeQueryParameter,
} from '@sourcegraph/common'
import { Position } from '@sourcegraph/extension-api-classes'
import { useQuery } from '@sourcegraph/http-client'
import { FetchFileParameters } from '@sourcegraph/shared/src/backend/file'
import { LanguageSpec } from '@sourcegraph/shared/src/codeintel/legacy-extensions/language-specs/language-spec'
import { findLanguageSpec } from '@sourcegraph/shared/src/codeintel/legacy-extensions/language-specs/languages'
import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import { HighlightResponseFormat } from '@sourcegraph/shared/src/graphql-operations'
import { getModeFromPath } from '@sourcegraph/shared/src/languages'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { SettingsCascadeProps, useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    RepoSpec,
    RevisionSpec,
    FileSpec,
    ResolvedRevisionSpec,
    parseQueryAndHash,
    toPrettyBlobURL,
} from '@sourcegraph/shared/src/util/url'
import {
    Link,
    LoadingSpinner,
    CardHeader,
    useDebounce,
    Button,
    Input,
    Icon,
    Badge,
    Collapse,
    CollapseHeader,
    CollapsePanel,
    Code,
    H4,
    Text,
    Tooltip,
    useSessionStorage,
} from '@sourcegraph/wildcard'

import { ReferencesPanelHighlightedBlobResult, ReferencesPanelHighlightedBlobVariables } from '../graphql-operations'
import { CodeMirrorBlob } from '../repo/blob/CodeMirrorBlob'
import { LegacyBlob } from '../repo/blob/LegacyBlob'
import * as BlobAPI from '../repo/blob/use-blob-store'
import { HoverThresholdProps } from '../repo/RepoContainer'
import { parseBrowserRepoURL } from '../util/url'

import { Location, LocationGroup, locationGroupQuality, buildRepoLocationGroups, RepoLocationGroup } from './location'
import { FETCH_HIGHLIGHTED_BLOB } from './ReferencesPanelQueries'
import { newSettingsGetter } from './settings'
import { findSearchToken } from './token'
import { useRepoAndBlob } from './useRepoAndBlob'
import { isDefined } from './util/helpers'

import { CodeIntelligenceProps } from '.'

import styles from './ReferencesPanel.module.scss'

type Token = HoveredToken & RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec

interface HighlightedFileLineRangesProps {
    fetchHighlightedFileLineRanges: (parameters: FetchFileParameters, force?: boolean) => Observable<string[][]>
}

export interface ReferencesPanelProps
    extends SettingsCascadeProps,
        PlatformContextProps,
        Pick<CodeIntelligenceProps, 'useCodeIntel'>,
        TelemetryProps,
        HoverThresholdProps,
        HighlightedFileLineRangesProps {
    /** Whether to show the first loaded reference in mini code view */
    jumpToFirst?: boolean

    /**
     * Used to overwrite the initial active URL
     */
    initialActiveURL?: string
}
interface State {
    repoName: string
    revision?: string
    filePath: string
    line: number
    character: number
    jumpToFirst: boolean
    collapsedState: {
        references: boolean
        definitions: boolean
        implementations: boolean
    }
}

function createStateFromLocation(location: H.Location): null | State {
    const { hash, pathname, search } = location
    const { line, character, viewState } = parseQueryAndHash(search, hash)
    const { filePath, repoName, revision } = parseBrowserRepoURL(pathname)

    // If we don't have enough information in the URL, we can't render the panel
    if (!(line && character && filePath)) {
        return null
    }

    const searchParameters = new URLSearchParams(search)
    const jumpToFirst = searchParameters.get('jumpToFirst') === 'true'

    const collapsedState: State['collapsedState'] = {
        references: viewState === 'references',
        definitions: viewState === 'definitions',
        implementations: viewState?.startsWith('implementations_') ?? false,
    }
    // If the URL doesn't contain tab=<tab>, we open it (likely because the
    // user clicked on a link in the preview code blob) to show definitions.
    if (!collapsedState.references && !collapsedState.definitions && !collapsedState.implementations) {
        collapsedState.definitions = true
    }

    return { repoName, revision, filePath, line, character, jumpToFirst, collapsedState }
}

export const ReferencesPanel: React.FunctionComponent<React.PropsWithChildren<ReferencesPanelProps>> = props => {
    const location = useLocation()
    const state = useMemo(() => createStateFromLocation(location), [location])

    if (state === null) {
        return null
    }

    return (
        <RevisionResolvingReferencesList
            {...props}
            repoName={state.repoName}
            revision={state.revision}
            filePath={state.filePath}
            line={state.line}
            character={state.character}
            jumpToFirst={state.jumpToFirst}
            collapsedState={state.collapsedState}
        />
    )
}

export const RevisionResolvingReferencesList: React.FunctionComponent<
    React.PropsWithChildren<
        ReferencesPanelProps & {
            repoName: string
            line: number
            character: number
            filePath: string
            revision?: string
            collapsedState: State['collapsedState']
        }
    >
> = props => {
    const { data, loading, error } = useRepoAndBlob(props.repoName, props.filePath, props.revision)

    // Scroll blob UI to the selected symbol right after the reference panel is rendered
    // and shifted the blob UI (scroll into view is needed because there are a few cases
    // when ref panel may overlap with current symbol)
    useEffect(() => BlobAPI.scrollIntoView({ line: props.line }), [props.line])

    if (loading && !data) {
        return <LoadingCodeIntel />
    }

    if (error && !data) {
        return <LoadingCodeIntelFailed error={error} />
    }

    if (!data) {
        return <>Nothing found</>
    }

    const useCodeIntel = props.useCodeIntel
    if (!useCodeIntel) {
        return <>Code intelligence is not available</>
    }

    const token = {
        repoName: props.repoName,
        line: props.line,
        character: props.character,
        filePath: props.filePath,
        revision: data.revision,
        commitID: data.commitID,
    }

    return (
        <SearchTokenFindingReferencesList
            {...props}
            useCodeIntel={useCodeIntel}
            token={token}
            isFork={data.isFork}
            isArchived={data.isArchived}
            fileContent={data.fileContent}
        />
    )
}

interface ReferencesPanelPropsWithToken extends ReferencesPanelProps {
    token: Token
    isFork: boolean
    isArchived: boolean
    fileContent: string
    useCodeIntel: NonNullable<ReferencesPanelProps['useCodeIntel']>
    collapsedState: State['collapsedState']
}

const SearchTokenFindingReferencesList: React.FunctionComponent<
    React.PropsWithChildren<ReferencesPanelPropsWithToken>
> = props => {
    const languageId = getModeFromPath(props.token.filePath)
    const spec = findLanguageSpec(languageId)
    const tokenResult =
        spec &&
        findSearchToken({
            text: props.fileContent,
            position: {
                line: props.token.line - 1,
                character: props.token.character - 1,
            },
            lineRegexes: spec.commentStyles.map(style => style.lineRegex).filter(isDefined),
            blockCommentStyles: spec.commentStyles.map(style => style.block).filter(isDefined),
            identCharPattern: spec.identCharPattern,
        })
    const shouldMixPreciseAndSearchBasedReferences: boolean = newSettingsGetter(props.settingsCascade)<boolean>(
        'codeIntel.mixPreciseAndSearchBasedReferences',
        false
    )

    if (!spec || !tokenResult?.searchToken) {
        return (
            <div>
                <Text className="text-danger">Could not find token.</Text>
            </div>
        )
    }

    return (
        <ReferencesList
            // Force the references list to recreate when the user settings
            // change. This way we avoid showing stale results.
            key={shouldMixPreciseAndSearchBasedReferences.toString()}
            {...props}
            token={props.token}
            searchToken={tokenResult?.searchToken}
            spec={spec}
            fileContent={props.fileContent}
            isFork={props.isFork}
            isArchived={props.isArchived}
        />
    )
}

const SHOW_SPINNER_DELAY_MS = 100

const ReferencesList: React.FunctionComponent<
    React.PropsWithChildren<
        ReferencesPanelPropsWithToken & {
            searchToken: string
            spec: LanguageSpec
            fileContent: string
            collapsedState: State['collapsedState']
        }
    >
> = props => {
    const [filter, setFilter] = useState<string>()
    const debouncedFilter = useDebounce(filter, 150)

    const navigate = useNavigate()

    useEffect(() => {
        setFilter(undefined)
    }, [props.token])

    const getSetting = newSettingsGetter(props.settingsCascade)

    const {
        data,
        error,
        loading,
        referencesHasNextPage,
        implementationsHasNextPage,
        fetchMoreReferences,
        fetchMoreImplementations,
        fetchMoreReferencesLoading,
        fetchMoreImplementationsLoading,
    } = props.useCodeIntel({
        variables: {
            repository: props.token.repoName,
            commit: props.token.commitID,
            path: props.token.filePath,
            // On the backend the line/character are 0-indexed, but what we
            // get from hoverifier is 1-indexed.
            line: props.token.line - 1,
            character: props.token.character - 1,
            filter: debouncedFilter || null,
            firstReferences: 100,
            afterReferences: null,
            firstImplementations: 100,
            afterImplementations: null,
        },
        fileContent: props.fileContent,
        searchToken: props.searchToken,
        spec: props.spec,
        isFork: props.isFork,
        isArchived: props.isArchived,
        getSetting,
    })

    // We only show the inline loading message if loading takes longer than
    // SHOW_SPINNER_DELAY_MS milliseconds.
    const [canShowSpinner, setCanShowSpinner] = useState(false)
    useEffect(() => {
        const handle = setTimeout(() => setCanShowSpinner(loading), SHOW_SPINNER_DELAY_MS)
        // In case the component un-mounts before
        return () => clearTimeout(handle)
        // Make sure this effect only runs once
    }, [loading])

    const references = useMemo(() => data?.references.nodes ?? [], [data])
    const definitions = useMemo(() => data?.definitions.nodes ?? [], [data])
    const implementations = useMemo(() => data?.implementations.nodes ?? [], [data])

    // The "active URL" is the URL of the highlighted line number in SideBlob,
    // which also influences which item gets highlighted inside
    // CollapsibleLocationList. This URL is persisted to session storage so that
    // it remains sticky between browser reloads and when pressing back/forward
    // in the browser history.
    const [activeURL, setActiveURL] = useSessionStorage<string | undefined>(
        'sideblob-active-url' + sessionStorageKeyFromToken(props.token),
        props.initialActiveURL
    )
    const setActiveLocation = useCallback(
        (location: Location | undefined): void => {
            if (!location) {
                setActiveURL(undefined)
                return
            }
            // Reconstruct the URL instead of using `location.url` to ensure that
            // the commitID is included even when `location.url` doesn't include the
            // commitID (because it's the default revision '').
            const absoluteURL = toPrettyBlobURL({
                filePath: location.file,
                revision: location.commitID,
                repoName: location.repo,
                commitID: location.commitID,
                range: location.range
                    ? {
                          start: {
                              line: location.range.start.line + 1,
                              character: location.range.start.character + 1,
                          },
                          end: {
                              line: location.range.end.line + 1,
                              character: location.range.end.character + 1,
                          },
                      }
                    : undefined,
            })
            setActiveURL(absoluteURL)
        },
        [setActiveURL]
    )

    const sideblob = useMemo(() => parseSideBlobProps(activeURL), [activeURL])

    const isActiveLocation = (location: Location): boolean => {
        const result =
            (sideblob?.position &&
                location.range &&
                sideblob.repository === location.repo &&
                sideblob.file === location.file &&
                sideblob.commitID === location.commitID &&
                sideblob.position.line === location.range.start.line) ||
            false
        return result
    }

    // If props.jumpToFirst is true and we finished loading (and have
    // definitions) we select the first definition. We set it as activeLocation
    // and push it to the blobMemoryHistory so the code blob is open.
    useEffect(() => {
        if (props.jumpToFirst && definitions.length > 0) {
            setActiveLocation(definitions[0])
        }
    }, [setActiveLocation, props.jumpToFirst, definitions, setActiveURL])

    const onBlobNav = (url: string): void => {
        // Store the URL that the user promoted even if no definition/reference
        // points to the same line. In case they press "back" in the browser history,
        // the promoted line should be highlighted.
        setActiveURL(url)
        navigate(url)
    }

    const navigateToUrl = (url: string): void => {
        navigate(url)
    }

    const [collapsed, setCollapsed] = useSessionStorage<Record<string, boolean>>(
        'sideblob-collapse-state-' + sessionStorageKeyFromToken(props.token),
        props.collapsedState
    )

    const handleOpenChange = (id: string, isOpen: boolean): void =>
        setCollapsed(previous => ({ ...previous, [id]: isOpen }))

    const isOpen = (id: string): boolean | undefined => collapsed[id]

    if (loading && !data) {
        return <LoadingCodeIntel />
    }

    // If we received an error before we had received any data
    if (error && !data) {
        return <LoadingCodeIntelFailed error={error} />
    }

    // If there weren't any errors and we just didn't receive any data
    if (!data) {
        return <>Nothing found</>
    }

    return (
        <div className={classNames('align-items-stretch', styles.panel)}>
            <div className={classNames('px-0', styles.leftSubPanel)}>
                <div className={classNames('d-flex justify-content-start mt-2', styles.filter)}>
                    <small>
                        <Icon
                            aria-hidden={true}
                            as={canShowSpinner ? LoadingSpinner : undefined}
                            svgPath={!canShowSpinner ? mdiFilterOutline : undefined}
                            size="md"
                            className={styles.filterIcon}
                        />
                    </small>
                    <Input
                        className={classNames('py-0 my-0 w-100 text-small')}
                        type="text"
                        placeholder="Type to filter by filename"
                        value={filter === undefined ? '' : filter}
                        onChange={event => setFilter(event.target.value)}
                    />
                </div>
                <div className={styles.locationLists}>
                    <CollapsibleLocationList
                        {...props}
                        name="definitions"
                        locations={definitions}
                        hasMore={false}
                        loadingMore={false}
                        filter={debouncedFilter}
                        activeURL={activeURL || ''}
                        navigateToUrl={navigateToUrl}
                        isActiveLocation={isActiveLocation}
                        setActiveLocation={setActiveLocation}
                        handleOpenChange={handleOpenChange}
                        isOpen={isOpen}
                    />
                    <CollapsibleLocationList
                        {...props}
                        name="references"
                        locations={references}
                        hasMore={referencesHasNextPage}
                        fetchMore={fetchMoreReferences}
                        loadingMore={fetchMoreReferencesLoading}
                        filter={debouncedFilter}
                        activeURL={activeURL || ''}
                        navigateToUrl={navigateToUrl}
                        setActiveLocation={setActiveLocation}
                        isActiveLocation={isActiveLocation}
                        handleOpenChange={handleOpenChange}
                        isOpen={isOpen}
                    />
                    <CollapsibleLocationList
                        {...props}
                        name="implementations"
                        locations={implementations}
                        hasMore={implementationsHasNextPage}
                        fetchMore={fetchMoreImplementations}
                        loadingMore={fetchMoreImplementationsLoading}
                        setActiveLocation={setActiveLocation}
                        filter={debouncedFilter}
                        isActiveLocation={isActiveLocation}
                        activeURL={activeURL || ''}
                        navigateToUrl={navigateToUrl}
                        handleOpenChange={handleOpenChange}
                        isOpen={isOpen}
                    />
                </div>
            </div>
            {sideblob && (
                <div data-testid="right-pane" className={classNames('px-0 border-left', styles.rightSubPanel)}>
                    <CardHeader className={classNames('d-flex', styles.cardHeader)}>
                        <small>
                            <Tooltip content="Close code view" placement="left">
                                <Button
                                    aria-label="Close"
                                    onClick={() => setActiveLocation(undefined)}
                                    className={classNames('p-0', styles.sideBlobCollapseButton)}
                                    size="sm"
                                    data-testid="close-code-view"
                                >
                                    <Icon
                                        aria-hidden={true}
                                        size="sm"
                                        svgPath={mdiArrowCollapseRight}
                                        className="border-0"
                                    />
                                </Button>
                            </Tooltip>
                            {activeURL && (
                                <Link
                                    to={activeURL}
                                    onClick={event => {
                                        event.preventDefault()
                                        navigateToUrl(activeURL)
                                    }}
                                    className={styles.sideBlobFilename}
                                >
                                    {sideblob.file}{' '}
                                </Link>
                            )}
                        </small>
                    </CardHeader>
                    <SideBlob {...props} {...sideblob} blobNav={onBlobNav} />
                </div>
            )}
        </div>
    )
}

interface SearchTokenProps {
    searchToken: string
}

interface CollapseProps {
    isOpen: (id: string) => boolean | undefined
    handleOpenChange: (id: string, isOpen: boolean) => void
}

interface ActiveLocationProps {
    isActiveLocation: (location: Location) => boolean
    setActiveLocation: (reference: Location | undefined) => void
}

interface CollapsibleLocationListProps
    extends ActiveLocationProps,
        CollapseProps,
        SearchTokenProps,
        HighlightedFileLineRangesProps {
    name: string
    locations: Location[]
    filter: string | undefined
    hasMore: boolean
    fetchMore?: () => void
    loadingMore: boolean
    navigateToUrl: (url: string) => void
    activeURL: string
}

const CollapsibleLocationList: React.FunctionComponent<
    React.PropsWithChildren<CollapsibleLocationListProps>
> = props => {
    const isOpen = props.isOpen(props.name) ?? true
    const quantityLabel = useMemo(() => {
        const repoNumber = uniqBy(props.locations, 'repo').length
        return `(${props.locations.length} ${pluralize('item', props.locations.length)}${
            repoNumber > 1 ? ` from ${repoNumber} repositories` : ''
        } displayed${props.hasMore ? ', more available' : ''})`
    }, [props.locations, props.hasMore])

    return (
        <Collapse isOpen={isOpen} onOpenChange={isOpen => props.handleOpenChange(props.name, isOpen)}>
            <>
                <CardHeader className={styles.cardHeaderBig}>
                    <CollapseHeader
                        as={Button}
                        aria-expanded={props.isOpen(props.name)}
                        type="button"
                        className="d-flex p-0 justify-content-start w-100"
                    >
                        {isOpen ? (
                            <Icon aria-hidden={true} svgPath={mdiChevronUp} />
                        ) : (
                            <Icon aria-hidden={true} svgPath={mdiChevronDown} />
                        )}{' '}
                        <H4 className="mb-0">{capitalize(props.name)}</H4>
                        <span className={classNames('ml-2 text-muted small', styles.cardHeaderSmallText)}>
                            {quantityLabel}
                        </span>
                    </CollapseHeader>
                </CardHeader>

                <CollapsePanel id={props.name} data-testid={props.name}>
                    {props.locations.length > 0 ? (
                        <LocationsList
                            searchToken={props.searchToken}
                            locations={props.locations}
                            isActiveLocation={props.isActiveLocation}
                            setActiveLocation={props.setActiveLocation}
                            filter={props.filter}
                            activeURL={props.activeURL}
                            navigateToUrl={props.navigateToUrl}
                            handleOpenChange={(id, isOpen) => props.handleOpenChange(props.name + id, isOpen)}
                            isOpen={id => props.isOpen(props.name + id)}
                            fetchHighlightedFileLineRanges={props.fetchHighlightedFileLineRanges}
                        />
                    ) : (
                        <Text className="text-muted pl-4 pb-0">
                            {props.filter ? (
                                <i>
                                    No {props.name} matching <strong>{props.filter}</strong> found
                                </i>
                            ) : (
                                <i>No {props.name} found</i>
                            )}
                        </Text>
                    )}

                    {props.hasMore &&
                        props.fetchMore !== undefined &&
                        (props.loadingMore ? (
                            <div className="text-center mb-1">
                                <em>Loading more {props.name}...</em>
                                <LoadingSpinner inline={true} />
                            </div>
                        ) : (
                            <div className="text-center mb-1">
                                <Button variant="secondary" onClick={props.fetchMore}>
                                    Load more {props.name}
                                </Button>
                            </div>
                        ))}
                </CollapsePanel>
            </>
        </Collapse>
    )
}

interface SideBlobProps extends ReferencesPanelProps {
    activeURL: string
    repository: string
    commitID: string
    file: string
    position?: Position
    blobNav: (url: string) => void
}

function parseSideBlobProps(
    activeURL: string | undefined
): Pick<SideBlobProps, 'activeURL' | 'repository' | 'commitID' | 'file' | 'position'> | undefined {
    if (!activeURL) {
        return undefined
    }
    try {
        const url = parseBrowserRepoURL(activeURL)
        if (!url.repoName || !url.filePath) {
            return undefined
        }

        const position = url.position
            ? new Position(Math.max(url.position.line - 1), Math.max(0, url.position.character - 1))
            : undefined
        return { activeURL, repository: url.repoName, commitID: url.commitID || '', file: url.filePath, position }
    } catch (error) {
        logger.error(`failed to parse activeURL ${activeURL}`, error)
        return undefined
    }
}

const SideBlob: React.FunctionComponent<React.PropsWithChildren<SideBlobProps>> = props => {
    const useCodeMirror = useExperimentalFeatures(features => features.enableCodeMirrorFileView ?? true)
    const BlobComponent = useCodeMirror ? CodeMirrorBlob : LegacyBlob

    const highlightFormat = useCodeMirror ? HighlightResponseFormat.JSON_SCIP : HighlightResponseFormat.HTML_HIGHLIGHT
    const { data, error, loading } = useQuery<
        ReferencesPanelHighlightedBlobResult,
        ReferencesPanelHighlightedBlobVariables
    >(FETCH_HIGHLIGHTED_BLOB, {
        variables: {
            repository: props.repository,
            commit: props.commitID,
            path: props.file,
            format: highlightFormat,
            html: highlightFormat === HighlightResponseFormat.HTML_HIGHLIGHT,
        },
        // Cache this data but always re-request it in the background when we revisit
        // this page to pick up newer changes.
        fetchPolicy: 'cache-and-network',
        nextFetchPolicy: 'network-only',
    })

    // If we're loading and haven't received any data yet
    if (loading && !data) {
        return (
            <>
                <LoadingSpinner inline={false} className="mx-auto my-4" />
                <Text alignment="center" className="text-muted">
                    <i>
                        Loading <Code>{props.file}</Code>...
                    </i>
                </Text>
            </>
        )
    }

    // If we received an error before we had received any data
    if (error && !data) {
        return (
            <div>
                <Text className="text-danger">
                    Loading <Code>{props.file}</Code> failed:
                </Text>
                <pre>{error.message}</pre>
            </div>
        )
    }

    // If there weren't any errors and we just didn't receive any data
    if (!data?.repository?.commit?.blob?.highlight) {
        return <>Nothing found</>
    }

    const { html, lsif } = data?.repository?.commit?.blob?.highlight

    // TODO: display a helpful message if syntax highlighting aborted, see https://github.com/sourcegraph/sourcegraph/issues/40841

    return (
        <BlobComponent
            {...props}
            nav={props.blobNav}
            wrapCode={true}
            className={styles.sideBlobCode}
            navigateToLineOnAnyClick={true}
            blobInfo={{
                html: html ?? '',
                lsif: lsif ?? '',
                content: data?.repository?.commit?.blob?.content ?? '',
                filePath: props.file,
                repoName: props.repository,
                commitID: props.commitID,
                revision: props.commitID,
                mode: 'lspmode',
            }}
        />
    )
}

interface LocationsListProps
    extends ActiveLocationProps,
        CollapseProps,
        SearchTokenProps,
        HighlightedFileLineRangesProps {
    locations: Location[]
    filter: string | undefined
    navigateToUrl: (url: string) => void
    activeURL: string
}

const LocationsList: React.FunctionComponent<React.PropsWithChildren<LocationsListProps>> = ({
    locations,
    isActiveLocation,
    setActiveLocation,
    filter,
    navigateToUrl,
    handleOpenChange,
    isOpen,
    searchToken,
    fetchHighlightedFileLineRanges,
    activeURL,
}) => {
    const repoLocationGroups = useMemo(() => buildRepoLocationGroups(locations), [locations])

    return (
        <>
            {repoLocationGroups.map((group, index) => (
                <CollapsibleRepoLocationGroup
                    key={group.repoName}
                    activeURL={activeURL}
                    searchToken={searchToken}
                    repoLocationGroup={group}
                    openByDefault={index === 0}
                    isActiveLocation={isActiveLocation}
                    setActiveLocation={setActiveLocation}
                    filter={filter}
                    navigateToUrl={navigateToUrl}
                    handleOpenChange={handleOpenChange}
                    isOpen={isOpen}
                    fetchHighlightedFileLineRanges={fetchHighlightedFileLineRanges}
                />
            ))}
        </>
    )
}

const CollapsibleRepoLocationGroup: React.FunctionComponent<
    React.PropsWithChildren<
        ActiveLocationProps &
            CollapseProps &
            SearchTokenProps &
            HighlightedFileLineRangesProps & {
                filter: string | undefined
                navigateToUrl: (url: string) => void
                repoLocationGroup: RepoLocationGroup
                openByDefault: boolean
                activeURL: string
            }
    >
> = ({
    repoLocationGroup,
    isActiveLocation,
    setActiveLocation,
    navigateToUrl,
    filter,
    openByDefault,
    isOpen,
    handleOpenChange,
    searchToken,
    fetchHighlightedFileLineRanges,
    activeURL,
}) => {
    const open = isOpen(repoLocationGroup.repoName) ?? openByDefault

    return (
        <Collapse isOpen={open} onOpenChange={isOpen => handleOpenChange(repoLocationGroup.repoName, isOpen)}>
            <div className={styles.repoLocationGroup}>
                <CollapseHeader
                    as={Button}
                    aria-expanded={open}
                    aria-label={`Repository ${repoLocationGroup.repoName}`}
                    type="button"
                    className={classNames('d-flex justify-content-start w-100', styles.repoLocationGroupHeader)}
                >
                    <Icon aria-hidden="true" svgPath={open ? mdiChevronUp : mdiChevronDown} />
                    <small>
                        <span className={classNames('text-small', styles.repoLocationGroupHeaderRepoName)}>
                            {displayRepoName(repoLocationGroup.repoName)}
                        </span>
                    </small>
                </CollapseHeader>

                <CollapsePanel id={repoLocationGroup.repoName}>
                    {repoLocationGroup.referenceGroups.map(group => (
                        <CollapsibleLocationGroup
                            key={group.path + group.repoName}
                            activeURL={activeURL}
                            searchToken={searchToken}
                            group={group}
                            isActiveLocation={isActiveLocation}
                            setActiveLocation={setActiveLocation}
                            filter={filter}
                            handleOpenChange={(id, isOpen) => handleOpenChange(repoLocationGroup.repoName + id, isOpen)}
                            isOpen={id => isOpen(repoLocationGroup.repoName + id)}
                            navigateToUrl={navigateToUrl}
                            fetchHighlightedFileLineRanges={fetchHighlightedFileLineRanges}
                        />
                    ))}
                </CollapsePanel>
            </div>
        </Collapse>
    )
}

const CollapsibleLocationGroup: React.FunctionComponent<
    React.PropsWithChildren<
        ActiveLocationProps &
            CollapseProps &
            SearchTokenProps &
            HighlightedFileLineRangesProps & {
                group: LocationGroup
                filter: string | undefined
                navigateToUrl: (url: string) => void
                activeURL: string
            }
    >
> = ({
    group,
    setActiveLocation,
    isActiveLocation,
    filter,
    isOpen,
    handleOpenChange,
    fetchHighlightedFileLineRanges,
    navigateToUrl,
    activeURL,
}) => {
    // On the first load, update the scroll position towards the active
    // location.  Without this behavior, the scroll position points at the top
    // of the reference panel when reloading the page or going back/forward in
    // the browser history.
    useLayoutEffect(() => {
        const activeLocationElement = document.querySelector('.' + styles.locationActive)
        if (activeLocationElement) {
            activeLocationElement.scrollIntoView({ behavior: 'auto', block: 'center', inline: 'center' })
        }
    }, [])
    let highlighted = [group.path]
    if (filter !== undefined) {
        highlighted = group.path.split(filter)
    }

    const { repo, commitID, file } = useMemo(() => group.locations[0], [group])
    const ranges = useMemo(
        () =>
            group.locations.map(location => ({
                startLine: location.range?.start.line ?? 0,
                endLine: (location.range?.end.line ?? 0) + 1,
            })),
        [group.locations]
    )

    const fetchHighlightedFileRangeLines = useCallback(
        (startLine: number, endLine: number): Observable<string[]> =>
            fetchHighlightedFileLineRanges(
                {
                    repoName: repo,
                    commitID,
                    filePath: file,
                    disableTimeout: false,
                    format: HighlightResponseFormat.HTML_HIGHLIGHT,
                    ranges,
                },
                false
            ).pipe(
                map(
                    lines =>
                        lines[ranges.findIndex(group => group.startLine === startLine && group.endLine === endLine + 1)]
                )
            ),
        [fetchHighlightedFileLineRanges, repo, commitID, file, ranges]
    )

    const fetchPlainTextFileRangeLines = (location: Location): Observable<string[]> => {
        const range = location.range
        if (range !== undefined) {
            const lineNumber = range.start.line + 1
            const lineContent = location.lines[range.start.line]
            const tableLine = `<tr><td class="line" data-line="${lineNumber}"></td><td class="code">${lineContent}</td></tr>`
            return of([tableLine])
        }
        return of([])
    }

    const open = isOpen(group.path) ?? true

    return (
        <Collapse isOpen={open} onOpenChange={isOpen => handleOpenChange(group.path, isOpen)}>
            <div className={styles.locationGroup}>
                <CollapseHeader
                    as={Button}
                    aria-expanded={open}
                    type="button"
                    className={classNames(
                        'bg-transparent border-top-0 border-left-0 border-right-0 d-flex justify-content-start w-100',
                        styles.locationGroupHeader
                    )}
                >
                    {open ? (
                        <Icon aria-hidden={true} svgPath={mdiChevronUp} />
                    ) : (
                        <Icon aria-hidden={true} svgPath={mdiChevronDown} />
                    )}
                    <small className={styles.locationGroupHeaderFilename}>
                        <span>
                            <span
                                aria-label={`File path ${group.path}`}
                                className={classNames('text-small', styles.repoLocationGroupHeaderRepoName)}
                            >
                                {highlighted.length === 2 ? (
                                    <span>
                                        {highlighted[0]}
                                        <mark>{filter}</mark>
                                        {highlighted[1]}
                                    </span>
                                ) : (
                                    group.path
                                )}{' '}
                            </span>
                            <span className={classNames('ml-2 text-muted', styles.cardHeaderSmallText)}>
                                ({group.locations.length}{' '}
                                {pluralize('occurrence', group.locations.length, 'occurrences')})
                            </span>
                        </span>
                        <Badge small={true} variant="secondary" className="ml-4">
                            {locationGroupQuality(group)}
                        </Badge>
                    </small>
                </CollapseHeader>

                <CollapsePanel id={group.repoName + group.path} className="ml-0">
                    <div className={styles.locationContainer}>
                        <ul className="list-unstyled mb-0">
                            {group.locations.map((reference, index) => {
                                const isActive = isActiveLocation(reference)
                                const isFirstInActive =
                                    isActive && !(index > 0 && isActiveLocation(group.locations[index - 1]))
                                const locationActive = isActive ? styles.locationActive : ''
                                const selectReference = (
                                    event: KeyboardEvent<HTMLElement> | MouseEvent<HTMLElement>
                                ): void => {
                                    onClickCodeExcerptHref(event, () => {
                                        if (isActive && activeURL) {
                                            navigateToUrl(activeURL)
                                        } else {
                                            setActiveLocation(reference)
                                        }
                                    })
                                }

                                return (
                                    <li
                                        key={reference.url}
                                        className={classNames('border-0 rounded-0 mb-0', styles.location)}
                                    >
                                        <div
                                            role="link"
                                            data-testid={`reference-item-${group.path}-${index}`}
                                            tabIndex={0}
                                            onClick={selectReference}
                                            onKeyDown={selectReference}
                                            data-href={reference.url}
                                            className={classNames(styles.locationLink, locationActive)}
                                        >
                                            <CodeExcerpt
                                                className={styles.locationLinkCodeExcerpt}
                                                commitID={reference.commitID}
                                                filePath={reference.file}
                                                repoName={reference.repo}
                                                highlightRanges={[
                                                    {
                                                        startLine: reference.range?.start.line ?? 0,
                                                        startCharacter: reference.range?.start.character ?? 0,
                                                        endLine: reference.range?.end.line ?? 0,
                                                        endCharacter: reference.range?.end.character ?? 0,
                                                    },
                                                ]}
                                                startLine={reference.range?.start.line ?? 0}
                                                endLine={reference.range?.end.line ?? 0}
                                                fetchHighlightedFileRangeLines={fetchHighlightedFileRangeLines}
                                                visibilityOffset={{ bottom: 0 }}
                                                fetchPlainTextFileRangeLines={(): Observable<string[]> =>
                                                    fetchPlainTextFileRangeLines(reference)
                                                }
                                            />
                                            {isFirstInActive ? (
                                                <span className={classNames('ml-2', styles.locationActiveIcon)}>
                                                    <Tooltip
                                                        content="Click again to open line in full view"
                                                        placement="left"
                                                    >
                                                        <Icon
                                                            aria-label="Open line in full view"
                                                            size="sm"
                                                            svgPath={mdiOpenInNew}
                                                        />
                                                    </Tooltip>
                                                </span>
                                            ) : null}
                                        </div>
                                    </li>
                                )
                            })}
                        </ul>
                    </div>
                </CollapsePanel>
            </div>
        </Collapse>
    )
}

const LoadingCodeIntel: React.FunctionComponent<React.PropsWithChildren<{}>> = () => (
    <>
        <LoadingSpinner inline={false} className="mx-auto my-4" />
        <Text alignment="center" className="text-muted">
            <i>Loading code intel ...</i>
        </Text>
    </>
)

const LoadingCodeIntelFailed: React.FunctionComponent<React.PropsWithChildren<{ error: ErrorLike }>> = props => (
    <>
        <div>
            <Text className="text-danger">Loading code intel failed:</Text>
            <pre>{props.error.message}</pre>
        </div>
    </>
)

export function locationWithoutViewState(location: H.Location): H.LocationDescriptorObject {
    const parsedQuery = parseQueryAndHash(location.search, location.hash)
    delete parsedQuery.viewState

    const lineRangeQueryParameter = toPositionOrRangeQueryParameter({ range: lprToRange(parsedQuery) })
    const result = {
        search: formatSearchParameters(
            addLineRangeQueryParameter(new URLSearchParams(location.search), lineRangeQueryParameter)
        ),
        hash: '',
    }
    return result
}

export const appendJumpToFirstQueryParameter = (url: string): string => {
    const newUrl = new URL(url, window.location.href)
    newUrl.searchParams.set('jumpToFirst', 'true')
    return newUrl.pathname + `?${formatSearchParameters(newUrl.searchParams)}` + newUrl.hash
}

function sessionStorageKeyFromToken(token: Token): string {
    return `${token.repoName}@${token.commitID}/${token.filePath}?L${token.line}:${token.character}`
}
