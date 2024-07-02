import type { FC, PropsWithChildren } from 'react'

import { mdiClose, mdiRadioboxBlank } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'
import Check from 'mdi-react/CheckIcon'

import { Icon, Code } from '@sourcegraph/wildcard'

import styles from './SearchQueryChecks.module.scss'

interface SearchQueryChecksProps {
    checks?: {
        isValidOperator: true | false | undefined
        isValidPatternType: true | false | undefined
        isNotRepo: true | false | undefined
        isNotCommitOrDiff: true | false | undefined
        isNotRev: true | false | undefined
    }
}

export const SearchQueryChecks: FC<SearchQueryChecksProps> = ({ checks }) => (
    <ul aria-label="Search query validation checks list" className={classNames(styles.checks)}>
        <CheckListItem
            errorMessage="shouldn't contain boolean operators, AND, OR, NOT (regular
                expression boolean operators can still be used)"
            valid={checks?.isValidOperator}
        >
            Does not contain boolean operators <Code>AND</Code>, <Code>OR</Code>, and <Code>NOT</Code> (regular
            expression boolean operators can still be used)
        </CheckListItem>
        <CheckListItem
            errorMessage="shouldn't contain 'keyword', 'literal', or 'structural' patterntype"
            valid={checks?.isValidPatternType}
        >
            Does not contain a <Code>patternType:keyword</Code>, <Code>standard</Code>, <Code>literal</Code>, or{' '}
            <Code>structural</Code>{' '}
        </CheckListItem>
        <CheckListItem errorMessage="shouldn't contain repo filter" valid={checks?.isNotRepo}>
            Does not contain <Code>repo:</Code> filter
        </CheckListItem>
        <CheckListItem errorMessage="shouldn't contain rev filter" valid={checks?.isNotRev}>
            Does not contain <Code>rev:</Code> filter
        </CheckListItem>
        <CheckListItem errorMessage="shouldn't contain commit or diff search" valid={checks?.isNotCommitOrDiff}>
            Does not contain <Code>commit</Code> or <Code>diff</Code> search
        </CheckListItem>
    </ul>
)

interface CheckListItemProps {
    valid: true | false | undefined
    errorMessage: string
}

const CheckListItem: FC<PropsWithChildren<CheckListItemProps>> = props => {
    const { valid, errorMessage, children } = props

    if (valid === true) {
        return (
            <li aria-label="Successful validation check">
                <Icon aria-hidden={true} className={classNames(styles.icon, 'text-success')} as={Check} />
                <span className={classNames(styles.valid, 'text-muted')}>{children}</span>
            </li>
        )
    }

    if (valid === false) {
        return (
            <li role="alert" aria-live="polite">
                <Icon aria-hidden={true} className={classNames(styles.icon, 'text-danger')} svgPath={mdiClose} />
                <span aria-hidden={true} className="text-muted">
                    {children}
                </span>
                <VisuallyHidden>Failed validation check. {errorMessage}</VisuallyHidden>
            </li>
        )
    }

    return (
        <li aria-label="Validation rule">
            <Icon aria-hidden={true} className={classNames(styles.icon, styles.smaller)} svgPath={mdiRadioboxBlank} />{' '}
            <span className="text-muted">{children}</span>
        </li>
    )
}
