import { EditorView, Tooltip, TooltipView } from '@codemirror/view'
import { concat, from, Observable, of } from 'rxjs'
import { map } from 'rxjs/operators'

import { HoverMerged } from '@sourcegraph/client-api'
import { MarkupKind } from '@sourcegraph/extension-api-classes'
import { ActionItemAction } from '@sourcegraph/shared/src/actions/ActionItem'
import { hasFindImplementationsSupport } from '@sourcegraph/shared/src/codeintel/api'
import { Occurrence } from '@sourcegraph/shared/src/codeintel/scip'
import { BlobViewState, toPrettyBlobURL } from '@sourcegraph/shared/src/util/url'

import { blobPropsFacet } from '..'
import { HovercardView, HoverData } from '../hovercard'
import { rangeToCmSelection } from '../occurrence-utils'
import { DefinitionResult, goToDefinitionAtOccurrence } from '../token-selection/definition'
import { modifierClickDescription } from '../token-selection/modifier-click'

export interface HoverResult {
    markdownContents: string
    hoverMerged?: HoverMerged | null
    isPrecise?: boolean
}
export const emptyHoverResult: HoverResult = { markdownContents: '' }

// Helper to handle the cases for "No definition found" and "You are at the definition".
interface AsyncDefinitionResult extends DefinitionResult {
    asyncHandler: () => Promise<void>
}

// CodeMirror tooltip wrapper around the "code intel" popover.  Implemented as a
// class so that we can detect it with instanceof checks.
export class CodeIntelTooltip implements Tooltip {
    public readonly above = true
    public readonly pos: number
    public readonly end: number
    public readonly create: () => TooltipView
    constructor(
        private readonly view: EditorView,
        private readonly occurrence: Occurrence,
        private readonly hover: HoverResult,
        // eslint-disable-next-line @typescript-eslint/explicit-member-accessibility
        readonly pinned: boolean
    ) {
        const range = rangeToCmSelection(view.state, occurrence.range)
        this.pos = range.from
        this.end = range.to
        this.create = () => {
            // To prevent the "Go to definition" from delaying the loading of
            // the popover, we provide an instant result that doesn't handle the
            // "No definition found" or "You are at the definition" cases. This
            // instant result gets dynamically replaced the actual result once
            // it finishes loading.
            const instantDefinitionResult: AsyncDefinitionResult = {
                locations: [{ uri: '' }],
                handler: () => {},
                asyncHandler: () =>
                    goToDefinitionAtOccurrence(view, occurrence).then(
                        ({ handler }) => handler(occurrence.range.start),
                        () => {}
                    ),
            }
            const definitionResults: Observable<AsyncDefinitionResult> = concat(
                // Show active "Go to definition" button until we have resolved
                // a definition.
                of(instantDefinitionResult),
                // Trigger "Go to definition" to identify if this hover message
                // is already at the definition or if there are no references.
                from(goToDefinitionAtOccurrence(view, occurrence)).pipe(
                    map(result => ({ ...result, asyncHandler: instantDefinitionResult.asyncHandler }))
                )
            )
            const hovercardData: Observable<HoverData> = definitionResults.pipe(
                map(result => this.hovercardData(result))
            )
            return new HovercardView(view, occurrence.range.withIncrementedValues(), pinned, hovercardData)
        }
    }
    private hovercardData(definition: AsyncDefinitionResult): HoverData {
        const { markdownContents } = this.hover
        const blobInfo = this.view.state.facet(blobPropsFacet).blobInfo
        const referencesURL = toPrettyBlobURL({
            ...blobInfo,
            range: this.occurrence.range.withIncrementedValues(),
            viewState: 'references',
        })
        const noDefinitionFound = definition.locations.length === 0
        // TODO(sqs): XXXXXXXXXXXXXXXXXXXX
        const actions: ActionItemAction[] = [
            {
                active: true,
                disabledWhen: noDefinitionFound || definition.atTheDefinition,
                action: {
                    id: 'invokeFunction',
                    title: 'Go to definition',
                    disabledTitle: noDefinitionFound ? 'No definition found' : 'You are at the definition',
                    command: definition.url ? 'open' : 'invokeFunction-new',
                    commandArguments: [definition.url ? definition.url : () => definition.asyncHandler()],
                },
            },
            {
                active: true,
                action: {
                    id: 'findReferences',
                    title: 'Find references',
                    command: 'open',
                    commandArguments: [referencesURL],
                },
            },
        ]
        if (
            this.hover.isPrecise &&
            hasFindImplementationsSupport(this.view.state.facet(blobPropsFacet).blobInfo.mode)
        ) {
            const implementationsURL = toPrettyBlobURL({
                ...blobInfo,
                range: this.occurrence.range.withIncrementedValues(),
                viewState: `implementations_${blobInfo.mode}` as BlobViewState,
            })
            actions.push({
                active: true,
                action: {
                    id: 'findImplementations',
                    title: 'Find implementations',
                    command: 'open',
                    commandArguments: [implementationsURL],
                },
            })
        }
        actions.push({
            active: true,
            action: {
                id: 'goToDefinition',
                title: '?', // special marker for the MDI "Help" icon.
                description: `Go to definition with ${modifierClickDescription}, long-click, or by pressing Enter with the keyboard. Display this popover by pressing Space with the keyboard.`,
                command: '',
            },
        })
        return {
            actionsOrError: actions,
            hoverOrError: {
                range: this.occurrence.range,
                aggregatedBadges: this.hover.hoverMerged?.aggregatedBadges,
                contents: [
                    {
                        value: markdownContents,
                        kind: MarkupKind.Markdown,
                    },
                ],
            },
        }
    }
}
