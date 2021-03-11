import {
    ChangesetCheckState,
    ChangesetReviewState,
    ChangesetSpecOperation,
    ChangesetState,
} from '../../graphql-operations'
import { HoveredToken } from '@sourcegraph/codeintellify'
import {
    RepoSpec,
    RevisionSpec,
    FileSpec,
    ResolvedRevisionSpec,
    UIPositionSpec,
    ModeSpec,
} from '../../../../shared/src/util/url'
import { getModeFromPath } from '../../../../shared/src/languages'

export function isValidChangesetReviewState(input: string): input is ChangesetReviewState {
    return Object.values<string>(ChangesetReviewState).includes(input)
}

export function isValidChangesetCheckState(input: string): input is ChangesetCheckState {
    return Object.values<string>(ChangesetCheckState).includes(input)
}

export function getLSPTextDocumentPositionParameters(
    hoveredToken: HoveredToken & RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec
): RepoSpec & RevisionSpec & ResolvedRevisionSpec & FileSpec & UIPositionSpec & ModeSpec {
    return {
        repoName: hoveredToken.repoName,
        revision: hoveredToken.revision,
        filePath: hoveredToken.filePath,
        commitID: hoveredToken.commitID,
        position: hoveredToken,
        mode: getModeFromPath(hoveredToken.filePath || ''),
    }
}

export function isValidChangesetSpecOperation(input: string): input is ChangesetSpecOperation {
    return Object.values<string>(ChangesetSpecOperation).includes(input)
}

export function isValidChangesetState(input: string): input is ChangesetState {
    return Object.values<string>(ChangesetState).includes(input)
}
