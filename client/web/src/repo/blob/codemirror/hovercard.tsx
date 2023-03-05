import { EditorView, repositionTooltips, TooltipView, ViewUpdate } from '@codemirror/view'
import classNames from 'classnames'
import { createRoot, Root } from 'react-dom/client'
import { combineLatest, Observable, Subject, Subscription } from 'rxjs'
import { startWith } from 'rxjs/operators'

import { addLineRangeQueryParameter, isErrorLike, toPositionOrRangeQueryParameter } from '@sourcegraph/common'
import { UIRangeSpec } from '@sourcegraph/shared/src/util/url'

import { WebHoverOverlay, WebHoverOverlayProps } from '../../../components/WebHoverOverlay'
import { updateBrowserHistoryIfChanged, BlobPropsFacet } from '../CodeMirrorBlob'

import { CodeMirrorContainer } from './react-interop'
import { zeroToOneBasedPosition } from './utils'

import { blobPropsFacet } from './index'

type UIRange = UIRangeSpec['range']

/**
 * Hover information received from a hover source.
 */
export type HoverData = Pick<WebHoverOverlayProps, 'hoverOrError' | 'actionsOrError'>

// WebHoverOverlay expects to be passed the overlay position. Since CodeMirror
// positions the element we always use the same value.
const dummyOverlayPosition = { left: 0, bottom: 0 }

/**
 * This class is responsible for rendering a WebHoverOverlay component as a
 * CodeMirror tooltip. When constructed the instance subscribes to the hovercard
 * data source and the component props, and updates the component as it receives
 * changes.
 */
export class HovercardView implements TooltipView {
    public dom: HTMLElement
    private root: Root | null = null
    private nextContainer = new Subject<HTMLElement>()
    private nextProps = new Subject<BlobPropsFacet>()
    private props: BlobPropsFacet | null = null
    public overlap = true
    private nextPinned = new Subject<boolean>()
    private subscription: Subscription

    constructor(
        private readonly view: EditorView,
        private readonly tokenRange: UIRange,
        pinned: boolean,
        hovercardData: Observable<HoverData>
    ) {
        this.dom = document.createElement('div')

        this.subscription = combineLatest([
            this.nextContainer,
            hovercardData,
            this.nextProps.pipe(startWith(view.state.facet(blobPropsFacet))),
            this.nextPinned.pipe(startWith(pinned)),
        ]).subscribe(([container, hovercardData, props, pinned]) => {
            if (!this.root) {
                this.root = createRoot(container)
            }
            this.render(this.root, hovercardData, props, pinned)
        })
    }

    public mount(): void {
        this.nextContainer.next(this.dom)
    }

    public update(update: ViewUpdate): void {
        // Update component when props change
        const props = update.state.facet(blobPropsFacet)
        if (this.props !== props) {
            this.props = props
            this.nextProps.next(props)
        }
    }

    public destroy(): void {
        this.subscription.unsubscribe()
        this.root?.unmount()
    }

    private render(
        root: Root,
        { hoverOrError, actionsOrError }: HoverData,
        props: BlobPropsFacet,
        pinned: boolean
    ): void {
        const hoverContext = {
            commitID: props.blobInfo.commitID,
            filePath: props.blobInfo.filePath,
            repoName: props.blobInfo.repoName,
            revision: props.blobInfo.revision,
        }

        let hoveredToken: Exclude<WebHoverOverlayProps['hoveredToken'], undefined> = {
            ...hoverContext,
            ...this.tokenRange.start,
        }

        if (hoverOrError && hoverOrError !== 'loading' && !isErrorLike(hoverOrError) && hoverOrError.range) {
            hoveredToken = {
                ...hoveredToken,
                ...zeroToOneBasedPosition(hoverOrError.range.start),
            }
        }

        root.render(
            <CodeMirrorContainer navigate={props.navigate} onRender={() => repositionTooltips(this.view)}>
                <div
                    className={classNames({
                        'cm-code-intel-hovercard': true,
                        'cm-code-intel-hovercard-pinned': pinned,
                    })}
                >
                    <WebHoverOverlay
                        // Blob props
                        location={props.location}
                        onHoverShown={props.onHoverShown}
                        settingsCascade={props.settingsCascade}
                        telemetryService={props.telemetryService}
                        // Hover props
                        actionsOrError={actionsOrError}
                        hoverOrError={hoverOrError}
                        // CodeMirror handles the positioning but a
                        // non-nullable value must be passed for the
                        // hovercard to render
                        overlayPosition={dummyOverlayPosition}
                        hoveredToken={hoveredToken}
                        pinOptions={{
                            showCloseButton: pinned,
                            onCloseButtonClick: () => {
                                const parameters = new URLSearchParams(props.location.search)
                                parameters.delete('popover')

                                updateBrowserHistoryIfChanged(props.navigate, props.location, parameters)
                                this.nextPinned.next(false)
                            },
                            onCopyLinkButtonClick: async () => {
                                const { line, character } = hoveredToken
                                const position = { line, character }

                                const search = new URLSearchParams(location.search)
                                search.set('popover', 'pinned')

                                updateBrowserHistoryIfChanged(
                                    props.navigate,
                                    props.location,
                                    // It may seem strange to set start and end to the same value, but that what's the old blob view is doing as well
                                    addLineRangeQueryParameter(
                                        search,
                                        toPositionOrRangeQueryParameter({
                                            position,
                                            range: { start: position, end: position },
                                        })
                                    )
                                )
                                await navigator.clipboard.writeText(window.location.href)

                                this.nextPinned.next(true)
                            },
                        }}
                        hoverOverlayContainerClassName="position-relative"
                    />
                </div>
            </CodeMirrorContainer>
        )
    }
}
