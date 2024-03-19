import React, { useCallback, useEffect, useState } from 'react'

import { mdiCheck } from '@mdi/js'
import * as jsonc from 'jsonc-parser'
import type { Observable } from 'rxjs'
import { delay, mergeMap, startWith, tap } from 'rxjs/operators'

import type { SearchContextRepositoryRevisionsFields } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, useEventObservable, Alert, Icon } from '@sourcegraph/wildcard'

import { DynamicallyImportedMonacoSettingsEditor } from '../../settings/DynamicallyImportedMonacoSettingsEditor'

import { MAX_REVISION_LENGTH, REPOSITORY_KEY, REVISIONS_KEY } from './repositoryRevisionsConfigParser'

const LOADING = 'LOADING' as const

const REPOSITORY_REVISIONS_INPUT_COMMENT = `// Define each repository and its revisions as objects
//
// [
//   {
//     "${REPOSITORY_KEY}": "github.com/example/repository-name",
//     "${REVISIONS_KEY}": [
//       "main", "ls/sample-branch", "aa2cf5feda231c46a329c01ca55c45f29b1708c4"
//     ]
//   }
// ]
`

export const REPOSITORIES_REVISIONS_CONFIG_SCHEMA = {
    $id: 'repositoriesAndRevisions.schema.json#',
    allowComments: true,
    type: 'array',
    items: {
        type: 'object',
        required: [REPOSITORY_KEY, REVISIONS_KEY],
        properties: {
            [REPOSITORY_KEY]: {
                type: 'string',
            },
            [REVISIONS_KEY]: {
                type: 'array',
                items: {
                    type: 'string',
                    maxLength: MAX_REVISION_LENGTH,
                },
            },
        },
    },
}

const defaultModificationOptions: jsonc.ModificationOptions = {
    formattingOptions: {
        eol: '\n',
        insertSpaces: true,
        tabSize: 2,
    },
}

const actions: {
    id: string
    label: string
    run: (config: string) => { edits: jsonc.Edit[]; selectText: string }
}[] = [
    {
        id: 'addRepository',
        label: 'Add repository',
        run: config => {
            const value = { [REPOSITORY_KEY]: 'github.com/example/repository-name', [REVISIONS_KEY]: ['HEAD'] }
            const edits = jsonc.modify(config, [-1], value, defaultModificationOptions)
            return { edits, selectText: 'github.com/example/repository-name' }
        },
    },
]

export interface SearchContextRepositoriesFormAreaProps extends TelemetryProps, TelemetryV2Props {
    isLightTheme: boolean
    repositories: SearchContextRepositoryRevisionsFields[] | undefined
    validateRepositories: () => Observable<Error[]>
    onChange: (config: string, isInitialValue?: boolean) => void
}

export const SearchContextRepositoriesFormArea: React.FunctionComponent<
    React.PropsWithChildren<SearchContextRepositoriesFormAreaProps>
> = ({ isLightTheme, telemetryService, repositories, onChange, validateRepositories, telemetryRecorder }) => {
    const [hasTestedConfig, setHasTestedConfig] = useState(false)
    const [triggerTestConfig, triggerTestConfigErrors] = useEventObservable(
        useCallback(
            (click: Observable<React.MouseEvent<HTMLButtonElement>>) =>
                click.pipe(
                    mergeMap(() =>
                        validateRepositories().pipe(
                            delay(500),
                            tap(() => setHasTestedConfig(true)),
                            startWith(LOADING)
                        )
                    )
                ),
            [validateRepositories, setHasTestedConfig]
        )
    )

    const testConfigButtonText =
        triggerTestConfigErrors === LOADING
            ? 'Testing configuration...'
            : hasTestedConfig
            ? 'Test configuration again'
            : 'Test configuration'

    const isValidConfig =
        hasTestedConfig && triggerTestConfigErrors !== undefined && triggerTestConfigErrors.length === 0

    const [repositoriesConfig, setRepositoriesConfig] = useState('')
    useEffect(
        () => {
            const mappedRepositories = repositories?.map(repository => ({
                repository: repository.repository.name,
                revisions: repository.revisions,
            }))
            const config =
                REPOSITORY_REVISIONS_INPUT_COMMENT +
                (mappedRepositories ? JSON.stringify(mappedRepositories, undefined, 2) : '[]')
            setRepositoriesConfig(config)
            onChange(config, true)
        },
        // Only stringify repositories on initial load
        // eslint-disable-next-line react-hooks/exhaustive-deps
        []
    )

    return (
        <div data-testid="repositories-config-area">
            <DynamicallyImportedMonacoSettingsEditor
                value={repositoriesConfig}
                jsonSchema={REPOSITORIES_REVISIONS_CONFIG_SCHEMA}
                actions={actions}
                canEdit={false}
                onChange={onChange}
                height={300}
                isLightTheme={isLightTheme}
                telemetryService={telemetryService}
                telemetryRecorder={telemetryRecorder}
                blockNavigationIfDirty={false}
            />
            {triggerTestConfigErrors && triggerTestConfigErrors !== LOADING && triggerTestConfigErrors.length > 0 && (
                <Alert className="my-2" variant="danger">
                    <strong>The following problems were found:</strong>
                    <ul className="mt-2">
                        {triggerTestConfigErrors.map(error => (
                            <li key={error.message}>{error.message}</li>
                        ))}
                    </ul>
                </Alert>
            )}
            <Button
                className="mt-3"
                data-testid="repositories-config-button"
                onClick={triggerTestConfig}
                disabled={triggerTestConfigErrors === LOADING || isValidConfig}
                outline={true}
                variant="secondary"
                size="sm"
            >
                {isValidConfig ? (
                    <span className="d-flex align-items-center">
                        <Icon
                            aria-hidden={true}
                            as="span"
                            data-testid="repositories-config-success"
                            className="text-success mr-1"
                        >
                            <Icon svgPath={mdiCheck} inline={false} aria-hidden={true} />{' '}
                        </Icon>
                        <span>Valid configuration</span>
                    </span>
                ) : (
                    testConfigButtonText
                )}
            </Button>
        </div>
    )
}
