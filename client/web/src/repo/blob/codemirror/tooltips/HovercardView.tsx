import type { ApolloClient } from '@apollo/client'
import { type EditorView, repositionTooltips, type TooltipView, type ViewUpdate } from '@codemirror/view'
import classNames from 'classnames'
import { createRoot, type Root } from 'react-dom/client'
import { combineLatest, type Observable, Subject, type Subscription } from 'rxjs'
import { distinctUntilChanged, startWith, map } from 'rxjs/operators'

import { type LineOrPositionOrRange, isErrorLike } from '@sourcegraph/common'

import { WebHoverOverlay, type WebHoverOverlayProps } from '../../../../components/WebHoverOverlay'
import type { BlobPropsFacet } from '../../CodeMirrorBlob'
import type { TooltipViewOptions } from '../codeintel/api'
import { pinConfig, pinnedLocation } from '../codeintel/pin'
import { blobPropsFacet } from '../index'
import { CodeMirrorContainer } from '../react-interop'
import { zeroToOneBasedPosition } from '../utils'

type Unwrap<T> = T extends Observable<infer U> ? U : never

// WebHoverOverlay expects to be passed the overlay position. Since CodeMirror
// positions the element we always use the same value.
const NOOP_OVERLAY_POSITION = { left: 0, bottom: 0 }

// WebHoverOverlay is shared witht the browser extension and expects to be passed
// an extension controller. This is a dummy value that is never used.
const NOOP_EXTENSION_CONTROLLER = { executeCommand: () => Promise.resolve(undefined) }

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
    private nextPinned = new Subject<LineOrPositionOrRange | null>()
    private subscription: Subscription

    constructor(
        private readonly view: EditorView,
        private readonly tokenRange: TooltipViewOptions['token'],
        hovercardData: TooltipViewOptions['hovercardData'],
        private readonly client: ApolloClient<any>
    ) {
        this.dom = document.createElement('div')
        this.dom.className = 'sg-code-intel-hovercard'

        this.subscription = combineLatest([
            this.nextContainer,
            hovercardData,
            this.nextProps.pipe(startWith(view.state.facet(blobPropsFacet))),
            this.nextPinned.pipe(
                startWith(view.state.facet(pinnedLocation)),
                map(pin => pin?.line === tokenRange.start.line && pin?.character === tokenRange.start.character),
                distinctUntilChanged()
            ),
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
        this.nextPinned.next(update.state.facet(pinnedLocation))
    }

    public destroy(): void {
        this.subscription.unsubscribe()
        window.requestAnimationFrame(() => this.root?.unmount())
    }

    private render(
        root: Root,
        { hoverOrError, actionsOrError }: Unwrap<TooltipViewOptions['hovercardData']>,
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
            <CodeMirrorContainer
                navigate={props.navigate}
                graphQLClient={this.client}
                onRender={() => repositionTooltips(this.view)}
            >
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
                        telemetryService={props.telemetryService}
                        extensionsController={NOOP_EXTENSION_CONTROLLER}
                        // Hover props
                        actionsOrError={actionsOrError}
                        hoverOrError={hoverOrError}
                        // CodeMirror handles the positioning but a
                        // non-nullable value must be passed for the
                        // hovercard to render
                        overlayPosition={NOOP_OVERLAY_POSITION}
                        hoveredToken={hoveredToken}
                        pinOptions={{
                            showCloseButton: pinned,
                            onCloseButtonClick: () => {
                                const { line, character } = hoveredToken
                                this.view.state.facet(pinConfig).onUnpin?.({ line, character })
                            },
                            onCopyLinkButtonClick: () => {
                                const { line, character } = hoveredToken
                                this.view.state.facet(pinConfig).onPin?.({ line, character })
                            },
                        }}
                        hoverOverlayContainerClassName="position-relative"
                    />
                </div>
            </CodeMirrorContainer>
        )
    }
}
