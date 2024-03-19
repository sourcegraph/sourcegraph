import * as React from 'react'

import { mdiHistory } from '@mdi/js'
import type { Location, NavigateFunction, To } from 'react-router-dom'
import { fromEvent, Subject, Subscription } from 'rxjs'
import { filter } from 'rxjs/operators'

import {
    addLineRangeQueryParameter,
    formatSearchParameters,
    lprToRange,
    toPositionOrRangeQueryParameter,
    toViewStateHash,
} from '@sourcegraph/common'
import { parseQueryAndHash } from '@sourcegraph/shared/src/util/url'
import { Icon, Tooltip } from '@sourcegraph/wildcard'

import { eventLogger } from '../../../tracking/eventLogger'
import { RepoHeaderActionButtonLink, RepoHeaderActionMenuItem } from '../../components/RepoHeaderActions'
import { RepoActionInfo } from '../../RepoActionInfo'
import type { RepoHeaderContext } from '../../RepoHeader'
import type { BlobPanelTabID } from '../panel/BlobPanel'

import styles from './actions.module.scss'

/**
 * A repository header action that toggles the visibility of the history panel.
 */
export class ToggleHistoryPanel extends React.PureComponent<
    {
        isPackage: boolean
        location: Location
        navigate: NavigateFunction
    } & RepoHeaderContext
> {
    private toggles = new Subject<void>()
    private subscriptions = new Subscription()

    /**
     * Reports the current visibility (derived from the location).
     */
    public static isVisible(location: Location): boolean {
        return parseQueryAndHash<BlobPanelTabID>(location.search, location.hash).viewState === 'history'
    }

    /**
     * Returns the location object (that can be passed to H.History's push/replace methods) that sets visibility to
     * the given value.
     */
    private static locationWithVisibility(location: Location, visible: boolean): To {
        const parsedQuery = parseQueryAndHash<BlobPanelTabID>(location.search, location.hash)
        if (visible) {
            parsedQuery.viewState = 'history' // defaults to last-viewed tab, or first tab
        } else {
            delete parsedQuery.viewState
        }
        const lineRangeQueryParameter = toPositionOrRangeQueryParameter({ range: lprToRange(parsedQuery) })

        return {
            search: formatSearchParameters(
                addLineRangeQueryParameter(new URLSearchParams(location.search), lineRangeQueryParameter)
            ),
            hash: toViewStateHash(parsedQuery.viewState),
        }
    }

    public componentDidMount(): void {
        this.subscriptions.add(
            this.toggles.subscribe(() => {
                const visible = ToggleHistoryPanel.isVisible(this.props.location)
                eventLogger.log(visible ? 'HideHistoryPanel' : 'ShowHistoryPanel')
                this.props.navigate(ToggleHistoryPanel.locationWithVisibility(this.props.location, !visible))
            })
        )

        // Toggle when the user presses 'alt+h' or 'opt+h'.
        this.subscriptions.add(
            fromEvent<KeyboardEvent>(window, 'keydown')
                .pipe(filter(event => !this.isDisabled() && event.altKey && event.code === 'KeyH'))
                .subscribe(event => {
                    event.preventDefault()
                    this.toggles.next()
                })
        )
    }

    private isDisabled(): boolean {
        return this.props.isPackage
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        const visible = ToggleHistoryPanel.isVisible(this.props.location)

        const toggleMessage = `${visible ? 'Hide' : 'Show'} history (Alt+H/Opt+H)`
        const disabled = this.isDisabled()
        const message = disabled ? 'Git history is not available when browsing packages' : toggleMessage

        if (this.props.actionType === 'dropdown') {
            return (
                <RepoHeaderActionMenuItem disabled={disabled} file={true} onSelect={this.onClick}>
                    <Icon aria-hidden={true} svgPath={mdiHistory} />
                    <span>{message}</span>
                </RepoHeaderActionMenuItem>
            )
        }
        return (
            <Tooltip content={message}>
                <RepoHeaderActionButtonLink
                    aria-label={message}
                    aria-controls="references-panel"
                    aria-expanded={visible}
                    file={false}
                    onSelect={this.onClick}
                    disabled={disabled}
                    className="d-flex justify-content-center align-items-center"
                >
                    <RepoActionInfo
                        displayName="History"
                        icon={<Icon aria-hidden={true} svgPath={mdiHistory} className={styles.repoActionIcon} />}
                    />
                </RepoHeaderActionButtonLink>
            </Tooltip>
        )
    }

    private onClick = (): void => this.toggles.next()
}
