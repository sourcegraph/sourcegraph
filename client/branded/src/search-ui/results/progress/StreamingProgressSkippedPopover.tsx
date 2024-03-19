import React, { useCallback, useState, type FC, useEffect } from 'react'

import { mdiAlertCircle, mdiChevronDown, mdiChevronLeft, mdiInformationOutline, mdiMagnify } from '@mdi/js'
import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { pluralize, renderMarkdown } from '@sourcegraph/common'
import { useMutation, gql, useQuery } from '@sourcegraph/http-client'
import type { Skipped, Progress } from '@sourcegraph/shared/src/search/stream'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Collapse,
    CollapseHeader,
    CollapsePanel,
    Icon,
    Checkbox,
    H4,
    Text,
    H3,
    Markdown,
    Form,
    ProductStatusBadge,
    Alert,
    ErrorAlert,
    LoadingSpinner,
} from '@sourcegraph/wildcard'

import { SyntaxHighlightedSearchQuery } from '../../components'

import { sortBySeverity, limitHit } from './utils'

import styles from './StreamingProgressSkippedPopover.module.scss'

const SkippedMessage: React.FunctionComponent<React.PropsWithChildren<{ skipped: Skipped; startOpen: boolean }>> = ({
    skipped,
    startOpen,
}) => {
    // Reactstrap is preventing default behavior on all non-DropdownItem elements inside a Dropdown,
    // so we need to stop propagation to allow normal behavior (e.g. enter and space to activate buttons)
    // See Reactstrap bug: https://github.com/reactstrap/reactstrap/issues/2099
    const onKeyDown = useCallback((event: React.KeyboardEvent<HTMLButtonElement>): void => {
        if (event.key === ' ' || event.key === 'Enter') {
            event.stopPropagation()
        }
    }, [])

    return (
        <div
            className={classNames(
                styles.streamingSkippedItem,
                skipped.severity !== 'info' && styles.streamingSkippedItemWarn
            )}
        >
            <Collapse openByDefault={startOpen}>
                {({ isOpen }) => (
                    <>
                        <CollapseHeader
                            className={classNames(styles.button, 'p-2 w-100 bg-transparent border-0')}
                            onKeyDown={onKeyDown}
                            disabled={!skipped.message}
                            as={Button}
                            outline={true}
                            variant={skipped.severity !== 'info' ? 'danger' : 'primary'}
                        >
                            <H4 className="d-flex align-items-center mb-0 w-100">
                                <Icon
                                    aria-label={skipped.severity === 'info' ? 'Information' : 'Alert'}
                                    className={classNames(styles.icon, 'flex-shrink-0')}
                                    svgPath={skipped.severity === 'info' ? mdiInformationOutline : mdiAlertCircle}
                                />

                                <span className="flex-grow-1 text-left">{skipped.title}</span>

                                {skipped.message && (
                                    <Icon
                                        aria-hidden={true}
                                        className={classNames('flex-shrink-0', styles.chevron)}
                                        svgPath={isOpen ? mdiChevronDown : mdiChevronLeft}
                                    />
                                )}
                            </H4>
                        </CollapseHeader>

                        {skipped.message && (
                            <CollapsePanel>
                                <Markdown
                                    className={classNames(styles.message, styles.markdown, 'text-left py-1')}
                                    dangerousInnerHTML={renderMarkdown(skipped.message)}
                                />
                            </CollapsePanel>
                        )}
                    </>
                )}
            </Collapse>
            <div className={classNames(styles.bottomBorderSpacer, 'mt-2')} />
        </div>
    )
}

interface StreamingProgressSkippedPopoverProps extends TelemetryProps {
    query: string
    progress: Progress
    isSearchJobsEnabled?: boolean
    onSearchAgain: (additionalFilters: string[]) => void
}

export const StreamingProgressSkippedPopover: FC<StreamingProgressSkippedPopoverProps> = props => {
    const { query, progress, isSearchJobsEnabled, onSearchAgain, telemetryService } = props

    const [selectedSuggestedSearches, setSelectedSuggestedSearches] = useState(new Set<string>())

    const submitHandler = useCallback(
        (event: React.FormEvent) => {
            onSearchAgain([...selectedSuggestedSearches])
            event.preventDefault()
        },
        [selectedSuggestedSearches, onSearchAgain]
    )
    const checkboxHandler = useCallback((event: React.FormEvent<HTMLInputElement>) => {
        const itemToToggle = event.currentTarget.value
        const checked = event.currentTarget.checked
        setSelectedSuggestedSearches(selected => {
            const newSelected = new Set(selected)
            if (checked) {
                newSelected.add(itemToToggle)
            } else {
                newSelected.delete(itemToToggle)
            }
            return newSelected
        })
    }, [])

    const sortedSkippedItems = sortBySeverity(progress.skipped)

    return (
        <>
            <Text className={classNames('mx-3 mt-3', isSearchJobsEnabled && 'mb-0')}>
                Found {limitHit(progress) ? 'more than ' : ''}
                {progress.matchCount} {pluralize('result', progress.matchCount)}
                {progress.repositoriesCount !== undefined
                    ? ` from ${progress.repositoriesCount} ${pluralize(
                          'repository',
                          progress.repositoriesCount,
                          'repositories'
                      )}`
                    : ''}
                .
            </Text>

            {isSearchJobsEnabled ? (
                <SlimSkippedReasons items={sortedSkippedItems} />
            ) : (
                <SkippedReasons items={sortedSkippedItems} />
            )}

            {sortedSkippedItems.some(skipped => skipped.suggested) && (
                <SkippedItemsSearch
                    slim={isSearchJobsEnabled ?? false}
                    items={sortedSkippedItems}
                    disabled={selectedSuggestedSearches.size === 0}
                    onSearchSettingsChange={checkboxHandler}
                    onSubmit={submitHandler}
                />
            )}

            {isSearchJobsEnabled && (
                <>
                    <hr className="mx-3 mt-3" />
                    <ExhaustiveSearchMessage query={query} telemetryService={telemetryService} />
                </>
            )}
        </>
    )
}

interface SkippedReasonsProps {
    items: Skipped[]
}

const SkippedReasons: FC<SkippedReasonsProps> = props => {
    const { items } = props

    return (
        <>
            {items.length > 0 && <H3 className="mx-3">Some results skipped:</H3>}
            {items.map((skipped, index) => (
                <SkippedMessage
                    key={skipped.reason}
                    skipped={skipped}
                    // Start with first item open, but only if it's not info severity or if there's only one item
                    startOpen={index === 0 && (skipped.severity !== 'info' || items.length === 1)}
                />
            ))}
        </>
    )
}

const SlimSkippedReasons: FC<SkippedReasonsProps> = props => {
    const { items } = props

    if (items.length === 0) {
        return null
    }

    return (
        <div className={styles.streamingSkippedItem}>
            <Collapse openByDefault={items.length === 1}>
                {({ isOpen }) => (
                    <>
                        <CollapseHeader
                            as={Button}
                            outline={true}
                            variant="primary"
                            className={classNames(styles.button, 'p-2 w-100 bg-transparent border-0')}
                        >
                            <H4 className="d-flex align-items-center mb-0 w-100">
                                <Icon
                                    aria-label="Information"
                                    svgPath={mdiInformationOutline}
                                    className={classNames(styles.icon, 'flex-shrink-0')}
                                />

                                <span className="flex-grow-1 text-left">Why was the limit reached?</span>

                                <Icon
                                    aria-hidden={true}
                                    className={classNames('flex-shrink-0', styles.chevron)}
                                    svgPath={isOpen ? mdiChevronDown : mdiChevronLeft}
                                />
                            </H4>
                        </CollapseHeader>

                        <CollapsePanel className={styles.nestedCollapseReasons}>
                            {items.map((skipped, index) => (
                                <SkippedMessage
                                    key={skipped.reason}
                                    skipped={skipped}
                                    // Start with first item open, but only if it's not info severity or if there's only one item
                                    startOpen={index === 0 && (skipped.severity !== 'info' || items.length === 1)}
                                />
                            ))}
                        </CollapsePanel>

                        {!isOpen && <div className={classNames(styles.bottomBorderSpacer, 'mt-1')} />}
                    </>
                )}
            </Collapse>
        </div>
    )
}

interface SkippedItemsSearchProps {
    slim: boolean
    items: Skipped[]
    disabled: boolean
    onSearchSettingsChange: (event: React.FormEvent<HTMLInputElement>) => void
    onSubmit: (event: React.FormEvent) => void
}

const SkippedItemsSearch: FC<SkippedItemsSearchProps> = props => {
    const { slim, items, disabled, onSearchSettingsChange, onSubmit } = props

    return (
        <Form className={classNames('px-3', { 'pb-3': !slim })} onSubmit={onSubmit} data-testid="popover-form">
            <div className="mb-2 mt-3">Search again:</div>
            <div className="form-check">
                {items.map(
                    (skipped, index) =>
                        skipped.suggested && (
                            <Checkbox
                                key={skipped.suggested.queryExpression}
                                value={skipped.suggested.queryExpression}
                                onChange={onSearchSettingsChange}
                                data-testid={`streaming-progress-skipped-suggest-check-${index}`}
                                id={`streaming-progress-skipped-suggest-check-${index}`}
                                wrapperClassName="mb-1 d-block"
                                label={
                                    <>
                                        {skipped.suggested.title} (
                                        <SyntaxHighlightedSearchQuery query={skipped.suggested.queryExpression} />)
                                    </>
                                }
                            />
                        )
                )}
            </div>

            <Button
                type="submit"
                className="mt-2"
                variant={slim ? 'secondary' : 'primary'}
                disabled={disabled}
                size={slim ? 'sm' : undefined}
                data-testid="skipped-popover-form-submit-btn"
            >
                <Icon aria-hidden={true} className="mr-1" svgPath={mdiMagnify} />
                {slim ? <>Modify and re-run</> : <>Search again</>}
            </Button>
        </Form>
    )
}

const VALIDATE_SEARCH_JOB = gql`
    query ValidateSearchJob($query: String!) {
        validateSearchJob(query: $query) {
            alwaysNil
        }
    }
`

const CREATE_SEARCH_JOB = gql`
    mutation CreateSearchJob($query: String!) {
        createSearchJob(query: $query) {
            id
            query
            state
            URL
            startedAt
            finishedAt
            repoStats {
                total
                completed
                failed
                inProgress
            }
            creator {
                id
                displayName
                username
                avatarURL
            }
        }
    }
`

interface ExhaustiveSearchMessageProps extends TelemetryProps {
    query: string
}

export const ExhaustiveSearchMessage: FC<ExhaustiveSearchMessageProps> = props => {
    const { query, telemetryService } = props
    const navigate = useNavigate()
    const [createSearchJob, { loading, error }] = useMutation(CREATE_SEARCH_JOB)
    const { error: validationError, loading: validationLoading } = useQuery(VALIDATE_SEARCH_JOB, {
        variables: { query },
    })

    useEffect(() => {
        const validState = validationError ? 'invalid' : 'valid'
        if (!validationLoading) {
            telemetryService.log('SearchJobsSearchFormShown', { validState }, { validState })
        }
    }, [telemetryService, validationError, validationLoading])

    const handleCreateSearchJobClick = async (): Promise<void> => {
        await createSearchJob({ variables: { query } })

        telemetryService.log('SearchJobsCreateClick', {}, {})
        navigate('/search-jobs')
    }

    return (
        <section className={styles.exhaustiveSearch}>
            <header className={styles.exhaustiveSearchHeader}>
                <Text className="m-0">Create a search job:</Text>
                <ProductStatusBadge status="beta" />
            </header>

            {validationError && (
                <Alert variant="info" withIcon={true}>
                    {validationError.message}
                </Alert>
            )}

            <Text className={classNames(validationError && 'text-muted', styles.exhaustiveSearchText)}>
                Search jobs exhaustively returns all matches of a query. Results can be downloaded in JSON Lines format.
            </Text>

            {error && <ErrorAlert error={error} className="mt-3" />}
            <Button
                variant="secondary"
                size="sm"
                disabled={Boolean(validationError) || validationLoading || loading}
                className={styles.exhaustiveSearchSubmit}
                onClick={handleCreateSearchJobClick}
            >
                {loading ? (
                    <>
                        <LoadingSpinner /> Starting search job
                    </>
                ) : (
                    <>
                        <Icon aria-hidden={true} svgPath={mdiMagnify} />
                        Create a search job
                    </>
                )}
            </Button>
        </section>
    )
}
