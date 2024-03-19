import React, { useMemo, useRef } from 'react'

import classNames from 'classnames'
import type * as H from 'history'
import { identity } from 'lodash'
import { combineLatest, from, ReplaySubject } from 'rxjs'
import { map, switchMap } from 'rxjs/operators'
import { useDeepCompareEffectNoCheck } from 'use-deep-compare-effect'

import type { Contributions, Evaluated, ContributableMenu } from '@sourcegraph/client-api'
import type { Context } from '@sourcegraph/template-parser'
import { useObservable } from '@sourcegraph/wildcard'

import { wrapRemoteObservable } from '../api/client/api/common'
import type { ContributionScope } from '../api/extension/api/context/context'
import type { ContributionOptions } from '../api/extension/extensionHostApi'
import { getContributedActionItems } from '../contributions/contributions'
import type { RequiredExtensionsControllerProps } from '../extensions/controller'
import type { PlatformContextProps } from '../platform/context'
import type { TelemetryProps } from '../telemetry/telemetryService'

import { ActionItem, type ActionItemProps } from './ActionItem'

import styles from './ActionsNavItems.module.scss'

export interface ActionNavItemsClassProps {
    /**
     * CSS class name for one action item (`<button>` or `<a>`)
     */
    actionItemClass?: string

    /**
     * Additional CSS class name when the action item is a toogle in its enabled state.
     */
    actionItemPressedClass?: string

    actionItemIconClass?: string

    /**
     * CSS class name for each `<li>` element wrapping the action item.
     */
    listItemClass?: string
}

interface ActionsProps
    extends RequiredExtensionsControllerProps<'executeCommand' | 'extHostAPI'>,
        PlatformContextProps<'settings'>,
        ContributionOptions {
    menu: ContributableMenu
    listClass?: string
    location: H.Location
}

export interface ActionsNavItemsProps
    extends ActionsProps,
        ActionNavItemsClassProps,
        TelemetryProps,
        Pick<ActionItemProps, 'showLoadingSpinnerDuringExecution' | 'actionItemStyleProps'> {
    /**
     * If true, it renders a `<ul className="nav">...</ul>` around the items. If there are no items, it renders `null`.
     *
     * If falsey (the default behavior), it emits a fragment of just the `<li>`s.
     */
    wrapInList?: boolean
    /**
     * Only applied if `wrapInList` is `true`
     */

    listClass?: string

    /**
     * Transform function called when latest contributions from extensions are received.
     * Likely temporary: quick fix to dedup panel actions from various code intel extensions.
     */
    transformContributions?: (contributions: Evaluated<Contributions>) => Evaluated<Contributions>
}

/**
 * Renders the actions as a fragment of <li class="nav-item"> elements, for use in with <ul
 * class="nav"> or <ul class="navbar-nav">.
 */
export const ActionsNavItems: React.FunctionComponent<React.PropsWithChildren<ActionsNavItemsProps>> = props => {
    const { scope, extraContext, extensionsController, menu, wrapInList, transformContributions = identity } = props

    const scopeChanges = useMemo(() => new ReplaySubject<ContributionScope | undefined>(1), [])
    useDeepCompareEffectNoCheck(() => {
        scopeChanges.next(scope)
    }, [scope])

    const extraContextChanges = useMemo(() => new ReplaySubject<Context<unknown> | undefined>(1), [])
    useDeepCompareEffectNoCheck(() => {
        extraContextChanges.next(extraContext)
    }, [extraContext])

    const contributions = useObservable(
        useMemo(
            () =>
                combineLatest([scopeChanges, extraContextChanges, from(extensionsController.extHostAPI)]).pipe(
                    switchMap(([scope, extraContext, extensionHostAPI]) =>
                        wrapRemoteObservable(extensionHostAPI.getContributions({ scope, extraContext }))
                    ),
                    map(transformContributions)
                ),
            [scopeChanges, extraContextChanges, extensionsController, transformContributions]
        )
    )

    const actionItems = useRef<JSX.Element[] | null>(null)

    if (!contributions) {
        // Show last known list while loading, or empty if nothing has been loaded yet
        return <>{actionItems.current}</>
    }

    actionItems.current = getContributedActionItems(contributions, menu).map(item => (
        <React.Fragment key={item.action.id}>
            {' '}
            <li className={props.listItemClass}>
                <ActionItem
                    key={item.action.id}
                    {...item}
                    {...props}
                    variant="actionItem"
                    iconClassName={props.actionItemIconClass}
                    className={classNames(styles.actionItem, props.actionItemClass)}
                    pressedClassName={props.actionItemPressedClass}
                    actionItemStyleProps={props.actionItemStyleProps}
                />
            </li>
        </React.Fragment>
    ))

    if (wrapInList) {
        return actionItems.current.length > 0 ? <ul className={props.listClass}>{actionItems.current}</ul> : null
    }
    return <>{actionItems.current}</>
}
