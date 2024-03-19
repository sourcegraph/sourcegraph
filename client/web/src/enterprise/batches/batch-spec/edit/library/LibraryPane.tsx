import React, { useState, useCallback } from 'react'

import { mdiChevronDoubleLeft, mdiChevronDoubleRight, mdiOpenInNew } from '@mdi/js'
import { useLocation } from 'react-router-dom'
import { animated, useSpring } from 'react-spring'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Button, useLocalStorage, H3, H4, Icon, Link, Text, VIEWPORT_XL } from '@sourcegraph/wildcard'

import type { Scalars } from '../../../../../graphql-operations'
import { eventLogger } from '../../../../../tracking/eventLogger'
import { createRenderTemplate } from '../../../create/useSearchTemplate'
import { insertNameIntoLibraryItem } from '../../yaml-util'

import combySample from './comby.batch.yaml'
import goImportsSample from './go-imports.batch.yaml'
import helloWorldSample from './hello-world.batch.yaml'
import manyCombySample from './many-comby.batch.yaml'
import minimalSample from './minimal.batch.yaml'
import monorepoDynamicSample from './monorepo-dynamic.batch.yaml'
import { ReplaceSpecModal } from './ReplaceSpecModal'
import regexSample from './sed.batch.yaml'

import styles from './LibraryPane.module.scss'

interface LibraryItem {
    name: string
    code: string
}

const LIBRARY: [LibraryItem, LibraryItem, LibraryItem, LibraryItem, LibraryItem, LibraryItem, LibraryItem] = [
    { name: 'hello world', code: helloWorldSample },
    { name: 'minimal', code: minimalSample },
    { name: 'modify with comby', code: combySample },
    { name: 'update go imports', code: goImportsSample },
    { name: 'apply a regex', code: regexSample },
    { name: 'apply many comby patterns', code: manyCombySample },
    { name: 'monorepo example', code: monorepoDynamicSample },
]

const LIBRARY_PANE_DEFAULT_COLLAPSED = 'batch-changes.ssbc-library-pane-default-collapsed'
// Match to `.collapse-button` class width
const BUTTON_WIDTH = '1.25rem'
// Match to `.list-container` class width
const CONTENT_WIDTH = '14rem'

type LibraryPaneProps =
    | {
          /**
           * The name of the batch change, used for automatically filling in the name for any
           * item selected from the library.
           */
          name: Scalars['String']
          onReplaceItem: (item: string) => void
          isReadOnly?: false
      }
    | {
          name: Scalars['String']
          isReadOnly: true
      }

export const LibraryPane: React.FunctionComponent<React.PropsWithChildren<LibraryPaneProps & TelemetryV2Props>> = ({
    name,
    ...props
}) => {
    // Remember the last collapsed state of the pane
    const [defaultCollapsed, setDefaultCollapsed] = useLocalStorage(LIBRARY_PANE_DEFAULT_COLLAPSED, false)
    // Start with the library collapsed by default if the batch spec is read-only, or if
    // the viewport is sufficiently narrow
    const [collapsed, setCollapsed] = useState(
        defaultCollapsed || ('isReadOnly' in props && props.isReadOnly) || window.innerWidth < VIEWPORT_XL
    )
    const [selectedItem, setSelectedItem] = useState<LibraryItem>()

    const [containerStyle, animateContainer] = useSpring(() => ({
        width: collapsed ? BUTTON_WIDTH : CONTENT_WIDTH,
    }))

    const [headerStyle, animateHeader] = useSpring(() => ({
        opacity: collapsed ? 0 : 1,
        width: collapsed ? '0rem' : CONTENT_WIDTH,
    }))

    const [contentStyle, animateContent] = useSpring(() => ({
        display: collapsed ? 'none' : 'block',
        opacity: collapsed ? 0 : 1,
    }))

    const toggleCollapse = useCallback(
        (collapsed: boolean) => {
            setCollapsed(collapsed)
            setDefaultCollapsed(collapsed)
            animateContainer.start({ width: collapsed ? BUTTON_WIDTH : CONTENT_WIDTH })
            animateContent.start({
                /* eslint-disable callback-return */
                // We need the display: none property change to happen in sequence *after*
                // the opacity property change or else the content will disappear
                // immediately. This use of the API is following the suggestion from
                // https://react-spring.io/hooks/use-spring#this-is-how-you-create-a-script
                to: async next => {
                    await next({ display: 'block', opacity: collapsed ? 0 : 1 })
                    if (collapsed) {
                        await next({ display: 'none' })
                    }
                },
                /* eslint-enable callback-return */
            })
            animateHeader.start({ opacity: collapsed ? 0 : 1, width: collapsed ? '0rem' : CONTENT_WIDTH })
        },
        [animateContainer, animateContent, animateHeader, setDefaultCollapsed]
    )

    const { search: searchQuery } = useLocation()
    const updateTemplateWithQueryAndName = useCallback(
        (template: string): string => {
            if (searchQuery !== '') {
                const parameters = new URLSearchParams(location.search)

                const query = parameters.get('q')
                const patternType = parameters.get('patternType')

                if (query) {
                    const searchQuery = `${query} ${patternType ? `patternType:${patternType}` : ''}`
                    const renderTemplate = createRenderTemplate(searchQuery, template, true)
                    return renderTemplate(name)
                }
            }
            return insertNameIntoLibraryItem(template, name)
        },
        [name, searchQuery]
    )

    const onConfirm = useCallback(() => {
        if (selectedItem && !('isReadOnly' in props && props.isReadOnly)) {
            const codeWithName = updateTemplateWithQueryAndName(selectedItem.code)
            const templateName = selectedItem.name
            eventLogger.log('batch_change_editor:template:loaded', { template: templateName })
            props.telemetryRecorder.recordEvent('batchChange.editor.template', 'load')
            props.onReplaceItem(codeWithName)
            setSelectedItem(undefined)
        }
    }, [selectedItem, props, updateTemplateWithQueryAndName])

    return (
        <div role="region" aria-label="batch spec template library">
            {selectedItem ? (
                <ReplaceSpecModal
                    libraryItemName={selectedItem.name}
                    onCancel={() => setSelectedItem(undefined)}
                    onConfirm={onConfirm}
                />
            ) : null}
            <animated.div style={containerStyle} className="d-none d-md-flex flex-column mr-3">
                <div className={styles.header}>
                    <animated.div style={headerStyle}>
                        <H4 as={H3} className="m-0">
                            Library
                        </H4>
                    </animated.div>
                    <div className={styles.collapseButton}>
                        <Button
                            className="p-0"
                            onClick={() => toggleCollapse(!collapsed)}
                            aria-label={collapsed ? 'Expand library' : 'Collapse library'}
                        >
                            <Icon
                                aria-hidden={true}
                                svgPath={collapsed ? mdiChevronDoubleRight : mdiChevronDoubleLeft}
                            />
                        </Button>
                    </div>
                </div>

                <animated.div style={contentStyle}>
                    <ul className={styles.listContainer} aria-label="batch spec templates">
                        {LIBRARY.map(item => (
                            <li className={styles.libraryItem} key={item.name}>
                                <Button
                                    className={styles.libraryItemButton}
                                    disabled={'isReadOnly' in props && props.isReadOnly}
                                    onClick={() => setSelectedItem(item)}
                                >
                                    {item.name}
                                </Button>
                            </li>
                        ))}
                    </ul>
                    <Text className={styles.lastItem}>
                        <Link
                            target="_blank"
                            rel="noopener noreferrer"
                            to="https://github.com/sourcegraph/batch-change-examples"
                            onClick={() => {
                                eventLogger.log('batch_change_editor:view_more_examples:clicked')
                                props.telemetryRecorder.recordEvent('batchChange.editor.viewMoreExamples', 'click')
                            }}
                        >
                            View more examples <Icon aria-hidden={true} svgPath={mdiOpenInNew} />
                        </Link>
                    </Text>
                </animated.div>
            </animated.div>
        </div>
    )
}
