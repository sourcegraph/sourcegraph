import { concat, from, of } from 'rxjs'
import { map, switchMap } from 'rxjs/operators'

import { HoverMerged, TextDocumentPositionParameters } from '@sourcegraph/client-api'
import { ActionsProvider, HoverProvider } from '@sourcegraph/codeintellify'
import { ActionItemAction } from '@sourcegraph/shared/src/actions/ActionItem'
import { getOrCreateCodeIntelAPI } from '@sourcegraph/shared/src/codeintel/api'
import { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import {
    toPrettyBlobURL,
    FileSpec,
    RepoSpec,
    ResolvedRevisionSpec,
    RevisionSpec,
    toURIWithPath,
} from '@sourcegraph/shared/src/util/url'

type ConcreteHoverProvider = HoverProvider<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec, HoverMerged>

type GetHover = (
    platformContext: PlatformContext,
    params: Parameters<ConcreteHoverProvider>[0]
) => ReturnType<ConcreteHoverProvider>

export const getHover: GetHover = (platformContext, params) =>
    concat(
        of({ isLoading: true, result: null }),
        from(getOrCreateCodeIntelAPI(platformContext)).pipe(
            switchMap(api =>
                api.getHover({
                    textDocument: { uri: toURIWithPath(params) },
                    position: { line: params.line - 1, character: params.character - 1 },
                })
            ),
            map(result => ({
                isLoading: false,
                result,
            }))
        )
    )

type ConcreteActionsProvider = ActionsProvider<
    RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec,
    ActionItemAction
>

type GetActions = (
    platformContext: PlatformContext,
    params: Parameters<ConcreteActionsProvider>[0]
) => ReturnType<ConcreteActionsProvider>

export const getActions: GetActions = (platformContext, params) => {
    const position: TextDocumentPositionParameters = {
        textDocument: { uri: toURIWithPath(params) },
        position: { line: params.line - 1, character: params.character - 1 },
    }

    return from(getOrCreateCodeIntelAPI(platformContext)).pipe(
        switchMap(api => api.getDefinition(position)),
        map(definitions => {
            const actions: ActionItemAction[] = [
                {
                    active: true,
                    action: {
                        id: 'invokeFunction',
                        title: 'Go to definition',
                    },
                },
                {
                    active: true,
                    action: {
                        id: 'findReferences',
                        title: 'Find references',
                        command: 'open',
                        commandArguments: [
                            toPrettyBlobURL({
                                ...params,
                                viewState: 'references',
                            }),
                        ],
                    },
                },
            ]
            return actions
        })
    )
}
