import { useCallback } from 'react'

import { mdiMagnifyPlus } from '@mdi/js'
import classNames from 'classnames'

import { useQuery } from '@sourcegraph/http-client'
import {
    Button,
    Icon,
    H3,
    Popover,
    PopoverContent,
    PopoverTail,
    PopoverTrigger,
    Position,
    TextArea,
    Tooltip,
} from '@sourcegraph/wildcard'

import { AllEventNamesResult } from '../../../graphql-operations'

import { ALL_EVENT_NAMES } from './queries'

import styles from './index.module.scss'

interface eventNamesInputButton {
    display: string
    value: string
    description?: string
    onClick?: (value: string) => void
}

const buttons: eventNamesInputButton[] = [
    {
        display: 'Searches',
        value: 'SearchResultsQueried',
        description: 'Count of search queries run by a user in the Sourcegraph web UI.',
    },
    {
        display: 'Code navigation actions',
        value: 'findRefs,hover,goToDef',
        description:
            "Count of code navigation actions like 'Find References', 'Go to Definition', and hovers run by a user in the Sourcegraph web UI.",
    },
    {
        display: 'Cody autocomplete suggestions',
        value: 'CodyVSCodeExtension:completions:suggested,CodtyJetBrainsExtension:completions:suggested',
        description: 'Count of Cody autocompletions suggested to the user in editor extensions.',
    },
    {
        display: 'Cody autocomplete acceptances',
        value: 'CodyVSCodeExtension:completions:accepted,CodyJetBrainsExtension:completions:accepted',
        description: 'Count of Cody autocompletions accepted by the user in editor extensions.',
    },
]

interface Props {
    onChange: (value: string) => void
    value: string
    label: string
}

export const EventNamesInputSection = ({ label, value, onChange }: Props): JSX.Element => {
    const allEventNamesResults = useQuery<AllEventNamesResult>(ALL_EVENT_NAMES, {})

    return (
        <div className="container">
            <H3>{label}</H3>
            <div className="row d-flex">
                <TextArea
                    id="event-names-input"
                    className={classNames('flex-grow-1 m-0', styles.textarea)}
                    value={value}
                    onChange={useCallback(
                        (event: React.ChangeEvent<HTMLTextAreaElement>) => onChange(event.target.value),
                        [onChange]
                    )}
                />
                <AddEventNamePopover
                    eventNames={allEventNamesResults.data?.site.analytics.allEventNames || []}
                    onSelect={(selectedValue: string) =>
                        onChange(value.length > 0 ? value + ', ' + selectedValue : selectedValue)
                    }
                />
            </div>
            <div className="row mt-2">
                {buttons.map(b => (
                    <Tooltip key={b.value} content={b.description} placement="top">
                        <Button
                            key={b.value}
                            value={b.value}
                            onClick={() => {
                                onChange(b.value)
                                return b.onClick ? b.onClick(b.value) : undefined
                            }}
                            outline={true}
                            variant="secondary"
                            display="inline"
                            size="sm"
                            className="mr-1"
                        >
                            {b.display}
                        </Button>
                    </Tooltip>
                ))}
            </div>
        </div>
    )
}

interface PopoverProps {
    eventNames: (string | null)[]
    onSelect: (e: string) => void
}

const AddEventNamePopover = ({ eventNames, onSelect }: PopoverProps): JSX.Element => (
    <Popover>
        <PopoverTrigger as={Button} variant="primary" aria-label="Add an event">
            <Icon svgPath={mdiMagnifyPlus} size="sm" aria-hidden={true} />
        </PopoverTrigger>

        <PopoverContent position={Position.bottom} className="d-flex flex-column">
            {eventNames.map(eventName =>
                eventName ? (
                    <Button
                        className="d-block"
                        key={eventName}
                        onClick={() => {
                            onSelect(eventName)
                        }}
                    >
                        {eventName}
                    </Button>
                ) : undefined
            )}
        </PopoverContent>
        <PopoverTail size="sm" />
    </Popover>
)
