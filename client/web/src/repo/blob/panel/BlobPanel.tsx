import * as H from 'history'
import React, { useCallback, useEffect, useMemo } from 'react'
import { from, Observable, ReplaySubject, Subscription } from 'rxjs'
import { map, switchMap, tap } from 'rxjs/operators'
import * as clientType from '@sourcegraph/extension-api-types'
import { ReferenceParameters, TextDocumentPositionParameters } from '../../../../../shared/src/api/protocol'
import { ActivationProps } from '../../../../../shared/src/components/activation/Activation'
import { ExtensionsControllerProps } from '../../../../../shared/src/extensions/controller'
import { AbsoluteRepoFile, ModeSpec, parseHash, UIPositionSpec } from '../../../../../shared/src/util/url'
import { RepoRevisionSidebarCommits } from '../../RepoRevisionSidebarCommits'
import { MaybeLoadingResult } from '@sourcegraph/codeintellify'
import { finallyReleaseProxy, wrapRemoteObservable } from '../../../../../shared/src/api/client/api/common'
import { Scalars } from '../../../../../shared/src/graphql-operations'
import { useBuiltinPanelViews } from '../../../../../branded/src/components/panel/Panel'

interface Props extends AbsoluteRepoFile, ModeSpec, ExtensionsControllerProps, ActivationProps {
    location: H.Location
    history: H.History
    repoID: Scalars['ID']
    repoName: string
    commitID: string
}

export type BlobPanelTabID = 'info' | 'def' | 'references' | 'impl' | 'typedef' | 'history'

/** The subject (what the contextual information refers to). */
interface PanelSubject extends AbsoluteRepoFile, ModeSpec, Partial<UIPositionSpec> {
    repoID: string

    /**
     * Include the full URI fragment here because it represents the state of panels, and we want
     * panels to be re-rendered when this state changes.
     */
    hash: string

    history: H.History
    location: H.Location
}

/**
 * A React hook that registers panel views for the blob.
 */
export function useBlobPanelViews({
    extensionsController,
    activation,
    repoName,
    commitID,
    revision,
    mode,
    filePath,
    repoID,
    location,
    history,
}: Props): void {
    const subscriptions = useMemo(() => new Subscription(), [])

    // Creates source for definition and reference panels
    const createLocationProvider = useCallback(
        <P extends TextDocumentPositionParameters>(
            id: string,
            title: string,
            priority: number,
            provideLocations: (parameters: P) => Observable<MaybeLoadingResult<clientType.Location[]>>,
            extraParameters?: Pick<P, Exclude<keyof P, keyof TextDocumentPositionParameters>>
        ) =>
            from(extensionsController.extHostAPI).pipe(
                // Get TextDocumentPositionParams from selection of active viewer
                switchMap(extensionHostAPI =>
                    wrapRemoteObservable(extensionHostAPI.getActiveCodeEditorPosition(), subscriptions).pipe(
                        finallyReleaseProxy()
                    )
                ),
                map(textDocumentPositionParameters => {
                    if (!textDocumentPositionParameters) {
                        return null
                    }

                    return {
                        title,
                        content: '',
                        priority,

                        // This disable directive is necessary because TypeScript is not yet smart
                        // enough to know that (typeof params & typeof extraParams) is P.
                        //
                        // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
                        locationProvider: provideLocations({
                            ...textDocumentPositionParameters,
                            ...extraParameters,
                        } as P).pipe(
                            tap(({ result: locations }) => {
                                if (activation && id === 'references' && locations.length > 0) {
                                    activation.update({ FoundReferences: true })
                                }
                            })
                        ),
                    }
                })
            ),
        [extensionsController, activation, subscriptions]
    )

    // Source for history panel
    const panelSubject = useMemo(() => {
        const parsedHash = parseHash(location.hash)
        return {
            repoID: repoID,
            repoName: repoName,
            commitID: commitID,
            revision: revision,
            filePath: filePath,
            mode: mode,
            position:
                parsedHash.line !== undefined
                    ? { line: parsedHash.line, character: parsedHash.character || 0 }
                    : undefined,
            hash: location.hash,
            history: history,
            location: location,
        }
    }, [location, repoID])

    const panelSubjectChanges = useMemo(() => new ReplaySubject<PanelSubject>(1), [])
    useEffect(() => {
        panelSubjectChanges.next(panelSubject)
    }, [panelSubject])

    useBuiltinPanelViews(
        useMemo(
            () => [
                {
                    id: 'history',
                    provider: panelSubjectChanges.pipe(
                        map(({ repoID, revision, filePath, history, location }) => ({
                            title: 'History',
                            content: '',
                            priority: 150,
                            locationProvider: undefined,
                            reactElement: (
                                <RepoRevisionSidebarCommits
                                    key="commits"
                                    repoID={repoID}
                                    revision={revision}
                                    filePath={filePath}
                                    history={history}
                                    location={location}
                                />
                            ),
                        }))
                    ),
                },
                {
                    id: 'def',
                    provider: createLocationProvider('def', 'Definition', 190, parameters =>
                        from(extensionsController.extHostAPI).pipe(
                            switchMap(extensionHostAPI =>
                                wrapRemoteObservable(extensionHostAPI.getDefinition(parameters))
                            )
                        )
                    ),
                },
                {
                    id: 'references',
                    provider: createLocationProvider<ReferenceParameters>('references', 'References', 180, parameters =>
                        from(extensionsController.extHostAPI).pipe(
                            switchMap(extensionHostAPI =>
                                wrapRemoteObservable(
                                    extensionHostAPI.getReferences(parameters, { includeDeclaration: false })
                                )
                            )
                        )
                    ),
                },
            ],
            [] // Activation props are not stable
        )
    )

    useEffect(() => () => subscriptions.unsubscribe(), [])
}
