import React, { useCallback } from 'react'

import CodeTagsIcon from 'mdi-react/CodeTagsIcon'
import FunctionIcon from 'mdi-react/FunctionIcon'
import LanguageMarkdownOutlineIcon from 'mdi-react/LanguageMarkdownOutlineIcon'
import LaptopIcon from 'mdi-react/LaptopIcon'
import MagnifyIcon from 'mdi-react/MagnifyIcon'

import { Button, Icon, Tooltip } from '@sourcegraph/wildcard'

import { BlockInput } from '..'
import { useExperimentalFeatures } from '../../stores'

import { EMPTY_FILE_BLOCK_INPUT, EMPTY_SYMBOL_BLOCK_INPUT } from './useCommandPaletteOptions'

import styles from './NotebookAddBlockButtons.module.scss'

interface NotebookAddBlockButtonsProps {
    onAddBlock: (blockIndex: number, blockInput: BlockInput) => void
    index: number
}

export const NotebookAddBlockButtons: React.FunctionComponent<
    React.PropsWithChildren<NotebookAddBlockButtonsProps>
> = ({ index, onAddBlock }) => {
    const showComputeComponent = useExperimentalFeatures(features => features.showComputeComponent)
    const addBlock = useCallback((blockInput: BlockInput) => onAddBlock(index, blockInput), [index, onAddBlock])
    return (
        <>
            <Tooltip content="Add Markdown text">
                <Button
                    className={styles.addBlockButton}
                    onClick={() => addBlock({ type: 'md', input: { text: '', initialFocusInput: true } })}
                    data-testid="add-md-block"
                >
                    <Icon aria-hidden={true} as={LanguageMarkdownOutlineIcon} size="sm" />
                </Button>
            </Tooltip>
            <Tooltip content="Add a Sourcegraph query">
                <Button
                    className={styles.addBlockButton}
                    onClick={() => addBlock({ type: 'query', input: { query: '', initialFocusInput: true } })}
                    data-testid="add-query-block"
                >
                    <Icon aria-hidden={true} as={MagnifyIcon} size="sm" />
                </Button>
            </Tooltip>
            <Tooltip content="Add code from a file">
                <Button
                    className={styles.addBlockButton}
                    onClick={() => addBlock({ type: 'file', input: EMPTY_FILE_BLOCK_INPUT })}
                    data-testid="add-file-block"
                >
                    <Icon aria-hidden={true} as={CodeTagsIcon} size="sm" />
                </Button>
            </Tooltip>
            <Tooltip content="Add a symbol">
                <Button
                    className={styles.addBlockButton}
                    onClick={() => addBlock({ type: 'symbol', input: EMPTY_SYMBOL_BLOCK_INPUT })}
                    data-testid="add-symbol-block"
                >
                    <Icon aria-hidden={true} as={FunctionIcon} size="sm" />
                </Button>
            </Tooltip>
            {showComputeComponent && (
                <Tooltip content="Add compute block">
                    <Button
                        className={styles.addBlockButton}
                        onClick={() => addBlock({ type: 'compute', input: '' })}
                        data-testid="add-compute-block"
                    >
                        {/* // TODO: Fix icon */}
                        <Icon aria-hidden={true} as={LaptopIcon} size="sm" />
                    </Button>
                </Tooltip>
            )}
        </>
    )
}
