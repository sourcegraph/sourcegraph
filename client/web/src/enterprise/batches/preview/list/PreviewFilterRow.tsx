import React, { useCallback, useEffect, useRef, useState } from 'react'
import * as H from 'history'
import { ChangesetSpecOperation, ChangesetState } from '../../../../graphql-operations'
import { isValidChangesetSpecOperation, isValidChangesetState } from '../../utils'
import { ChangesetFilter } from '../../ChangesetFilter'
import { Form } from 'reactstrap'

export interface PreviewFilters {
    search: string | null
    currentState: ChangesetState | null
    action: ChangesetSpecOperation | null
}

export interface PreviewFilterRowProps {
    history: H.History
    location: H.Location
    onFiltersChange: (newFilters: PreviewFilters) => void
}

export const PreviewFilterRow: React.FunctionComponent<PreviewFilterRowProps> = ({
    history,
    location,
    onFiltersChange,
}) => {
    const urlParameters = new URLSearchParams(location.search)

    const searchElement = useRef<HTMLInputElement | null>(null)

    const [action, setAction] = useState<ChangesetSpecOperation | undefined>(() => {
        const value = urlParameters.get('action')
        return value && isValidChangesetSpecOperation(value) ? value : undefined
    })
    const [currentState, setCurrentState] = useState<ChangesetState | undefined>(() => {
        const value = urlParameters.get('current_state')
        return value && isValidChangesetState(value) ? value : undefined
    })
    const [search, setSearch] = useState<string | undefined>(() => urlParameters.get('search') ?? undefined)

    useEffect(() => {
        const urlParameters = new URLSearchParams(location.search)

        if (search) {
            urlParameters.set('search', search)
        } else {
            urlParameters.delete('search')
        }

        if (action) {
            urlParameters.set('action', action)
        } else {
            urlParameters.delete('action')
        }

        if (currentState) {
            urlParameters.set('current_state', currentState)
        } else {
            urlParameters.delete('current_state')
        }

        if (location.search !== urlParameters.toString()) {
            history.replace({ ...location, search: urlParameters.toString() })
        }

        // Update the filters in the parent component.
        onFiltersChange({ search: search || null, action: action || null, currentState: currentState || null })

        // We cannot depend on the history, since it's modified by this hook and that would cause an infinite render loop.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [search, action, currentState])

    const onSubmit = useCallback(
        (event: React.FormEvent<HTMLFormElement>): void => {
            event.preventDefault()
            setSearch(searchElement.current?.value)
        },
        [setSearch, searchElement]
    )

    return (
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
                            label="Current state"
                            selected={currentState}
                            onChange={setCurrentState}
                            className="w-100"
                        />
                    </div>
                    <div className="col my-2 ml-0 ml-md-2">
                        <ChangesetFilter<ChangesetSpecOperation>
                            values={Object.values(ChangesetSpecOperation)}
                            label="Actions"
                            selected={action}
                            onChange={setAction}
                            className="w-100"
                        />
                    </div>
                </div>
            </div>
        </div>
    )
}
