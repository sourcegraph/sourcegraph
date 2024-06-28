import React, { useCallback, useMemo, useState } from 'react'

import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'
import { from, of, throwError, type Observable } from 'rxjs'
import { catchError, map, startWith, switchMap, tap } from 'rxjs/operators'

import { LazyQueryInputFormControl, SyntaxHighlightedSearchQuery } from '@sourcegraph/branded'
import { asError, createAggregateError, isErrorLike } from '@sourcegraph/common'
import {
    SearchPatternType,
    type Scalars,
    type SearchContextFields,
    type SearchContextInput,
    type SearchContextRepositoryRevisionsInput,
} from '@sourcegraph/shared/src/graphql-operations'
import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import type { QueryState, SearchContextProps } from '@sourcegraph/shared/src/search'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import {
    Alert,
    Button,
    Code,
    Container,
    Form,
    Input,
    Link,
    RadioButton,
    TextArea,
    useEventObservable,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { ALLOW_NAVIGATION, AwayPrompt } from '../../components/AwayPrompt'

import { fetchRepositoriesByNames } from './backend'
import { DeleteSearchContextModal } from './DeleteSearchContextModal'
import { parseConfig } from './repositoryRevisionsConfigParser'
import {
    getSelectedNamespace,
    getSelectedNamespaceFromUser,
    SearchContextOwnerDropdown,
    type SelectedNamespace,
    type SelectedNamespaceType,
} from './SearchContextOwnerDropdown'
import { SearchContextRepositoriesFormArea } from './SearchContextRepositoriesFormArea'

import styles from './SearchContextForm.module.scss'

const MAX_DESCRIPTION_LENGTH = 1024
const MAX_NAME_LENGTH = 32
const VALIDATE_NAME_REGEXP = /^[\w./-]+$/

type SelectedVisibility = 'public' | 'private'

type ContextType = 'dynamic' | 'static'

interface VisibilityRadioButton {
    visibility: SelectedVisibility
    title: string
    description: string
}

function getVisibilityRadioButtons(selectedNamespaceType: SelectedNamespaceType): VisibilityRadioButton[] {
    const privateVisibilityDescription =
        selectedNamespaceType === 'global-owner'
            ? 'Only site-admins can view this context.'
            : selectedNamespaceType === 'org'
            ? 'Only organization members can view this context.'
            : 'Only you can view this context.'

    return [
        {
            visibility: 'public',
            title: 'Public',
            description:
                'Anyone can view this context. Public repositories will be visible to everyone. ' +
                'Private repositories will only be visible to users that have permission to view the repository via the code host.',
        },
        {
            visibility: 'private',
            title: 'Private',
            description: privateVisibilityDescription,
        },
    ]
}

function getSearchContextSpecPreview(selectedNamespace: SelectedNamespace, searchContextName: string): JSX.Element {
    return (
        <Code className={styles.searchContextFormPreview} data-testid="search-context-preview">
            {/*
                a11y-ignore
                Rule: "color-contrast" (Elements must have sufficient color contrast)
                GitHub issue: https://github.com/sourcegraph/sourcegraph/issues/33343
            */}
            <span className="search-filter-keyword a11y-ignore">context:</span>
            {selectedNamespace.name.length > 0 && (
                <>
                    <span className="search-keyword">@</span>
                    <span>{selectedNamespace.name}/</span>
                </>
            )}
            <span>{searchContextName}</span>
        </Code>
    )
}

const LOADING = 'loading' as const

export interface SearchContextFormProps
    extends TelemetryProps,
        Pick<SearchContextProps, 'deleteSearchContext'>,
        PlatformContextProps<'requestGraphQL' | 'telemetryRecorder'> {
    searchContext?: SearchContextFields
    query?: string
    authenticatedUser: AuthenticatedUser
    isSourcegraphDotCom: boolean

    onSubmit: (
        id: Scalars['ID'] | undefined,
        searchContext: SearchContextInput,
        repositories: SearchContextRepositoryRevisionsInput[]
    ) => Observable<SearchContextFields>
}

const searchContextVisibility = (searchContext: SearchContextFields): SelectedVisibility =>
    searchContext.public ? 'public' : 'private'

type RepositoriesParseResult =
    | {
          type: 'errors'
          errors: Error[]
      }
    | {
          type: 'repositories'
          repositories: SearchContextRepositoryRevisionsInput[]
      }

export const SearchContextForm: React.FunctionComponent<React.PropsWithChildren<SearchContextFormProps>> = props => {
    const { authenticatedUser, onSubmit, searchContext, deleteSearchContext, isSourcegraphDotCom, platformContext } =
        props
    const navigate = useNavigate()
    const isLightTheme = useIsLightTheme()

    const [name, setName] = useState(searchContext ? searchContext.name : '')
    const [description, setDescription] = useState(searchContext ? searchContext.description : '')
    const [visibility, setVisibility] = useState<SelectedVisibility>(
        searchContext ? searchContextVisibility(searchContext) : 'public'
    )
    const [contextType, setContextType] = useState<ContextType>(
        searchContext ? (searchContext.query.length > 0 ? 'dynamic' : 'static') : 'dynamic'
    )
    const [queryState, setQueryState] = useState<QueryState>({ query: searchContext?.query || props.query || '' })

    const isValidName = useMemo(() => name.length === 0 || name.match(VALIDATE_NAME_REGEXP) !== null, [name])

    const [selectedNamespace, setSelectedNamespace] = useState<SelectedNamespace>(
        searchContext ? getSelectedNamespace(searchContext.namespace) : getSelectedNamespaceFromUser(authenticatedUser)
    )

    const visibilityRadioButtons = useMemo(() => getVisibilityRadioButtons(selectedNamespace.type), [selectedNamespace])

    const searchContextSpecPreview = isValidName ? (
        getSearchContextSpecPreview(selectedNamespace, name)
    ) : (
        <div className="text-danger">Invalid context name</div>
    )

    const [hasRepositoriesConfigChanged, setHasRepositoriesConfigChanged] = useState(false)
    const [repositoriesConfig, setRepositoriesConfig] = useState('')
    const onRepositoriesConfigChange = useCallback(
        (config: string, isInitialValue?: boolean) => {
            setRepositoriesConfig(config)
            if (!isInitialValue && config !== repositoriesConfig) {
                setHasRepositoriesConfigChanged(true)
            }
        },
        [repositoriesConfig, setRepositoriesConfig, setHasRepositoriesConfigChanged]
    )

    const hasChanges = useMemo(() => {
        if (!searchContext) {
            return (
                name.length > 0 ||
                description.length > 0 ||
                queryState.query.length > 0 ||
                visibility !== 'public' ||
                selectedNamespace.type !== 'user' ||
                hasRepositoriesConfigChanged
            )
        }
        return (
            searchContext.name !== name ||
            searchContext.description !== description ||
            searchContext.query !== queryState.query ||
            searchContextVisibility(searchContext) !== visibility ||
            hasRepositoriesConfigChanged
        )
    }, [description, name, searchContext, selectedNamespace, visibility, queryState, hasRepositoriesConfigChanged])

    const parseRepositories = useCallback(
        (): Observable<RepositoriesParseResult> =>
            of(parseConfig(repositoriesConfig)).pipe(
                switchMap(config => {
                    if (config === null) {
                        const configErrorResult: RepositoriesParseResult = {
                            type: 'errors',
                            errors: [
                                new Error('Invalid configuration format. Check for inline editor warnings and errors.'),
                            ],
                        }
                        return of(configErrorResult)
                    }
                    const repositoryNames = config.map(({ repository }) => repository)

                    if (repositoryNames.length === 0) {
                        return of({ type: 'repositories', repositories: [] } as RepositoriesParseResult)
                    }

                    return from(fetchRepositoriesByNames(repositoryNames)).pipe(
                        map(repositories => {
                            const repositoryNameToID = new Map(repositories.map(({ id, name }) => [name, id]))
                            const errors: Error[] = []
                            const validRepositories: SearchContextRepositoryRevisionsInput[] = []
                            for (const { repository, revisions } of config) {
                                const repositoryID = repositoryNameToID.get(repository)
                                if (repositoryID) {
                                    validRepositories.push({ repositoryID, revisions })
                                } else {
                                    errors.push(new Error(`Cannot find ${repository} repository.`))
                                }
                            }
                            const parseResult: RepositoriesParseResult =
                                errors.length > 0
                                    ? { type: 'errors', errors }
                                    : { type: 'repositories', repositories: validRepositories }
                            return parseResult
                        })
                    )
                })
            ),
        [repositoriesConfig]
    )

    const validateRepositories = useCallback(
        () =>
            parseRepositories().pipe(
                map(repositoriesOrErrors => (repositoriesOrErrors.type === 'errors' ? repositoriesOrErrors.errors : []))
            ),
        [parseRepositories]
    )

    const [submitRequest, searchContextOrError] = useEventObservable(
        useCallback(
            (submit: Observable<React.FormEvent<HTMLFormElement>>) =>
                submit.pipe(
                    tap(event => event.preventDefault()),
                    switchMap(() => {
                        const partialInput = {
                            name,
                            description,
                            public: visibility === 'public',
                            namespace: selectedNamespace.id,
                        }
                        if (contextType === 'static') {
                            return parseRepositories().pipe(
                                switchMap(repositoriesOrError => {
                                    if (repositoriesOrError.type === 'errors') {
                                        return throwError(() => createAggregateError(repositoriesOrError.errors))
                                    }
                                    return of(repositoriesOrError.repositories)
                                }),
                                map(repositories => ({ input: { ...partialInput, query: '' }, repositories }))
                            )
                        }
                        if (queryState.query.trim().length === 0) {
                            return throwError(() => new Error('Search query has to be non-empty.'))
                        }
                        return of({ input: { ...partialInput, query: queryState.query }, repositories: [] })
                    }),
                    switchMap(({ input, repositories }) =>
                        onSubmit(searchContext?.id, input, repositories).pipe(
                            startWith(LOADING),
                            catchError(error => [asError(error)]),
                            tap(successOrError => {
                                if (!isErrorLike(successOrError) && successOrError !== LOADING) {
                                    navigate('/contexts?order=updated-at-desc', { state: ALLOW_NAVIGATION })
                                }
                            })
                        )
                    ),
                    catchError(error => [asError(error)])
                ),
            [
                onSubmit,
                parseRepositories,
                name,
                description,
                queryState,
                visibility,
                selectedNamespace,
                navigate,
                searchContext,
                contextType,
            ]
        )
    )

    const onCancel = useCallback(() => {
        navigate('/contexts')
    }, [navigate])

    const [showDeleteModal, setShowDeleteModal] = useState(false)
    const toggleDeleteModal = useCallback(() => setShowDeleteModal(show => !show), [setShowDeleteModal])

    return (
        <Form onSubmit={submitRequest}>
            <Container className="mb-3">
                <div className="d-flex">
                    <div className="mr-2">
                        <div className="mb-2">Owner</div>
                        <SearchContextOwnerDropdown
                            isDisabled={!!searchContext}
                            selectedNamespace={selectedNamespace}
                            setSelectedNamespace={setSelectedNamespace}
                            authenticatedUser={authenticatedUser}
                        />
                    </div>
                    <Input
                        className="flex-1 mb-0"
                        inputClassName={styles.searchContextFormNameInput}
                        aria-labelledby="context-name-label"
                        label={<span className="font-weight-normal">Context name</span>}
                        data-testid="search-context-name-input"
                        value={name}
                        pattern="^[a-zA-Z0-9_\-\/\.]+$"
                        required={true}
                        maxLength={MAX_NAME_LENGTH}
                        onChange={event => {
                            setName(event.target.value)
                        }}
                    />
                </div>
                <div className="text-muted my-2">
                    <small>
                        The best context names are short and semantic. {MAX_NAME_LENGTH} characters max. Alphanumeric
                        and <kbd>.</kbd>
                        <kbd>_</kbd>
                        <kbd>/</kbd>
                        <kbd>-</kbd> characters only.
                    </small>
                </div>
                <div>
                    <div className={classNames('mb-1', styles.searchContextFormPreviewTitle)}>Preview</div>
                    {searchContextSpecPreview}
                </div>
                <hr aria-hidden={true} className={classNames('my-4', styles.searchContextFormDivider)} />
                <TextArea
                    label={
                        <>
                            Description <span className="text-muted">(optional)</span>
                        </>
                    }
                    message={
                        <span className="font-weight-normal">
                            <span>Markdown formatting is supported</span>
                            <span aria-hidden={true} className="px-1">
                                &middot;
                            </span>
                            <span>{MAX_DESCRIPTION_LENGTH - description.length} characters remaining</span>
                        </span>
                    }
                    className="w-100 mb-2"
                    data-testid="search-context-description-input"
                    maxLength={MAX_DESCRIPTION_LENGTH}
                    value={description}
                    rows={5}
                    onChange={event => {
                        const value = event.target.value
                        if (value.length <= MAX_DESCRIPTION_LENGTH) {
                            setDescription(event.target.value)
                        }
                    }}
                />
                <div className={classNames('mt-3', styles.searchContextFormVisibility)}>
                    <div className="mb-3">Visibility</div>
                    {visibilityRadioButtons.map((radio, index) => (
                        <React.Fragment key={radio.visibility}>
                            <RadioButton
                                id={`visibility_${index}`}
                                className={styles.searchContextFormRadio}
                                name="visibility"
                                value={radio.visibility}
                                checked={visibility === radio.visibility}
                                required={true}
                                onChange={() => setVisibility(radio.visibility)}
                                label={
                                    <div>
                                        <strong className={styles.searchContextFormVisibilityTitle}>
                                            {radio.title}
                                        </strong>
                                    </div>
                                }
                            />
                            <div className="ml-4 mb-2">
                                <small className="text-muted">{radio.description}</small>
                            </div>
                        </React.Fragment>
                    ))}
                </div>
                <hr aria-hidden={true} className={classNames('my-4', styles.searchContextFormDivider)} />
                <div>
                    <div className="mb-1">Choose repositories and revisions</div>
                    <div className="text-muted mb-3">
                        For a dynamic set of repositories and revisions, such as for project or team repos, use a{' '}
                        <Link target="_blank" rel="noopener" to="/help/code_search/how-to/search_contexts">
                            search query
                        </Link>
                        . For a static set, use the JSON configuration.
                    </div>
                    <div>
                        <RadioButton
                            id="search-context-type-dynamic"
                            className={styles.searchContextFormRadio}
                            name="search-context-type"
                            value="dynamic"
                            checked={contextType === 'dynamic'}
                            required={true}
                            onChange={() => setContextType('dynamic')}
                            label={<>Search query</>}
                        />
                        <div className={styles.searchContextFormQuery} data-testid="search-context-dynamic-query">
                            <LazyQueryInputFormControl
                                patternType={SearchPatternType.regexp}
                                isSourcegraphDotCom={isSourcegraphDotCom}
                                caseSensitive={true}
                                queryState={queryState}
                                onChange={setQueryState}
                                preventNewLine={false}
                            />
                        </div>
                        <div className={classNames(styles.searchContextFormQueryLabel, 'text-muted')}>
                            <small>
                                Valid filters: <SyntaxHighlightedSearchQuery query="repo" />,{' '}
                                <SyntaxHighlightedSearchQuery query="rev" />,{' '}
                                <SyntaxHighlightedSearchQuery query="file" /> ,{' '}
                                <SyntaxHighlightedSearchQuery query="lang" />,{' '}
                                <SyntaxHighlightedSearchQuery query="case" />,{' '}
                                <SyntaxHighlightedSearchQuery query="fork" />, and{' '}
                                <SyntaxHighlightedSearchQuery query="visibility" />.{' '}
                                <SyntaxHighlightedSearchQuery query="OR" /> and{' '}
                                <SyntaxHighlightedSearchQuery query="AND" /> expressions are also allowed.
                            </small>
                        </div>
                    </div>
                    <div className="mt-3">
                        <RadioButton
                            id="search-context-type-static"
                            className={styles.searchContextFormRadio}
                            name="search-context-type"
                            value="static"
                            checked={contextType === 'static'}
                            required={true}
                            onChange={() => setContextType('static')}
                            label={<> JSON configuration </>}
                        />
                        <div className={styles.searchContextFormStaticConfig}>
                            <SearchContextRepositoriesFormArea
                                {...props}
                                isLightTheme={isLightTheme}
                                onChange={onRepositoriesConfigChange}
                                validateRepositories={validateRepositories}
                                repositories={searchContext?.repositories}
                                telemetryRecorder={platformContext.telemetryRecorder}
                            />
                        </div>
                    </div>
                </div>
                {isErrorLike(searchContextOrError) && (
                    <Alert className="mt-3" variant="danger">
                        Failed to create search context: {searchContextOrError.message}
                    </Alert>
                )}
            </Container>
            <div className="d-flex">
                <Button
                    type="submit"
                    className="mr-2 test-search-context-submit-button"
                    data-testid="search-context-submit-button"
                    disabled={searchContextOrError && searchContextOrError === LOADING}
                    variant="primary"
                >
                    {!searchContext ? 'Create search context' : 'Save'}
                </Button>
                <Button onClick={onCancel} outline={true} variant="secondary">
                    Cancel
                </Button>
                {searchContext && (
                    <>
                        <div className="flex-grow-1" />
                        <Button
                            data-testid="search-context-delete-button"
                            onClick={toggleDeleteModal}
                            outline={true}
                            variant="danger"
                        >
                            Delete
                        </Button>
                        <DeleteSearchContextModal
                            isOpen={showDeleteModal}
                            deleteSearchContext={deleteSearchContext}
                            searchContext={searchContext}
                            toggleDeleteModal={toggleDeleteModal}
                            platformContext={platformContext}
                        />
                    </>
                )}
            </div>
            <AwayPrompt
                header="Discard unsaved changes?"
                message="All unsaved changes will be lost."
                button_ok_text="Discard"
                when={hasChanges}
            />
        </Form>
    )
}
