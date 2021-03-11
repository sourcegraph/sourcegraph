import classNames from 'classnames'
import * as H from 'history'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import ChevronDownIcon from 'mdi-react/ChevronDownIcon'
import ChevronLeftIcon from 'mdi-react/ChevronLeftIcon'
import InformationOutlineIcon from 'mdi-react/InformationOutlineIcon'
import SearchIcon from 'mdi-react/SearchIcon'
import React, { useCallback, useState } from 'react'
import { Button, Collapse, Form, FormGroup, Input, Label } from 'reactstrap'
import { Markdown } from '../../../../../../shared/src/components/Markdown'
import { renderMarkdown } from '../../../../../../shared/src/util/markdown'
import { SyntaxHighlightedSearchQuery } from '../../../../components/SyntaxHighlightedSearchQuery'
import { Skipped } from '../../../stream'
import { StreamingProgressProps } from './StreamingProgress'

const severityToNumber = (severity: Skipped['severity']): number => {
    switch (severity) {
        case 'error':
            return 1
        case 'warn':
            return 2
        case 'info':
            return 3
    }
}

const sortBySeverity = (a: Skipped, b: Skipped): number => {
    const aSev = severityToNumber(a.severity)
    const bSev = severityToNumber(b.severity)

    return aSev - bSev
}

const SkippedMessage: React.FunctionComponent<{ skipped: Skipped; history: H.History; startOpen: boolean }> = ({
    skipped,
    history,
    startOpen,
}) => {
    const [isOpen, setIsOpen] = useState(startOpen)

    const toggleIsOpen = useCallback(() => setIsOpen(oldValue => !oldValue), [])

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
            className={classNames('streaming-skipped-item pt-2 w-100', {
                'streaming-skipped-item--warn': skipped.severity !== 'info',
            })}
        >
            <Button
                className="streaming-skipped-item__button py-2 w-100 bg-transparent border-0"
                onClick={toggleIsOpen}
                onKeyDown={onKeyDown}
                disabled={!skipped.message}
            >
                <h4 className="d-flex align-items-center mb-0 w-100">
                    {skipped.severity === 'info' ? (
                        <InformationOutlineIcon className="icon-inline mr-2 streaming-skipped-item__icon flex-shrink-0" />
                    ) : (
                        <AlertCircleIcon className="icon-inline mr-2 streaming-skipped-item__icon flex-shrink-0" />
                    )}
                    <span className="flex-grow-1 text-left">{skipped.title}</span>

                    {skipped.message &&
                        (isOpen ? (
                            <ChevronDownIcon className="icon-inline flex-shrink-0" />
                        ) : (
                            <ChevronLeftIcon className="icon-inline flex-shrink-0" />
                        ))}
                </h4>
            </Button>
            {skipped.message && (
                <Collapse isOpen={isOpen}>
                    <Markdown
                        className="streaming-skipped-item__message text-left py-1"
                        dangerousInnerHTML={renderMarkdown(skipped.message)}
                        history={history}
                    />
                </Collapse>
            )}
            <div className="streaming-skipped-item__bottom-border-spacer mt-2" />
        </div>
    )
}

export const StreamingProgressSkippedPopover: React.FunctionComponent<
    Pick<StreamingProgressProps, 'progress' | 'onSearchAgain' | 'history'>
> = ({ progress, onSearchAgain, history }) => {
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

    const sortedSkippedItems = progress.skipped.sort(sortBySeverity)

    return (
        <>
            {sortedSkippedItems.map((skipped, index) => (
                <SkippedMessage
                    key={skipped.reason}
                    skipped={skipped}
                    history={history}
                    // Start with first item open, but only if it's not info severity or if there's only one item
                    startOpen={index === 0 && (skipped.severity !== 'info' || sortedSkippedItems.length === 1)}
                />
            ))}
            {sortedSkippedItems.some(skipped => skipped.suggested) && (
                <Form className="pb-3 px-3" onSubmit={submitHandler}>
                    <div className="mb-2 mt-3">Search again:</div>
                    <FormGroup check={true}>
                        {sortedSkippedItems.map(
                            skipped =>
                                skipped.suggested && (
                                    <Label
                                        check={true}
                                        className="mb-1 d-block"
                                        key={skipped.suggested.queryExpression}
                                    >
                                        <Input
                                            type="checkbox"
                                            value={skipped.suggested.queryExpression}
                                            onChange={checkboxHandler}
                                        />{' '}
                                        {skipped.suggested.title} (
                                        <SyntaxHighlightedSearchQuery query={skipped.suggested.queryExpression} />)
                                    </Label>
                                )
                        )}
                    </FormGroup>

                    <Button
                        type="submit"
                        className="mt-2"
                        color="primary"
                        disabled={selectedSuggestedSearches.size === 0}
                    >
                        <SearchIcon className="icon-inline mr-1" />
                        Search again
                    </Button>
                </Form>
            )}
        </>
    )
}
