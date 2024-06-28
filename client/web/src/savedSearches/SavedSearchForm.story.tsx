import type { Meta, StoryFn } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import { WebStory } from '../components/WebStory'

import { SavedSearchForm, type SavedSearchFormProps } from './SavedSearchForm'

const config: Meta = {
    title: 'web/savedSearches/SavedSearchForm',
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

window.context.emailEnabled = true

const commonProps: Omit<SavedSearchFormProps, 'isLightTheme'> = {
    isSourcegraphDotCom: false,
    submitLabel: 'Submit',
    title: 'Title',
    defaultValues: {},
    authenticatedUser: null,
    onSubmit: () => {},
    loading: false,
    error: null,
    namespace: {
        __typename: 'User',
        id: '',
        url: '',
    },
    telemetryRecorder: noOpTelemetryRecorder,
}

export const New: StoryFn = () => (
    <WebStory>
        {webProps => (
            <SavedSearchForm
                {...webProps}
                {...commonProps}
                submitLabel="Add saved search"
                title="Add saved search"
                defaultValues={{}}
            />
        )}
    </WebStory>
)

export const Existing: StoryFn = () => (
    <WebStory>
        {webProps => (
            <SavedSearchForm
                {...webProps}
                {...commonProps}
                submitLabel="Update saved search"
                title="Edit saved search"
                defaultValues={{
                    id: '1',
                    description: 'Existing saved search',
                    query: 'test',
                }}
            />
        )}
    </WebStory>
)
