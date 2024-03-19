import * as React from 'react'

import classNames from 'classnames'
import * as jsonc from 'jsonc-parser'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { Subject, Subscription } from 'rxjs'
import { distinctUntilChanged, distinctUntilKeyChanged, map, startWith } from 'rxjs/operators'

import { MonacoEditor } from '@sourcegraph/shared/src/components/MonacoEditor'
import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryService } from '@sourcegraph/shared/src/telemetry/telemetryService'

import jsonSchemaMetaSchema from '../../../../schema/json-schema-draft-07.schema.json'
import settingsSchema from '../../../../schema/settings.schema.json'

import styles from './MonacoSettingsEditor.module.scss'

/**
 * Minimal shape of a JSON Schema. These values are treated as opaque, so more specific types are
 * not needed.
 */
interface JSONSchema {
    $id: string
}

export interface Props {
    id?: string
    className?: string
    value: string | undefined
    isLightTheme: boolean
    onChange?: (newValue: string) => void
    readOnly?: boolean | undefined
    height?: number

    language?: string

    /**
     * JSON Schema of the document.
     */
    jsonSchema?: JSONSchema

    monacoRef?: (monacoValue: typeof monaco | null) => void
    /**
     * Called when the user presses the key binding for "save" (Ctrl+S/Cmd+S).
     */
    onDidSave?: () => void
}

interface State {}

/**
 * A JSON settings editor using the Monaco editor.
 */
export class MonacoSettingsEditor extends React.PureComponent<Props, State> {
    public state: State = {}

    private monaco: typeof monaco | null = null
    private editor: monaco.editor.ICodeEditor | null = null

    private componentUpdates = new Subject<Props>()
    private subscriptions = new Subscription()
    private disposables: monaco.IDisposable[] = []

    public componentDidMount(): void {
        const componentUpdates = this.componentUpdates.pipe(startWith(this.props))

        this.subscriptions.add(
            componentUpdates
                .pipe(
                    map(props => props.readOnly),
                    distinctUntilChanged()
                )
                .subscribe(readOnly => {
                    if (this.editor) {
                        this.editor.updateOptions({ readOnly })
                    }
                })
        )

        this.subscriptions.add(
            componentUpdates.pipe(distinctUntilKeyChanged('jsonSchema')).subscribe(props => {
                if (this.monaco) {
                    setDiagnosticsOptions(this.monaco, props.jsonSchema)
                }
            })
        )
    }

    public componentDidUpdate(): void {
        this.componentUpdates.next(this.props)
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
        for (const disposable of this.disposables) {
            disposable.dispose()
        }
        this.monaco = null
        this.editor = null
    }

    public render(): JSX.Element | null {
        return (
            <MonacoEditor
                id={this.props.id}
                className={classNames(styles.monacoSettingsEditor, this.props.className)}
                language={this.props.language || 'json'}
                height={this.props.height || 400}
                isLightTheme={this.props.isLightTheme}
                value={this.props.value}
                editorWillMount={this.editorWillMount}
                options={{
                    lineNumbers: 'off',
                    automaticLayout: true,
                    minimap: { enabled: false },
                    formatOnType: true,
                    formatOnPaste: true,
                    renderIndentGuides: false,
                    glyphMargin: false,
                    folding: false,
                    renderLineHighlight: 'none',
                    scrollBeyondLastLine: false,
                    quickSuggestions: true,
                    quickSuggestionsDelay: 200,
                    wordBasedSuggestions: false,
                    readOnly: this.props.readOnly,
                    wordWrap: 'on',
                }}
            />
        )
    }

    private editorWillMount = (editor: typeof monaco): void => {
        this.monaco = editor
        if (editor) {
            this.onDidEditorMount()
        }
    }

    private onDidEditorMount(): void {
        const monaco = this.monaco!

        if (this.props.monacoRef) {
            this.props.monacoRef(monaco)
            this.subscriptions.add(() => {
                if (this.props.monacoRef) {
                    this.props.monacoRef(null)
                }
            })
        }

        this.disposables.push(registerRedactedHover(monaco))

        setDiagnosticsOptions(monaco, this.props.jsonSchema)

        // Only listen to 1 event each to avoid receiving events from other Monaco editors on the
        // same page (if there are multiple).
        const editorDisposable = monaco.editor.onDidCreateEditor(editor => {
            this.onDidCreateEditor(editor)
            editorDisposable.dispose()
        })
        this.disposables.push(editorDisposable)
        const modelDisposable = monaco.editor.onDidCreateModel(model => {
            this.onDidCreateModel(model)
            modelDisposable.dispose()
        })
        this.disposables.push(modelDisposable)
    }

    private onDidCreateEditor(editor: monaco.editor.ICodeEditor): void {
        this.editor = editor

        // Necessary to wrap in setTimeout or else _standaloneKeyBindingService won't be ready and the editor will
        // refuse to add the command because it's missing the keybinding service.
        setTimeout(() => {
            if (MonacoSettingsEditor.isStandaloneCodeEditor(editor)) {
                editor.addCommand(
                    monaco.KeyMod.CtrlCmd | monaco.KeyCode.KEY_S,
                    () => {
                        if (this.props.onDidSave) {
                            this.props.onDidSave()
                        }
                    },
                    ''
                )
            }
        })
    }

    private onDidCreateModel(model: monaco.editor.IModel): void {
        this.disposables.push(
            model.onDidChangeContent(() => {
                if (this.props.onChange) {
                    this.props.onChange(model.getValue())
                }
            })
        )
    }

    public static isStandaloneCodeEditor(
        editor: monaco.editor.ICodeEditor
    ): editor is monaco.editor.IStandaloneCodeEditor {
        const maybeStandaloneEditor: Partial<monaco.editor.IStandaloneCodeEditor> = editor
        return (
            editor.getEditorType() === monaco.editor.EditorType.ICodeEditor &&
            typeof maybeStandaloneEditor.addAction === 'function'
        )
    }

    public static addEditorAction(
        inputEditor: monaco.editor.IStandaloneCodeEditor,
        model: monaco.editor.IModel,
        label: string,
        id: string,
        run: ConfigInsertionFunction,
        telemetryService: TelemetryService,
        telemetryRecorder: TelemetryRecorder
    ): void {
        inputEditor.addAction({
            label,
            id,
            run: editor => {
                telemetryService.log('SiteConfigurationActionExecuted')
                telemetryRecorder.recordEvent('settingsEditor.action', 'execute')
                editor.focus()
                editor.pushUndoStop()
                const { edits, selectText, cursorOffset } = run(editor.getValue())
                const monacoEdits = toMonacoEdits(model, edits)
                let selection: monaco.Selection | undefined
                if (typeof selectText === 'string') {
                    const afterText = jsonc.applyEdits(editor.getValue(), edits)
                    let offset = afterText.slice(edits[0].offset).indexOf(selectText)
                    if (offset !== -1) {
                        offset += edits[0].offset
                        if (typeof cursorOffset === 'number') {
                            selection = monaco.Selection.fromPositions(
                                getPositionAt(afterText, offset + cursorOffset),
                                getPositionAt(afterText, offset + cursorOffset)
                            )
                        } else {
                            selection = monaco.Selection.fromPositions(
                                getPositionAt(afterText, offset),
                                getPositionAt(afterText, offset + selectText.length)
                            )
                        }
                    }
                }
                if (!selection) {
                    // TODO: This is buggy. See
                    // https://github.com/sourcegraph/sourcegraph/issues/2756.
                    selection = monaco.Selection.fromPositions(
                        {
                            lineNumber: monacoEdits[0].range.startLineNumber,
                            column: monacoEdits[0].range.startColumn,
                        },
                        {
                            lineNumber: monacoEdits.at(-1)!.range.endLineNumber,
                            column: monacoEdits.at(-1)!.range.endColumn,
                        }
                    )
                }
                editor.executeEdits(id, monacoEdits, [selection])
                editor.revealPositionInCenter(selection.getStartPosition())
            },
        })
    }
}

function setDiagnosticsOptions(editor: typeof monaco, jsonSchema: JSONSchema | undefined): void {
    const schema = { ...settingsSchema, properties: { ...settingsSchema.properties } }
    editor.languages.json.jsonDefaults.setDiagnosticsOptions({
        validate: true,
        allowComments: true,
        schemas: [
            {
                uri: 'file:///root',
                schema: jsonSchema,
                fileMatch: ['*'],
            },

            // Include these schemas because they are referenced by other schemas.
            {
                uri: 'http://json-schema.org/draft-07/schema',
                schema: jsonSchemaMetaSchema as JSONSchema,
            },
            {
                uri: 'settings.schema.json#',
                schema,
            },
            {
                uri: 'settings.schema.json',
                schema,
            },
        ],
    })
}

function toMonacoEdits(
    model: monaco.editor.IModel,
    edits: jsonc.Edit[]
): monaco.editor.IIdentifiedSingleEditOperation[] {
    return edits.map((edit, index) => ({
        identifier: { major: model.getVersionId(), minor: index },
        range: monaco.Range.fromPositions(
            model.getPositionAt(edit.offset),
            model.getPositionAt(edit.offset + edit.length)
        ),
        forceMoveMarkers: true,
        text: edit.content,
    }))
}

function registerRedactedHover(editor: typeof monaco): monaco.IDisposable {
    return editor.languages.registerHoverProvider('json', {
        provideHover(model, position, token): monaco.languages.ProviderResult<monaco.languages.Hover> {
            if (model.getWordAtPosition(position)?.word === 'REDACTED') {
                return {
                    contents: [
                        {
                            value: "**This field is redacted.** To update, replace with a new value. Otherwise, don't modify this field.",
                        },
                    ],
                }
            }
            return { contents: [] }
        },
    })
}

/**
 * A helper function that modifies site configuration to configure specific
 * common things, such as syncing GitHub repositories.
 */
export type ConfigInsertionFunction = (configJSON: string) => {
    /** The edits to make to the input configuration to insert the new configuration. */
    edits: jsonc.Edit[]

    /** Select text in inserted JSON. */
    selectText?: string | number

    /**
     * If set, the selection is an empty selection that begins at the left-hand match of selectText plus this
     * offset. For example, if selectText is "foo" and cursorOffset is 2, then the final selection will be a cursor
     * "|" positioned as "fo|o".
     */
    cursorOffset?: number
}

function getPositionAt(text: string, offset: number): monaco.IPosition {
    const lines = text.split('\n')
    let position = 0
    for (const [index, line] of lines.entries()) {
        if (offset < position + line.length + 1) {
            return new monaco.Position(index + 1, offset - position + 1)
        }
        position += line.length + 1
    }
    throw new Error(`offset ${offset} out of bounds in text of length ${text.length}`)
}

declare global {
    interface Window {
        MonacoEnvironment?: monaco.Environment | undefined
    }
}

// Manually configure the MonacoEnvironment for the Monaco editor.
if (!window.MonacoEnvironment) {
    window.MonacoEnvironment = {
        /* eslint-disable @typescript-eslint/ban-ts-comment */

        // NOTE: We used to use `getWorkerUrl` to load the worker, but that stopped working when
        // we switched to using bazel to build the Monaco editor. If we wanted to use `getWorkerUrl`
        // we would have to add the two workers below as entry points to an esbuild build.
        // We cannot use the same build that builds the webapp because the output file names
        // currently contain the content hash of the file, which would make it impossible to know
        // the file name of the worker. Specifying a different file name pattern for the workers only
        // doesn't seem to be possible with esbuild.
        // We could add a separate bazel build target for the workers only, but using `getWorker` with
        // the workerPlugin allows us to have a single build target for the webapp, and ensures that
        // bazel and non-bazel builds work the same way.

        // Returning Promise<Worker> works, but it appears that the type definition does not
        // reflect that. We have to dynamically import these modules because they are converted
        // to "inline workers" via the workerPlugin for esbuild.
        // Ignoring the error is necessary for the bazel build to work.
        // @ts-ignore
        async getWorker(_moduleId: string, label: string): Promise<Worker> {
            if (label === 'json') {
                // There are no type definitions for this file. The workerPlugin for esbuild converts
                // this file to a module which exports a function that returns a worker.
                // See build-config/src/esbuild/workerPlugin.ts.
                // Ignoring the error is necessary for the bazel build to work.
                // @ts-ignore
                return (await import('monaco-editor/esm/vs/language/json/json.worker')).default()
            }
            // There are no type definitions for this file. The workerPlugin for esbuild converts
            // this file to a module which exports a function that returns a worker.
            // See build-config/src/esbuild/workerPlugin.ts.
            // Ignoring the error is necessary for the bazel build to work.
            // @ts-ignore
            return (await import('monaco-editor/esm/vs/editor/editor.worker')).default()
        },

        /* eslint-enable @typescript-eslint/ban-ts-comment */
    }
}
