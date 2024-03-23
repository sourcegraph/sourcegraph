import { useMemo } from 'react'

import { EditorState, type Extension } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { useLocation, useNavigate } from 'react-router-dom'

import { SourcegraphURL } from '@sourcegraph/common'
import { CodeMirrorEditor, defaultEditorTheme } from '@sourcegraph/shared/src/components/CodeMirrorEditor'

import { selectableLineNumbers } from '../../repo/blob/codemirror/linenumbers'

const theme = EditorView.theme({
    '.selected-line, .cm-line.selected-line': {
        backgroundColor: 'var(--code-selection-bg)',
    },
    '.cm-scroller': {
        borderRadius: 'var(--border-radius)',
    },
})

export const IngestedFileViewer: React.FunctionComponent<{ contents: string }> = ({ contents }) => {
    const location = useLocation()
    const navigate = useNavigate()

    const lineNumber = useMemo(() => SourcegraphURL.from(location).lineRange.line, [location])

    const extensions: Extension[] = useMemo(
        () => [
            EditorState.readOnly.of(true),
            theme,
            selectableLineNumbers({
                onSelection(range) {
                    navigate(
                        SourcegraphURL.from({ search: location.search, hash: location.hash })
                            .setLineRange(range ? { line: range.line, endLine: range?.endLine } : null)
                            .toString()
                    )
                },
                initialSelection: lineNumber ? { line: lineNumber } : null,
            }),
            defaultEditorTheme,
        ],
        [lineNumber, location, navigate]
    )

    return <CodeMirrorEditor value={contents} extensions={extensions} />
}
