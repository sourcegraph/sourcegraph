import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { mdiCheck, mdiHelpCircle, mdiOpenInNew, mdiRadioboxBlank } from '@mdi/js'
import { VisuallyHidden } from '@reach/visually-hidden'
import classNames from 'classnames'

import { LazyQueryInputFormControl } from '@sourcegraph/branded'
import type { QueryState } from '@sourcegraph/shared/src/search'
import { FilterType, resolveFilter, validateFilter } from '@sourcegraph/shared/src/search/query/filters'
import { scanSearchQuery } from '@sourcegraph/shared/src/search/query/scanner'
import { useSettingsCascade } from '@sourcegraph/shared/src/settings/settings'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import { Button, Card, Checkbox, Code, H3, Icon, Link, Tooltip } from '@sourcegraph/wildcard'

import type { SearchPatternType } from '../../../graphql-operations'
import { defaultPatternTypeFromSettings } from '../../../util/settings'

import styles from './FormTriggerArea.module.scss'

interface TriggerAreaProps {
    query: string
    onQueryChange: (query: string) => void
    triggerCompleted: boolean
    setTriggerCompleted: (complete: boolean) => void
    startExpanded: boolean
    cardClassName?: string
    cardBtnClassName?: string
    cardLinkClassName?: string
    isSourcegraphDotCom: boolean
}

const isDiffOrCommit = (value: string): boolean => value === 'diff' || value === 'commit'

// Code monitors don't support pattern type "structural"
const isValidPatternType = (value: string): boolean =>
    value === 'keyword' || value === 'standard' || value === 'literal' || value === 'regexp'

const ValidQueryChecklistItem: React.FunctionComponent<
    React.PropsWithChildren<{
        checked: boolean
        hint?: string
        className?: string
        dataTestid?: string
    }>
> = ({ checked, children, hint, className, dataTestid }) => (
    <Checkbox
        wrapperClassName={classNames('d-flex align-items-center text-muted pl-0', className)}
        className="sr-only"
        disabled={true}
        checked={checked}
        data-testid={dataTestid}
        id={dataTestid || 'ValidQueryCheckListInput'}
        label={
            <div className="d-flex align-items-center mb-1">
                {checked ? (
                    <Icon
                        className={classNames('text-success', styles.checklistCheckbox)}
                        aria-hidden={true}
                        svgPath={mdiCheck}
                    />
                ) : (
                    <Icon
                        className={classNames(styles.checklistCheckbox, styles.checklistCheckboxUnchecked)}
                        aria-hidden={true}
                        svgPath={mdiRadioboxBlank}
                    />
                )}

                <small className={checked ? styles.checklistChildrenFaded : ''}>{children}</small>

                {hint && (
                    <>
                        <span className="sr-only"> {hint}</span>

                        <Tooltip content={hint} placement="bottom">
                            <span className="d-inline-flex">
                                <Icon
                                    className={classNames(styles.checklistHint, checked && styles.checklistHintFaded)}
                                    aria-hidden={true}
                                    svgPath={mdiHelpCircle}
                                />
                            </span>
                        </Tooltip>
                    </>
                )}
            </div>
        }
    />
)

export const FormTriggerArea: React.FunctionComponent<React.PropsWithChildren<TriggerAreaProps>> = ({
    query,
    onQueryChange,
    triggerCompleted,
    setTriggerCompleted,
    startExpanded,
    cardClassName,
    cardBtnClassName,
    cardLinkClassName,
    isSourcegraphDotCom,
}) => {
    const [expanded, setExpanded] = useState(startExpanded)

    // Focus card when collapsing
    const collapsedCard = useRef<HTMLButtonElement>(null)
    const closeCard = useCallback((): void => {
        setExpanded(false)

        // Use timeout to wait for render to complete after calling setExpanded
        // so that collapsedCard is rendered and can be focused.
        setTimeout(() => {
            collapsedCard.current?.focus()
        }, 0)
    }, [])

    const [isValidQuery, setIsValidQuery] = useState(false)
    const [hasTypeDiffOrCommitFilter, setHasTypeDiffOrCommitFilter] = useState(false)
    const [hasRepoFilter, setHasRepoFilter] = useState(false)
    const [hasPatternTypeFilter, setHasPatternTypeFilter] = useState(false)
    const [hasValidPatternTypeFilter, setHasValidPatternTypeFilter] = useState(true)
    const isTriggerQueryComplete = useMemo(
        () =>
            isValidQuery &&
            hasTypeDiffOrCommitFilter &&
            (!isSourcegraphDotCom || hasRepoFilter) &&
            hasValidPatternTypeFilter,
        [hasRepoFilter, hasTypeDiffOrCommitFilter, hasValidPatternTypeFilter, isValidQuery, isSourcegraphDotCom]
    )

    const [queryState, setQueryState] = useState<QueryState>({ query: query || '' })

    useEffect(() => {
        const value = queryState.query
        const tokens = scanSearchQuery(value)

        const isValidQuery = !!value && tokens.type === 'success'
        setIsValidQuery(isValidQuery)

        let hasTypeDiffOrCommitFilter = false
        let hasRepoFilter = false
        let hasPatternTypeFilter = false
        let hasValidPatternTypeFilter = true

        if (tokens.type === 'success') {
            const filters = tokens.term.filter(token => token.type === 'filter')
            hasTypeDiffOrCommitFilter = filters.some(
                filter =>
                    filter.type === 'filter' &&
                    resolveFilter(filter.field.value)?.type === FilterType.type &&
                    filter.value &&
                    isDiffOrCommit(filter.value.value)
            )

            hasRepoFilter = filters.some(
                filter =>
                    filter.type === 'filter' &&
                    resolveFilter(filter.field.value)?.type === FilterType.repo &&
                    filter.value
            )

            hasPatternTypeFilter = filters.some(
                filter =>
                    filter.type === 'filter' &&
                    resolveFilter(filter.field.value)?.type === FilterType.patterntype &&
                    filter.value &&
                    validateFilter(filter.field.value, filter.value)
            )

            // No explicit patternType filter means we use the default pattern type
            hasValidPatternTypeFilter =
                !hasPatternTypeFilter ||
                filters.some(
                    filter =>
                        filter.type === 'filter' &&
                        resolveFilter(filter.field.value)?.type === FilterType.patterntype &&
                        filter.value &&
                        isValidPatternType(filter.value.value)
                )
        }

        setHasTypeDiffOrCommitFilter(hasTypeDiffOrCommitFilter)
        setHasRepoFilter(hasRepoFilter)
        setHasPatternTypeFilter(hasPatternTypeFilter)
        setHasValidPatternTypeFilter(hasValidPatternTypeFilter)
    }, [queryState.query])

    const defaultPatternType: SearchPatternType = defaultPatternTypeFromSettings(useSettingsCascade())

    const completeForm: React.FormEventHandler = useCallback(
        event => {
            event.preventDefault()
            closeCard()
            setTriggerCompleted(true)
            onQueryChange(`${queryState.query}${hasPatternTypeFilter ? '' : ` patternType:${defaultPatternType}`}`)
        },
        [closeCard, setTriggerCompleted, onQueryChange, queryState.query, hasPatternTypeFilter, defaultPatternType]
    )

    const cancelForm: React.FormEventHandler = useCallback(
        event => {
            event.preventDefault()
            closeCard()
            setQueryState({ query })
        },
        [closeCard, query]
    )

    const derivedInputClassName = useMemo(() => {
        if (!queryState.query) {
            return ''
        }
        if (isTriggerQueryComplete) {
            return 'is-valid'
        }
        return 'is-invalid'
    }, [isTriggerQueryComplete, queryState.query])

    return (
        <>
            <H3>Trigger</H3>
            {expanded && (
                <Card className={classNames(cardClassName, 'p-3')}>
                    <div className="font-weight-bold">When there are new search results</div>
                    <span className="text-muted">
                        This trigger will fire when new search results are found for a given search query.
                    </span>
                    <span className="mt-4">Search query</span>
                    <div>
                        <div className="my-2">
                            <div
                                className={classNames(`test-${derivedInputClassName}`)}
                                data-testid="trigger-query-edit"
                            >
                                <LazyQueryInputFormControl
                                    className="test-trigger-input"
                                    patternType={defaultPatternType}
                                    isSourcegraphDotCom={isSourcegraphDotCom}
                                    caseSensitive={false}
                                    queryState={queryState}
                                    onChange={setQueryState}
                                    preventNewLine={true}
                                    autoFocus={true}
                                />
                            </div>
                            <Link
                                to={`/search?${buildSearchURLQuery(queryState.query, defaultPatternType, false)}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="test-preview-link d-flex align-items-center flex-gap-1 my-1"
                            >
                                Preview results{' '}
                                <Icon aria-label="Open in new window" className="ml-1" svgPath={mdiOpenInNew} />
                            </Link>
                        </div>

                        <ul className={classNames(styles.checklist, 'mb-4')}>
                            <li>
                                <ValidQueryChecklistItem
                                    checked={hasValidPatternTypeFilter}
                                    hint={`Code monitors support keyword, standard, literal and regex search. The default is ${defaultPatternType}`}
                                    dataTestid="patterntype-checkbox"
                                >
                                    Is <Code>patternType:keyword</Code>, <Code>standard</Code>, <Code>literal</Code> or{' '}
                                    <Code>regexp</Code>
                                </ValidQueryChecklistItem>
                            </li>
                            <li>
                                <ValidQueryChecklistItem
                                    checked={hasTypeDiffOrCommitFilter}
                                    hint="type:diff targets code present in new commits, while type:commit targets commit messages"
                                    dataTestid="type-checkbox"
                                >
                                    Contains a <Code>type:diff</Code> or <Code>type:commit</Code> filter
                                </ValidQueryChecklistItem>
                            </li>
                            {/* Enforce repo filter on sourcegraph.com because otherwise it's too easy to generate a lot of load */}
                            {isSourcegraphDotCom && (
                                <li>
                                    <ValidQueryChecklistItem
                                        checked={hasRepoFilter}
                                        hint="The repo: filter is required to narrow down your search."
                                        dataTestid="repo-checkbox"
                                    >
                                        Contains a <Code>repo:</Code> filter
                                    </ValidQueryChecklistItem>
                                </li>
                            )}
                            <li>
                                <ValidQueryChecklistItem checked={isValidQuery} dataTestid="valid-checkbox">
                                    Is a valid search query
                                </ValidQueryChecklistItem>
                            </li>
                        </ul>
                    </div>
                    <div>
                        <Button
                            data-testid="submit-trigger"
                            className="mr-1 test-submit-trigger"
                            onClick={completeForm}
                            type="submit"
                            disabled={!isTriggerQueryComplete}
                            variant="secondary"
                        >
                            Continue
                        </Button>
                        <Button onClick={cancelForm} outline={true} variant="secondary">
                            Cancel
                        </Button>
                    </div>
                </Card>
            )}
            {!expanded && (
                <Card
                    data-testid="trigger-button"
                    as={Button}
                    className={classNames('test-trigger-button', cardBtnClassName)}
                    onClick={() => setExpanded(true)}
                    ref={collapsedCard}
                >
                    <div className="d-flex flex-wrap justify-content-between align-items-center w-100">
                        <div>
                            <VisuallyHidden>Edit trigger: </VisuallyHidden>
                            <div
                                className={classNames(
                                    'font-weight-bold',
                                    triggerCompleted
                                        ? styles.triggerBtnText
                                        : classNames(cardLinkClassName, styles.triggerLabel)
                                )}
                            >
                                When there are new search results
                            </div>
                            {triggerCompleted ? (
                                <Code
                                    className={classNames('text-break text-muted', styles.queryLabel)}
                                    data-testid="trigger-query-existing"
                                >
                                    {query}
                                </Code>
                            ) : (
                                <span className="text-muted">
                                    This trigger will fire when new search results are found for a given search query.
                                </span>
                            )}
                        </div>
                        {triggerCompleted && (
                            <Button variant="link" as="div" className="p-0">
                                Edit
                            </Button>
                        )}
                    </div>
                </Card>
            )}
            <small className="text-muted">
                {' '}
                What other events would you like to monitor?{' '}
                <Link to="mailto:feedback@sourcegraph.com" target="_blank" rel="noopener">
                    Share feedback.
                </Link>
            </small>
        </>
    )
}
