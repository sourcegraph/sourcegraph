import { useCallback, useEffect, type FunctionComponent } from 'react'

import { useNavigate } from 'react-router-dom'

import { useMutation } from '@sourcegraph/http-client'
import { useSettingsCascade } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { screenReaderAnnounce } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import type { CreateSavedSearchResult, CreateSavedSearchVariables } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'
import { defaultPatternTypeFromSettings } from '../util/settings'

import { createSavedSearchMutation } from './backend'
import { SavedSearchForm, type SavedSearchFormValue } from './SavedSearchForm'

export const SavedSearchCreateForm: FunctionComponent<
    NamespaceProps & {
        authenticatedUser: AuthenticatedUser | null
        isSourcegraphDotCom: boolean
        telemetryRecorder: TelemetryRecorder
    }
> = ({ namespace, authenticatedUser, isSourcegraphDotCom, telemetryRecorder }) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('savedSearches.new', 'view', {
            metadata: namespaceTelemetryMetadata(namespace),
        })
    }, [namespace, telemetryRecorder])

    const navigate = useNavigate()

    const [createSavedSearch, { loading, error }] = useMutation<CreateSavedSearchResult, CreateSavedSearchVariables>(
        createSavedSearchMutation,
        {}
    )

    const onSubmit = useCallback(
        async (fields: Omit<SavedSearchFormValue, 'id'>): Promise<void> => {
            try {
                await createSavedSearch({
                    variables: {
                        input: {
                            description: fields.description,
                            query: fields.query,
                            owner: namespace.id,
                        },
                    },
                })
                telemetryRecorder.recordEvent('savedSearches', 'create', {
                    metadata: namespaceTelemetryMetadata(namespace),
                })
                screenReaderAnnounce(`Saved ${fields.description} search`)
                navigate(`${namespace.url}/searches`, {
                    state: { description: fields.description },
                })
            } catch {
                // Mutation error is read in useMutation call.
            }
        },
        [namespace, telemetryRecorder, navigate, createSavedSearch]
    )

    const searchParameters = new URLSearchParams(location.search)
    const query = searchParameters.get('query')
    const settingsCascade = useSettingsCascade()
    const patternType = searchParameters.get('patternType') ?? defaultPatternTypeFromSettings(settingsCascade)
    const defaultValue: Partial<SavedSearchFormValue> = {
        query: [patternType ? `patternType:${patternType} ` : null, query].filter(Boolean).join(''),
    }

    return (
        <SavedSearchForm
            namespace={namespace}
            authenticatedUser={authenticatedUser}
            isSourcegraphDotCom={isSourcegraphDotCom}
            telemetryRecorder={telemetryRecorder}
            submitLabel="Create"
            title="New saved search"
            defaultValues={defaultValue}
            onSubmit={onSubmit}
            loading={loading}
            error={error}
        />
    )
}
