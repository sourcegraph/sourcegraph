import { Selection } from '@sourcegraph/extension-api-types'
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

/**
 * Describes a code editor that has been added to the {@link ViewerService}.
 */
export interface CodeEditor extends ViewerId, CodeEditorData {}

/**
 * Describes a directory viewer that has been added to the {@link ViewerService}.
 */
export interface DirectoryViewer extends ViewerId, DirectoryViewerData {}
