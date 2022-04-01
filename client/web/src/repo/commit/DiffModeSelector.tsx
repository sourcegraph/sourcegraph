import React from 'react'

import { Button, ButtonGroup } from '@sourcegraph/wildcard'

import { DiffMode } from './RepositoryCommitPage'

interface DiffModeSelectorProps {
    className?: string
    small?: boolean
    onHandleDiffMode: (mode: DiffMode) => void
    diffMode: DiffMode
}

export const DiffModeSelector: React.FunctionComponent<DiffModeSelectorProps> = ({
    className,
    diffMode,
    onHandleDiffMode,
    small,
}) => (
    <div className={className}>
        <ButtonGroup>
            <Button
                onClick={() => onHandleDiffMode('unified')}
                size={small ? 'sm' : undefined}
                variant="secondary"
                outline={diffMode !== 'unified'}
            >
                Unified
            </Button>
            <Button
                onClick={() => onHandleDiffMode('split')}
                size={small ? 'sm' : undefined}
                variant="secondary"
                outline={diffMode !== 'split'}
            >
                Split
            </Button>
            <Button
                onClick={() => onHandleDiffMode('difft')}
                size={small ? 'sm' : undefined}
                variant="secondary"
                outline={diffMode !== 'difft'}
            >
                Difft
            </Button>
        </ButtonGroup>
    </div>
)
