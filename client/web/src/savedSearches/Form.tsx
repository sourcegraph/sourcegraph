import React, { useEffect, useState, type ReactNode } from 'react'

import { mdiHelpCircleOutline, mdiOpenInNew } from '@mdi/js'

import { LazyQueryInputFormControl } from '@sourcegraph/branded'
import type { QueryState } from '@sourcegraph/shared/src/search'
import { useSettingsCascade } from '@sourcegraph/shared/src/settings/settings'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import {
    Alert,
    Button,
    Container,
    ErrorAlert,
    Form,
    Icon,
    Input,
    InputDescription,
    Label,
    Link,
} from '@sourcegraph/wildcard'

import type { SavedSearchInput, SavedSearchUpdateInput, SearchPatternType } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { defaultPatternTypeFromSettings } from '../util/settings'

export interface SavedSearchFormValue
    extends Pick<SavedSearchInput | SavedSearchUpdateInput, 'description' | 'query'> {}

export interface SavedSearchFormProps extends NamespaceProps {
    initialValue?: Partial<SavedSearchFormValue>
    submitLabel: string
    onSubmit: (fields: SavedSearchFormValue) => void
    otherButtons?: ReactNode
    loading: boolean
    error?: any
    flash?: ReactNode
    isSourcegraphDotCom: boolean
}

export const SavedSearchForm: React.FunctionComponent<React.PropsWithChildren<SavedSearchFormProps>> = ({
    initialValue,
    submitLabel,
    onSubmit,
    otherButtons,
    loading,
    error,
    flash,
    isSourcegraphDotCom,
}) => {
    const [value, setValue] = useState<SavedSearchFormValue>(() => ({
        description: initialValue?.description || '',
        query: initialValue?.query || '',
    }))

    /**
     * Returns an input change handler that updates the SavedQueryFields in the component's state
     * @param key The key of saved query fields that a change of this input should update
     */
    const createInputChangeHandler =
        (key: keyof SavedSearchFormValue): React.FormEventHandler<HTMLInputElement> =>
        event => {
            const { value, checked, type } = event.currentTarget
            setValue(values => ({
                ...values,
                [key]: type === 'checkbox' ? checked : value,
            }))
        }

    const [queryState, setQueryState] = useState<QueryState>({ query: value.query || '' })
    const defaultPatternType: SearchPatternType = defaultPatternTypeFromSettings(useSettingsCascade())
    useEffect(() => {
        setValue(values => ({ ...values, query: queryState.query }))
    }, [queryState.query])

    const QUERY_LABEL_ID = 'query-input-label'

    return (
        <Form
            onSubmit={event => {
                event.preventDefault()
                onSubmit(value)
            }}
            data-testid="saved-search-form"
            className="d-flex flex-column flex-gap-4"
        >
            <Container>
                <Input
                    name="description"
                    required={true}
                    value={value.description}
                    onChange={createInputChangeHandler('description')}
                    className="form-group"
                    label="Description"
                    autoFocus={true}
                />
                <div>
                    <Label id={QUERY_LABEL_ID}>Query</Label>
                    <LazyQueryInputFormControl
                        patternType={defaultPatternType}
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        caseSensitive={false}
                        queryState={queryState}
                        onChange={setQueryState}
                        preventNewLine={false}
                        ariaLabelledby={QUERY_LABEL_ID}
                    />
                    <div className="d-flex align-items-start justify-content-between flex-gap-2 flex-wrap">
                        <Link
                            to={`/search?${buildSearchURLQuery(queryState.query, defaultPatternType, false)}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="d-inline-flex align-items-center flex-gap-1 my-1"
                        >
                            Preview results
                            <Icon aria-label="Open in new window" svgPath={mdiOpenInNew} />
                        </Link>
                        <InputDescription className="mb-1 ml-0">
                            <Icon aria-label="Info" className="mr-1" svgPath={mdiHelpCircleOutline} />
                            <Link to="/help/code-search/queries" target="_blank" rel="noopener noreferrer">
                                Query reference
                            </Link>{' '}
                            and{' '}
                            <Link to="/help/code-search/examples" target="_blank" rel="noopener noreferrer">
                                examples
                            </Link>
                        </InputDescription>
                    </div>
                </div>
            </Container>
            <div className="d-flex flex-gap-4">
                <Button type="submit" disabled={loading} variant="primary">
                    {submitLabel}
                </Button>
                {otherButtons}
            </div>
            {flash && !loading && (
                <Alert variant="success" className="mb-0">
                    {flash}
                </Alert>
            )}
            {error && !loading && <ErrorAlert className="mb-0" error={error} />}
        </Form>
    )
}
