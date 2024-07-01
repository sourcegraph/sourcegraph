import type { Meta, StoryFn } from '@storybook/react'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'

import { WebStory } from '../components/WebStory'

import { WorkflowForm, type WorkflowFormProps } from './WorkflowForm'

const config: Meta = {
    title: 'web/workflows/WorkflowForm',
    component: WorkflowForm,
    decorators: [story => <div className="container mt-5">{story()}</div>],
    parameters: {
        chromatic: { disableSnapshot: false },
    },
}

export default config

const commonProps: WorkflowFormProps = {
    submitLabel: 'Submit',
    title: 'Title',
    initialValue: {},
    onSubmit: () => {},
    loading: false,
    error: null,
    namespace: {
        __typename: 'User',
        username: 'alice',
        namespaceName: 'alice',
        displayName: 'Alice',
        id: '',
        url: '',
    },
    telemetryRecorder: noOpTelemetryRecorder,
}

export const New: StoryFn = () => (
    <WebStory>
        {webProps => (
            <WorkflowForm
                {...webProps}
                {...commonProps}
                submitLabel="Add workflow"
                title="Add workflow"
                initialValue={{}}
            />
        )}
    </WebStory>
)

export const Existing: StoryFn = () => (
    <WebStory>
        {webProps => (
            <WorkflowForm
                {...webProps}
                {...commonProps}
                submitLabel="Update workflow"
                title="Edit workflow"
                initialValue={{
                    name: 'my-workflow',
                    description: 'Existing workflow',
                    templateText: 'My template text',
                }}
            />
        )}
    </WebStory>
)

export const HasError: StoryFn = () => (
    <WebStory>
        {webProps => (
            <WorkflowForm
                {...webProps}
                {...commonProps}
                initialValue={{
                    name: 'my-workflow',
                    description: 'Existing workflow',
                    templateText: 'My template text',
                }}
                error={new Error('Error updating workflow')}
            />
        )}
    </WebStory>
)

export const HasFlash: StoryFn = () => (
    <WebStory>
        {webProps => (
            <WorkflowForm
                {...webProps}
                {...commonProps}
                initialValue={{
                    name: 'my-workflow',
                    description: 'Existing workflow',
                    templateText: 'My template text',
                }}
                flash="Success!"
            />
        )}
    </WebStory>
)
