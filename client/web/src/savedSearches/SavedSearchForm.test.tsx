import { describe, expect, test, vi } from 'vitest'

import { LazyQueryInputFormControl } from '@sourcegraph/branded'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import { SearchPatternType } from '../graphql-operations'

import { SavedSearchForm } from './SavedSearchForm'

const DEFAULT_PATTERN_TYPE = SearchPatternType.regexp

describe('SavedSearchForm', () => {
    test('renders LazyQueryInputFormControl with the default patternType', () => {
        vi.mock('@sourcegraph/branded', () => ({
            LazyQueryInputFormControl: vi.fn(() => null),
        }))
        vi.mock('../util/settings', () => ({
            defaultPatternTypeFromSettings: () => DEFAULT_PATTERN_TYPE,
        }))

        renderWithBrandedContext(
            <SavedSearchForm
                isSourcegraphDotCom={false}
                submitLabel="Submit"
                title="Title"
                defaultValues={{}}
                authenticatedUser={null}
                onSubmit={() => {}}
                loading={false}
                error={null}
                namespace={{
                    __typename: 'User',
                    id: '',
                    url: '',
                }}
                telemetryRecorder={noOpTelemetryRecorder}
            />
        )

        expect(LazyQueryInputFormControl).toHaveBeenCalledWith(
            expect.objectContaining({
                patternType: DEFAULT_PATTERN_TYPE,
            }),
            expect.anything()
        )
    })
})
