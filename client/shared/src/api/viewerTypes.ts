import { Selection } from '@sourcegraph/extension-api-types'
import { TextDocument } from 'sourcegraph'
import { ExtensionCodeEditor } from './extension/api/codeEditor'
import { ExtensionDirectoryViewer } from './extension/api/directoryViewer'

export type ExtensionViewer = ExtensionCodeEditor | ExtensionDirectoryViewer

export type Viewer = CodeEditor | DirectoryViewer
export type ViewerData = CodeEditorData | DirectoryViewerData

/**
 * ViewerId exposes the unique ID of a viewer.
 */
export interface ViewerId {
    /** The unique ID of the viewer. */
    readonly viewerId: string
}

export interface BaseViewerData {
    readonly isActive: boolean
}

export interface DirectoryViewerData extends BaseViewerData {
    readonly type: 'DirectoryViewer'
    /** The URI of the directory that this viewer is displaying. */
    readonly resource: string
}

/**
 * Describes a code viewer to be created.
 */
export interface CodeEditorData extends BaseViewerData {
    readonly type: 'CodeEditor'

    /** The URI of the model that this viewer is displaying. */
    readonly resource: string

    readonly selections: Selection[]
}

export interface CodeEditor extends ViewerId, CodeEditorData {}

/**
 * Describes a directory viewer that has been added to the {@link ViewerService}.
 */
export interface DirectoryViewer extends ViewerId, DirectoryViewerData {}

export type ViewerWithPartialModel = CodeEditorWithPartialModel | DirectoryViewer // Directories don't have a model

/**
 * A code editor with a partial model.
 */
export interface CodeEditorWithPartialModel extends CodeEditor {
    model: PartialModel
}

/**
 * A text model is a text document and associated metadata.
 *
 * How does this relate to editors? A model is the file, an editor is the
 * window that the file is shown in. Things like the content and language are properties of the
 * model; things like decorations and the selection ranges are properties of the editor.
 */
export interface TextModel extends Pick<TextDocument, 'uri' | 'languageId' | 'text'> {}

/**
 * A partial {@link TextModel}, containing only the fields that are
 * guaranteed to never be updated.
 */
export interface PartialModel extends Pick<TextModel, 'languageId'> {}

export type ViewerUpdate = ViewerId & { action: 'removal' | 'addition'; type: 'DirectoryViewer' | 'CodeEditor' }
