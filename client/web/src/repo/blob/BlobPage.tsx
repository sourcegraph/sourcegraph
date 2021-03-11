import * as H from 'history'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import React, { useState, useEffect, useCallback, useMemo } from 'react'
import { Observable } from 'rxjs'
import { catchError, map, mapTo, startWith, switchMap } from 'rxjs/operators'
import { ExtensionsControllerProps } from '../../../../shared/src/extensions/controller'
import { gql, dataOrThrowErrors } from '../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../shared/src/graphql/schema'
import { PlatformContextProps } from '../../../../shared/src/platform/context'
import { SettingsCascadeProps } from '../../../../shared/src/settings/settings'
import { ErrorLike, isErrorLike, asError } from '../../../../shared/src/util/errors'
import { memoizeObservable } from '../../../../shared/src/util/memoizeObservable'
import {
    AbsoluteRepoFile,
    lprToRange,
    makeRepoURI,
    ModeSpec,
    ParsedRepoURI,
    parseHash,
} from '../../../../shared/src/util/url'
import { queryGraphQL } from '../../backend/graphql'
import { HeroPage } from '../../components/HeroPage'
import { PageTitle } from '../../components/PageTitle'
import { RepoHeaderContributionsLifecycleProps } from '../RepoHeader'
import { RepoHeaderContributionPortal } from '../RepoHeaderContributionPortal'
import { ToggleHistoryPanel } from './actions/ToggleHistoryPanel'
import { ToggleLineWrap } from './actions/ToggleLineWrap'
import { ToggleRenderedFileMode } from './actions/ToggleRenderedFileMode'
import { Blob, BlobInfo } from './Blob'
import { BlobPanel } from './panel/BlobPanel'
import { GoToRawAction } from './GoToRawAction'
import { RenderedFile } from './RenderedFile'
import { ThemeProps } from '../../../../shared/src/theme'
import { ErrorMessage } from '../../components/alerts'
import { Redirect } from 'react-router'
import { toTreeURL } from '../../util/url'
import { BreadcrumbSetters } from '../../components/Breadcrumbs'
import { useEventObservable } from '../../../../shared/src/util/useObservable'
import { FilePathBreadcrumbs } from '../FilePathBreadcrumbs'
import { AuthenticatedUser } from '../../auth'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { HoverThresholdProps } from '../RepoContainer'
import { Scalars } from '../../../../shared/src/graphql-operations'

function fetchBlobCacheKey(parsed: ParsedRepoURI & { isLightTheme: boolean; disableTimeout: boolean }): string {
    return makeRepoURI(parsed) + String(parsed.isLightTheme) + String(parsed.disableTimeout)
}

const fetchBlob = memoizeObservable(
    (args: {
        repoName: string
        commitID: string
        filePath: string
        isLightTheme: boolean
        disableTimeout: boolean
    }): Observable<GQL.File2> =>
        queryGraphQL(
            gql`
                query Blob(
                    $repoName: String!
                    $commitID: String!
                    $filePath: String!
                    $isLightTheme: Boolean!
                    $disableTimeout: Boolean!
                ) {
                    repository(name: $repoName) {
                        commit(rev: $commitID) {
                            file(path: $filePath) {
                                content
                                richHTML
                                highlight(disableTimeout: $disableTimeout, isLightTheme: $isLightTheme) {
                                    aborted
                                    html
                                }
                            }
                        }
                    }
                }
            `,
            args
        ).pipe(
            map(dataOrThrowErrors),
            map(data => {
                if (!data.repository?.commit?.file?.highlight) {
                    throw new Error('Not found')
                }
                return data.repository.commit.file
            })
        ),
    fetchBlobCacheKey
)

interface Props
    extends AbsoluteRepoFile,
        ModeSpec,
        RepoHeaderContributionsLifecycleProps,
        SettingsCascadeProps,
        PlatformContextProps,
        TelemetryProps,
        ExtensionsControllerProps,
        ThemeProps,
        HoverThresholdProps,
        BreadcrumbSetters {
    location: H.Location
    history: H.History
    repoID: Scalars['ID']
    authenticatedUser: AuthenticatedUser | null
}

export const BlobPage: React.FunctionComponent<Props> = props => {
    const [wrapCode, setWrapCode] = useState(ToggleLineWrap.getValue())
    let renderMode = ToggleRenderedFileMode.getModeFromURL(props.location)
    const { repoName, revision, commitID, filePath, isLightTheme, useBreadcrumb, mode } = props

    // Log view event whenever a new Blob, or a Blob with a different render mode, is visited.
    useEffect(() => {
        props.telemetryService.logViewEvent('Blob', { repoName, filePath })
    }, [repoName, commitID, filePath, isLightTheme, renderMode, props.telemetryService])

    useBreadcrumb(
        useMemo(() => {
            if (!filePath) {
                return
            }

            return {
                key: 'filePath',
                className: 'flex-shrink-past-contents',
                element: (
                    // TODO should these be "flattened" all using setBreadcrumb()?
                    <FilePathBreadcrumbs
                        key="path"
                        repoName={repoName}
                        revision={revision}
                        filePath={filePath}
                        isDir={false}
                    />
                ),
            }
        }, [filePath, revision, repoName])
    )

    // Bundle latest blob with all other file info to pass to `Blob`
    // Prevents https://github.com/sourcegraph/sourcegraph/issues/14965 by not allowing
    // components to use current file props while blob hasn't updated, since all information
    // is bundled in one object whose creation is blocked by `fetchBlob` emission.
    const [nextFetchWithDisabledTimeout, blobInfoOrError] = useEventObservable<
        void,
        (BlobInfo & { richHTML: string; aborted: boolean }) | ErrorLike
    >(
        useCallback(
            (clicks: Observable<void>) =>
                clicks.pipe(
                    mapTo(true),
                    startWith(false),
                    switchMap(disableTimeout =>
                        fetchBlob({
                            repoName,
                            commitID,
                            filePath,
                            isLightTheme,
                            disableTimeout,
                        })
                    ),
                    map(blob => {
                        const blobInfo: BlobInfo & { richHTML: string; aborted: boolean } = {
                            content: blob.content,
                            html: blob.highlight.html,
                            repoName,
                            revision,
                            commitID,
                            filePath,
                            mode,
                            isLightTheme,
                            // Properties used in `BlobPage` but not `Blob`
                            richHTML: blob.richHTML,
                            aborted: blob.highlight.aborted,
                        }
                        return blobInfo
                    }),
                    catchError((error): [ErrorLike] => {
                        console.error(error)
                        return [asError(error)]
                    })
                ),
            [repoName, revision, commitID, filePath, isLightTheme, mode]
        )
    )

    const onExtendTimeoutClick = useCallback(
        (event: React.MouseEvent): void => {
            event.preventDefault()
            nextFetchWithDisabledTimeout()
        },
        [nextFetchWithDisabledTimeout]
    )

    const getPageTitle = (): string => {
        const repoNameSplit = repoName.split('/')
        const repoString = repoNameSplit.length > 2 ? repoNameSplit.slice(1).join('/') : repoName
        if (filePath) {
            const fileOrDirectory = filePath.split('/').pop()!
            return `${fileOrDirectory} - ${repoString}`
        }
        return `${repoString}`
    }

    // Clear the Sourcegraph extensions model's component when the blob is no longer shown.
    useEffect(() => () => props.extensionsController.services.viewer.removeAllViewers(), [
        props.extensionsController.services.viewer,
    ])

    // If url explicitly asks for a certain rendering mode, renderMode is set to that mode, else it checks:
    // - If file contains richHTML and url does not include a line number: We render in richHTML.
    // - If file does not contain richHTML or the url includes a line number: We render in code view.
    if (!renderMode) {
        renderMode =
            blobInfoOrError &&
            !isErrorLike(blobInfoOrError) &&
            blobInfoOrError.richHTML &&
            !parseHash(props.location.hash).line
                ? 'rendered'
                : 'code'
    }

    // Always render these to avoid UI jitter during loading when switching to a new file.
    const alwaysRender = (
        <>
            <PageTitle title={getPageTitle()} />
            <RepoHeaderContributionPortal
                position="right"
                priority={20}
                element={
                    <ToggleHistoryPanel key="toggle-blob-panel" location={props.location} history={props.history} />
                }
                repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
            />
            {renderMode === 'code' && (
                <RepoHeaderContributionPortal
                    position="right"
                    priority={99}
                    element={<ToggleLineWrap key="toggle-line-wrap" onDidUpdate={setWrapCode} />}
                    repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
                />
            )}
            <RepoHeaderContributionPortal
                position="right"
                priority={30}
                element={
                    <GoToRawAction key="raw-action" repoName={repoName} revision={props.revision} filePath={filePath} />
                }
                repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
            />
            <BlobPanel
                {...props}
                position={
                    lprToRange(parseHash(props.location.hash))
                        ? lprToRange(parseHash(props.location.hash))!.start
                        : undefined
                }
            />
        </>
    )

    if (isErrorLike(blobInfoOrError)) {
        // Be helpful if the URL was actually a tree and redirect.
        // Some extensions may optimistically construct blob URLs because
        // they cannot easily determine eagerly if a file path is a tree or a blob.
        // We don't have error names on GraphQL errors.
        if (/not a blob/i.test(blobInfoOrError.message)) {
            return <Redirect to={toTreeURL(props)} />
        }
        return (
            <>
                {alwaysRender}
                <HeroPage icon={AlertCircleIcon} title="Error" subtitle={<ErrorMessage error={blobInfoOrError} />} />
            </>
        )
    }

    if (!blobInfoOrError) {
        // Render placeholder for layout before content is fetched.
        return <div className="blob-page__placeholder">{alwaysRender}</div>
    }

    return (
        <>
            {alwaysRender}
            {blobInfoOrError.richHTML && (
                <RepoHeaderContributionPortal
                    position="right"
                    priority={100}
                    element={
                        <ToggleRenderedFileMode
                            key="toggle-rendered-file-mode"
                            mode={renderMode || 'rendered'}
                            location={props.location}
                        />
                    }
                    repoHeaderContributionsLifecycleProps={props.repoHeaderContributionsLifecycleProps}
                />
            )}
            {blobInfoOrError.richHTML && renderMode === 'rendered' && (
                <RenderedFile
                    dangerousInnerHTML={blobInfoOrError.richHTML}
                    location={props.location}
                    history={props.history}
                />
            )}
            {!blobInfoOrError.richHTML && blobInfoOrError.aborted && (
                <div className="blob-page__aborted">
                    <div className="alert alert-info">
                        Syntax-highlighting this file took too long. &nbsp;
                        <button type="button" onClick={onExtendTimeoutClick} className="btn btn-sm btn-primary">
                            Try again
                        </button>
                    </div>
                </div>
            )}
            {/* Render the (unhighlighted) blob also in the case highlighting timed out */}
            {renderMode === 'code' && (
                <Blob
                    className="blob-page__blob test-repo-blob"
                    blobInfo={blobInfoOrError}
                    wrapCode={wrapCode}
                    platformContext={props.platformContext}
                    extensionsController={props.extensionsController}
                    settingsCascade={props.settingsCascade}
                    onHoverShown={props.onHoverShown}
                    history={props.history}
                    isLightTheme={isLightTheme}
                    telemetryService={props.telemetryService}
                    location={props.location}
                />
            )}
        </>
    )
}
