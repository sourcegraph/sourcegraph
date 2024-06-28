import { useCallback, useEffect, type FunctionComponent } from 'react'

import { useNavigate } from 'react-router-dom'

import { useMutation } from '@sourcegraph/http-client'
import { useSettingsCascade } from '@sourcegraph/shared/src/settings/settings'
import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { Button, Link, screenReaderAnnounce } from '@sourcegraph/wildcard'

import type { CreateSavedSearchResult, CreateSavedSearchVariables } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'
import { defaultPatternTypeFromSettings } from '../util/settings'

import { SavedSearchForm, type SavedSearchFormValue } from './Form'
import { createSavedSearchMutation } from './graphql'

export const CreateForm: FunctionComponent<
    NamespaceProps & {
        isSourcegraphDotCom: boolean
        telemetryRecorder: TelemetryRecorder
    }
> = ({ namespace, isSourcegraphDotCom, telemetryRecorder }) => {
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
        async (fields: SavedSearchFormValue): Promise<void> => {
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
                screenReaderAnnounce(`Created new saved search: ${fields.description}`)
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
            isSourcegraphDotCom={isSourcegraphDotCom}
            telemetryRecorder={telemetryRecorder}
            submitLabel="Create saved search"
            onSubmit={onSubmit}
            otherButtons={
                <Button as={Link} variant="secondary" outline={true} to={`${namespace.url}/searches`}>
                    Cancel
                </Button>
            }
            initialValue={defaultValue}
            loading={loading}
            error={error}
        />
    )
}
