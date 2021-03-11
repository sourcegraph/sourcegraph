import OpenInNewIcon from 'mdi-react/OpenInNewIcon'
import classnames from 'classnames'
import React, { useState, useCallback, useMemo } from 'react'
import { Link } from '../../../../../shared/src/components/Link'
import { buildSearchURLQuery } from '../../../../../shared/src/util/url'
import { useInputValidation, deriveInputClassName } from '../../../../../shared/src/util/useInputValidation'
import { SearchPatternType } from '../../../graphql-operations'
import { scanSearchQuery } from '../../../../../shared/src/search/query/scanner'
import { resolveFilter, validateFilter, FilterType } from '../../../../../shared/src/search/query/filters'

interface TriggerAreaProps {
    query: string
    onQueryChange: (query: string) => void
    triggerCompleted: boolean
    setTriggerCompleted: (complete: boolean) => void
    startExpanded: boolean
}

const isDiffOrCommit = (value: string): boolean => value === 'diff' || value === 'commit'

export const FormTriggerArea: React.FunctionComponent<TriggerAreaProps> = ({
    query,
    onQueryChange,
    triggerCompleted,
    setTriggerCompleted,
    startExpanded,
}) => {
    const [showQueryForm, setShowQueryForm] = useState(startExpanded)
    const toggleQueryForm: React.FormEventHandler = useCallback(event => {
        event.preventDefault()
        setShowQueryForm(show => !show)
    }, [])

    const [queryState, nextQueryFieldChange, queryInputReference, overrideState] = useInputValidation(
        useMemo(
            () => ({
                initialValue: query,
                synchronousValidators: [
                    (value: string) => {
                        const tokens = scanSearchQuery(value)
                        if (tokens.type === 'success') {
                            const filters = tokens.term.filter(token => token.type === 'filter')
                            const hasTypeDiffOrCommitFilter = filters.some(
                                filter =>
                                    filter.type === 'filter' &&
                                    resolveFilter(filter.field.value)?.type === FilterType.type &&
                                    filter.value &&
                                    isDiffOrCommit(filter.value.value)
                            )
                            if (!hasTypeDiffOrCommitFilter) {
                                return 'Code monitors require queries to specify either `type:commit` or `type:diff`.'
                            }
                            const hasPattern = tokens.term.some(term => term.type === 'pattern')
                            const hasPatternTypeFilter = filters.some(
                                filter =>
                                    filter.type === 'filter' &&
                                    resolveFilter(filter.field.value)?.type === FilterType.patterntype &&
                                    filter.value &&
                                    validateFilter(filter.field.value, filter.value)
                            )
                            if (!hasPatternTypeFilter && hasPattern) {
                                return 'Code monitors require queries to specify a `patternType:` of literal or regexp.'
                            }
                            return undefined
                        }
                        return 'Failed to parse query'
                    },
                ],
            }),
            [query]
        )
    )

    const completeForm: React.FormEventHandler = useCallback(
        event => {
            event.preventDefault()
            setShowQueryForm(false)
            setTriggerCompleted(true)
            onQueryChange(queryState.value)
        },
        [setTriggerCompleted, setShowQueryForm, onQueryChange, queryState]
    )

    const cancelForm: React.FormEventHandler = useCallback(
        event => {
            event.preventDefault()
            setShowQueryForm(false)
            overrideState({ value: query })
        },
        [setShowQueryForm, overrideState, query]
    )

    return (
        <>
            <h3>Trigger</h3>
            {!showQueryForm && !triggerCompleted && (
                <button
                    type="button"
                    onClick={toggleQueryForm}
                    className="code-monitor-form__card--button card p-3 w-100 test-trigger-button text-left"
                >
                    <div className="code-monitor-form__card-link btn-link font-weight-bold p-0">
                        When there are new search results
                    </div>
                    <span className="text-muted">
                        This trigger will fire when new search results are found for a given search query.
                    </span>
                </button>
            )}
            {showQueryForm && (
                <div className="code-monitor-form__card card p-3">
                    <div className="font-weight-bold">When there are new search results</div>
                    <span className="text-muted">
                        This trigger will fire when new search results are found for a given search query.
                    </span>
                    <span className="mt-4">Search query</span>
                    <div>
                        <div className="trigger-area__query-input mb-4">
                            <div className="d-flex flex-column flex-grow-1">
                                <input
                                    type="text"
                                    className={classnames(
                                        'trigger-area__query-input-field form-control my-2 test-trigger-input',
                                        deriveInputClassName(queryState)
                                    )}
                                    onChange={nextQueryFieldChange}
                                    value={queryState.value}
                                    required={true}
                                    autoFocus={true}
                                    ref={queryInputReference}
                                    spellCheck={false}
                                    data-testid="trigger-query-edit"
                                />
                                {queryState.kind === 'INVALID' && (
                                    <small className="trigger-area__query-input-error-message invalid-feedback test-trigger-error">
                                        {queryState.reason}
                                    </small>
                                )}
                                {(queryState.kind === 'NOT_VALIDATED' ||
                                    queryState.kind === 'VALID' ||
                                    queryState.kind === 'LOADING') && (
                                    <small className="text-muted mt-1">
                                        Code monitors only support <code className="bg-code">type:diff</code> and{' '}
                                        <code className="bg-code">type:commit</code> search queries.
                                    </small>
                                )}
                            </div>
                            <div className="trigger-area__query-input-preview-link p-2 my-2">
                                <Link
                                    to={`/search?${buildSearchURLQuery(
                                        queryState.value,
                                        SearchPatternType.literal,
                                        false
                                    )}`}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="trigger-area__query-input-preview-link-text test-preview-link"
                                >
                                    Preview results{' '}
                                    <OpenInNewIcon className="trigger-area__query-input-preview-link-icon ml-1 icon-inline" />
                                </Link>
                            </div>
                        </div>
                    </div>
                    <div>
                        <button
                            className="btn btn-secondary mr-1 test-submit-trigger"
                            onClick={completeForm}
                            type="submit"
                            disabled={queryState.kind !== 'VALID'}
                        >
                            Continue
                        </button>
                        <button type="button" className="btn btn-outline-secondary" onClick={cancelForm}>
                            Cancel
                        </button>
                    </div>
                </div>
            )}
            {!showQueryForm && triggerCompleted && (
                <div className="code-monitor-form__card--button card p-3" onClick={toggleQueryForm}>
                    <div className="d-flex justify-content-between align-items-center">
                        <div>
                            <div className="font-weight-bold">When there are new search results</div>
                            <code
                                className="trigger-area__query-label text-break text-muted test-existing-query"
                                data-testid="trigger-query-existing"
                            >
                                {query}
                            </code>
                        </div>
                        <div>
                            <button type="button" className="btn btn-link p-0 text-left test-edit-trigger">
                                Edit
                            </button>
                        </div>
                    </div>
                </div>
            )}
            <small className="text-muted">
                {' '}
                What other events would you like to monitor?{' '}
                <a href="mailto:feedback@sourcegraph.com" target="_blank" rel="noopener">
                    Share feedback.
                </a>
            </small>
        </>
    )
}
