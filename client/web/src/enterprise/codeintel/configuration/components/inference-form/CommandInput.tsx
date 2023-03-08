import React, { useMemo, useRef } from 'react'

import { defaultKeymap, history } from '@codemirror/commands'
import { StreamLanguage, syntaxHighlighting, HighlightStyle } from '@codemirror/language'
import { shell } from '@codemirror/legacy-modes/mode/shell'
import { EditorState, Extension } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import { tags } from '@lezer/highlight'
import classNames from 'classnames'

import { useCodeMirror, defaultSyntaxHighlighting } from '@sourcegraph/shared/src/components/CodeMirrorEditor'

const shellHighlighting: Extension = [
    syntaxHighlighting(HighlightStyle.define([{ tag: [tags.keyword], class: 'hljs-keyword' }])),
    defaultSyntaxHighlighting,
]

const staticExtensions: Extension = [
    keymap.of(defaultKeymap),
    history(),
    EditorView.theme({
        '&': {
            flex: 1,
            backgroundColor: 'var(--input-bg)',
            borderRadius: 'var(--border-radius)',
            borderColor: 'var(--border-color)',
            marginRight: '0.5rem',
        },
        '&.cm-editor.cm-focused': {
            outline: 'none',
        },
        '.cm-scroller': {
            overflowX: 'hidden',
        },
        '.cm-content': {
            caretColor: 'var(--search-query-text-color)',
            fontFamily: 'var(--code-font-family)',
            fontSize: 'var(--code-font-size)',
        },
        '.cm-content.focus-visible': {
            boxShadow: 'none',
        },
        '.cm-line': {
            padding: '0',
        },
    }),
    StreamLanguage.define(shell),
    shellHighlighting,
]

interface CommandInputProps {
    value: string
    onChange?: (value: string) => void
    readOnly: boolean
    className?: string
}

export const CommandInput: React.FunctionComponent<CommandInputProps> = React.memo(function CodeMirrorComandInput({
    value,
    className,
    readOnly,
    onChange = () => {},
}) {
    const containerRef = useRef<HTMLDivElement | null>(null)
    const editorRef = useRef<EditorView | null>(null)

    useCodeMirror(
        editorRef,
        containerRef,
        value,
        useMemo(
            () => [
                staticExtensions,
                EditorState.readOnly.of(readOnly),
                EditorView.updateListener.of(update => {
                    if (update.docChanged) {
                        onChange(update.state.sliceDoc())
                    }
                }),
            ],
            [onChange, readOnly]
        )
    )

    return <div ref={containerRef} data-editor="codemirror6" className={classNames('form-control', className)} />
})
