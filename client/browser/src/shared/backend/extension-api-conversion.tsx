import type { TextDocumentPositionParameters } from '@sourcegraph/client-api'
import { type AbsoluteRepoFilePosition, makeRepoGitURI } from '@sourcegraph/shared/src/util/url'

export const toTextDocumentPositionParameters = (
    position: AbsoluteRepoFilePosition
): TextDocumentPositionParameters => ({
    textDocument: {
        uri: makeRepoGitURI({
            repoName: position.repoName,
            filePath: position.filePath,
            commitID: position.commitID,
            revision: position.revision,
        }),
    },
    position: {
        character: position.position.character - 1,
        line: position.position.line - 1,
    },
})
