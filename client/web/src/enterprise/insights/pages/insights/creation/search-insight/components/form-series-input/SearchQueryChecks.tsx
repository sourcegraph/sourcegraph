import classNames from 'classnames'
import Check from 'mdi-react/CheckIcon'
import Info from 'mdi-react/InfoCircleOutlineIcon'
import RadioboxBlankIcon from 'mdi-react/RadioboxBlankIcon'
import React from 'react'

import styles from './FormSeriesInput.module.scss'

interface SearchQueryChecksProps {
    checks: {
        isValidRegex: boolean
        isValidOperator: boolean
        isValidPatternType: boolean
        isNotRepoOrFile: boolean
        isNotCommitOrDiff: boolean
        isNoRepoFilter: boolean
    }
}

const CheckListItem: React.FunctionComponent<{ valid?: boolean }> = ({ children, valid }) => {
    const StatusIcon: React.FunctionComponent = () =>
        valid ? (
            <Check size={16} className="text-success icon-inline" style={{ top: '3px' }} />
        ) : (
            <RadioboxBlankIcon size={16} className="icon-inline" style={{ top: '3px' }} />
        )
    return (
        <>
            <StatusIcon /> {children}
        </>
    )
}

export const SearchQueryChecks: React.FunctionComponent<SearchQueryChecksProps> = ({ checks }) => (
    <div className={classNames(styles.formSeriesInput)}>
        <ul className={classNames(['mt-4 text-muted', styles.formSeriesInputSeriesCheck])}>
            <li>
                <CheckListItem valid={checks.isValidRegex}>
                    Contains a properly formatted regular expression with at least one capture group
                </CheckListItem>
            </li>
            <li>
                <CheckListItem valid={checks.isValidOperator}>
                    Does not contain boolean operator <code>AND</code> and <code>OR</code> (regular expression boolean
                    operators can still be used)
                </CheckListItem>
            </li>
            <li>
                <CheckListItem valid={checks.isValidPatternType}>
                    Does not contain <code>patternType:literal</code> and <code>patternType:structural</code>
                </CheckListItem>
            </li>
            <li>
                <CheckListItem valid={checks.isNotRepoOrFile}>
                    The capture group matches file contents (not <code>repo</code> or <code>file</code>)
                </CheckListItem>
            </li>
            <li>
                <CheckListItem valid={checks.isNotCommitOrDiff}>
                    Does not contain <code>commit</code> or <code>diff</code> search
                </CheckListItem>
            </li>
            <li>
                <CheckListItem valid={checks.isNoRepoFilter}>
                    Does not contain the <code>repo:</code> filter as it will be added automatically if needed
                </CheckListItem>
            </li>
        </ul>
        <p className="mt-4 text-muted">
            Tip: use <code>archived:no</code> or <code>fork:no</code> to exclude results from archived or forked
            repositories. Explore{' '}
            <a href="https://docs.sourcegraph.com/code_insights/references/common_use_cases">example queries</a> and
            learn more about{' '}
            <a href="https://docs.sourcegraph.com/code_insights/references/common_reasons_code_insights_may_not_match_search_results">
                automatically generated data series
            </a>
            .
        </p>
        <p className="mt-4 text-muted">
            <Info size={16} /> <b>Name</b> and <b>color</b> of each data seris will be generated automatically. Chart
            will display <b>up to 20</b> data series.
        </p>
    </div>
)
