import classNames from 'classnames'
import CalculatorIcon from 'mdi-react/CalculatorIcon'
import * as React from 'react'
import { pluralize } from '../../../../../../shared/src/util/strings'
import { Progress } from '../../../stream'
import { StreamingProgressProps } from './StreamingProgress'

const abbreviateNumber = (number: number): string => {
    if (number < 1e3) {
        return number.toString()
    }
    if (number >= 1e3 && number < 1e6) {
        return (number / 1e3).toFixed(1) + 'k'
    }
    if (number >= 1e6 && number < 1e9) {
        return (number / 1e6).toFixed(1) + 'm'
    }
    return (number / 1e9).toFixed(1) + 'b'
}

const limitHit = (progress: Progress): boolean => progress.skipped.some(skipped => skipped.reason.indexOf('-limit') > 0)

export const StreamingProgressCount: React.FunctionComponent<
    Pick<StreamingProgressProps, 'progress' | 'state'> & { className?: string }
> = ({ progress, state, className = '' }) => (
    <div
        className={classNames(className, 'streaming-progress__count d-flex align-items-center', {
            'streaming-progress__count--in-progress': state === 'loading',
        })}
    >
        <CalculatorIcon className="mr-2 icon-inline" />
        {abbreviateNumber(progress.matchCount)}
        {limitHit(progress) ? '+' : ''} {pluralize('result', progress.matchCount)} in{' '}
        {(progress.durationMs / 1000).toFixed(2)}s
        {progress.repositoriesCount !== undefined && (
            <>
                {' '}
                from {abbreviateNumber(progress.repositoriesCount)}{' '}
                {pluralize('repository', progress.repositoriesCount, 'repositories')}
            </>
        )}
    </div>
)
