import React, { useState, useEffect, useRef, useCallback } from 'react'
import * as H from 'history'
import { ChangesetReviewState, ChangesetCheckState, ChangesetState } from '../../../../graphql-operations'
import { isValidChangesetState, isValidChangesetReviewState, isValidChangesetCheckState } from '../../utils'
import { ChangesetFilter } from '../../ChangesetFilter'
import { Form } from 'reactstrap'

export interface ChangesetFilters {
    state: ChangesetState | null
    reviewState: ChangesetReviewState | null
    checkState: ChangesetCheckState | null
    search: string | null
}

export interface ChangesetFilterRowProps {
    history: H.History
    location: H.Location
    onFiltersChange: (newFilters: ChangesetFilters) => void
}

export const ChangesetFilterRow: React.FunctionComponent<ChangesetFilterRowProps> = ({
    history,
    location,
    onFiltersChange,
}) => {
    const searchElement = useRef<HTMLInputElement | null>(null)
    const searchParameters = new URLSearchParams(location.search)
    const [state, setState] = useState<ChangesetState | undefined>(() => {
        const value = searchParameters.get('status')
        return value && isValidChangesetState(value) ? value : undefined
    })
    const [reviewState, setReviewState] = useState<ChangesetReviewState | undefined>(() => {
        const value = searchParameters.get('review_state')
        return value && isValidChangesetReviewState(value) ? value : undefined
    })
    const [checkState, setCheckState] = useState<ChangesetCheckState | undefined>(() => {
        const value = searchParameters.get('check_state')
        return value && isValidChangesetCheckState(value) ? value : undefined
    })
    const [search, setSearch] = useState<string | undefined>(() => searchParameters.get('search') ?? undefined)
    useEffect(() => {
        const searchParameters = new URLSearchParams(location.search)
        if (state) {
            searchParameters.set('status', state)
        } else {
            searchParameters.delete('status')
        }
        if (reviewState) {
            searchParameters.set('review_state', reviewState)
        } else {
            searchParameters.delete('review_state')
        }
        if (checkState) {
            searchParameters.set('check_state', checkState)
        } else {
            searchParameters.delete('check_state')
        }
        if (search) {
            searchParameters.set('search', search)
        } else {
            searchParameters.delete('search')
        }
        if (location.search !== searchParameters.toString()) {
            history.replace({ ...location, search: searchParameters.toString() })
        }
        // Update the filters in the parent component.
        onFiltersChange({
            state: state || null,
            reviewState: reviewState || null,
            checkState: checkState || null,
            search: search || null,
        })
        // We cannot depend on the history, since it's modified by this hook and that would cause an infinite render loop.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [state, reviewState, checkState, search])

    const onSubmit = useCallback(
        (event?: React.FormEvent<HTMLFormElement>): void => {
            event?.preventDefault()
            setSearch(searchElement.current?.value)
        },
        [setSearch, searchElement]
    )

    return (
        <>
            <div className="row no-gutters">
                <div className="m-0 col">
                    <Form className="form-inline d-flex my-2" onSubmit={onSubmit}>
                        <input
                            className="form-control flex-grow-1"
                            type="search"
                            ref={searchElement}
                            defaultValue={search}
                            placeholder="Search title and repository name"
                        />
                    </Form>
                </div>
                <div className="w-100 d-block d-md-none" />
                <div className="m-0 col col-md-auto">
                    <div className="row no-gutters">
                        <div className="col my-2 ml-0 ml-md-2">
                            <ChangesetFilter<ChangesetState>
                                values={Object.values(ChangesetState)}
                                label="Status"
                                selected={state}
                                onChange={setState}
                                className="w-100"
                            />
                        </div>
                        <div className="col my-2 ml-2">
                            <ChangesetFilter<ChangesetCheckState>
                                values={Object.values(ChangesetCheckState)}
                                label="Check state"
                                selected={checkState}
                                onChange={setCheckState}
                                className="w-100"
                            />
                        </div>
                        <div className="w-100 d-block d-sm-none" />
                        <div className="col my-2 ml-0 ml-sm-2">
                            <ChangesetFilter<ChangesetReviewState>
                                values={Object.values(ChangesetReviewState)}
                                label="Review state"
                                selected={reviewState}
                                onChange={setReviewState}
                                className="w-100"
                            />
                        </div>
                        <div className="col d-block d-sm-none ml-2" />
                    </div>
                </div>
            </div>
        </>
    )
}
