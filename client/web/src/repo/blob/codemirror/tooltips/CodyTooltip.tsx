import { type EditorState, type Extension, StateField, type Transaction } from '@codemirror/state'
import { type Tooltip, showTooltip } from '@codemirror/view'
import ReactDOM from 'react-dom/client'

import type { BillingCategory, BillingProduct } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryRecorder } from '@sourcegraph/telemetry'

import type { CodeMirrorEditor } from '../../../../cody/components/CodeMirrorEditor'
import { CodyRecipesWidget } from '../../../../cody/widgets/CodyRecipesWidget'

// Only show the cody widget if the transaction
// - changes the selection
// - the selection is not empty (i.e. multiple characters are selected)
// - the selection was user initiated (user event 'select')
// - the selection was not triggers via searches (user event 'select.search')
//
// Unfortunately checking for 'select.search' is necessary because
// - we cannot check for text selection via the keyboard
//   (user event 'select.keyboard' doesn't exist, 'select.pointer' does)
// - isUserEvent('select') will be true for any user event that _starts with_ 'select',
//   i.e. also 'select.search', which we want to ignore.
//
// This also means we need to explicitly add any other 'select.x' event that should
// be ignored.
function shouldShowCodyWidget(transaction: Transaction): boolean {
    return (
        !!transaction.selection &&
        !transaction.selection.main.empty &&
        transaction.isUserEvent('select') &&
        !transaction.isUserEvent('select.search')
    )
}

/**
 * Add a extension to CodeMirror extensions to display the Cody widget
 * when some code is selected in the editor.
 */
export function codyWidgetExtension(
    telemetryRecorder: TelemetryRecorder<BillingCategory, BillingProduct>,
    editor?: CodeMirrorEditor
): Extension {
    return StateField.define<Tooltip | null>({
        create() {
            return null
        },

        update(value, transaction) {
            if (transaction.newSelection.main.empty) {
                // Don't show
                return null
            }

            if (transaction.selection) {
                if (shouldShowCodyWidget(transaction)) {
                    const tooltip = createCodyWidget(transaction.state, telemetryRecorder, editor)
                    // Only create a new tooltip if the position changes, to avoid flickering
                    return tooltip?.pos !== value?.pos ? tooltip : value
                }
                return null
            }

            return value
        },
        provide: field => showTooltip.compute([field], state => state.field(field)),
    })
}

function createCodyWidget(
    state: EditorState,
    telemetryRecorder: TelemetryRecorder<BillingCategory, BillingProduct>,
    editor?: CodeMirrorEditor
): Tooltip {
    const { selection } = state

    // Find a position that is always the left most position of the selection bounding box
    const lineFrom = state.doc.lineAt(selection.main.from)
    const lineTo = state.doc.lineAt(selection.main.to)
    const isMultiline = lineFrom.number !== lineTo.number
    // When a multiline selection is made, the tooltip should be anchored to the start of the last
    // line. This is to avoid the tooltip from jumping around too much. Otherwise, anchor the
    // tooltip to the start of the selection
    const pos = isMultiline ? lineTo.from : selection.main.from

    return {
        pos,
        above: false,
        strictSide: true,
        arrow: false,
        create: () => {
            const dom = document.createElement('div')
            dom.style.background = 'transparent'
            ReactDOM.createRoot(dom).render(<CodyRecipesWidget editor={editor} telemetryRecorder={telemetryRecorder} />)
            return { dom }
        },
    }
}
