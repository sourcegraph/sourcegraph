import React, { useEffect, useState } from 'react'

import type { Omit } from 'utility-types'

import { LazyQueryInputFormControl } from '@sourcegraph/branded'
import type { QueryState } from '@sourcegraph/shared/src/search'
import { useSettingsCascade } from '@sourcegraph/shared/src/settings/settings'
import { Button, Container, ErrorAlert, Form, Input, Label, PageHeader } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { PageTitle } from '../components/PageTitle'
import type { SavedSearchFields, SearchPatternType } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { defaultPatternTypeFromSettings } from '../util/settings'

export interface SavedSearchFormValue extends Pick<SavedSearchFields, 'id' | 'description' | 'query'> {}

export interface SavedSearchFormProps extends NamespaceProps {
    authenticatedUser: AuthenticatedUser | null
    defaultValues?: Partial<SavedSearchFormValue>
    title?: string
    submitLabel: string
    onSubmit: (fields: Omit<SavedSearchFormValue, 'id'>) => void
    loading: boolean
    error?: any
    isSourcegraphDotCom: boolean
}

export const SavedSearchForm: React.FunctionComponent<React.PropsWithChildren<SavedSearchFormProps>> = props => {
    const [value, setValue] = useState<Omit<SavedSearchFormValue, 'id'>>(() => ({
        description: props.defaultValues?.description || '',
        query: props.defaultValues?.query || '',
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

    const handleSubmit = (event: React.FormEvent<HTMLFormElement>): void => {
        event.preventDefault()
        props.onSubmit(value)
    }

    const { query, description } = value

    const [queryState, setQueryState] = useState<QueryState>({ query: query || '' })

    const defaultPatternType: SearchPatternType = defaultPatternTypeFromSettings(useSettingsCascade())

    useEffect(() => {
        setValue(values => ({ ...values, query: queryState.query }))
    }, [queryState.query])

    return (
        <div className="saved-search-form" data-testid="saved-search-form">
            <PageHeader className="mb-3">
                <PageTitle title={props.title} />
                <PageHeader.Heading as="h3" styleAs="h2">
                    <PageHeader.Breadcrumb>{props.title}</PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>
            <Form onSubmit={handleSubmit}>
                <Container className="mb-3">
                    <Input
                        name="description"
                        required={true}
                        value={description}
                        onChange={createInputChangeHandler('description')}
                        className="form-group"
                        label="Description"
                        autoFocus={true}
                    />
                    <Label className="w-100 form-group'">
                        <div className="mb-2">Query</div>
                        <LazyQueryInputFormControl
                            patternType={defaultPatternType}
                            isSourcegraphDotCom={props.isSourcegraphDotCom}
                            caseSensitive={false}
                            queryState={queryState}
                            onChange={setQueryState}
                            preventNewLine={true}
                        />
                    </Label>
                </Container>
                <Button
                    type="submit"
                    disabled={props.loading}
                    className="mb-3 test-saved-search-form-submit-button"
                    variant="primary"
                >
                    {props.submitLabel}
                </Button>

                {props.error && !props.loading && <ErrorAlert className="mb-3" error={props.error} />}
            </Form>
        </div>
    )
}
