import React, { FC, useEffect, useState } from 'react'

import { mdiHelpCircleOutline } from '@mdi/js'

import { Icon, Select, Tooltip, Input, Button, Label, Form } from '@sourcegraph/wildcard'

export interface SimpleSearchProps {
    onSimpleSearchUpdate
    onSubmit
    searchContext?
}

const languages = ['JavaScript', 'TypeScript', 'Java', 'C++', 'Python', 'Go', 'C#', 'Ruby']

const getQuery = ({
    repoPattern,
    repoNames,
    filePaths,
    useForks,
    literalContent,
    regexpContent,
    languageFilter,
    useArchive,
    searchContext,
}): string => {
    // build query
    const terms: string[] = []

    if (searchContext?.length > 0) {
        terms.push(`context:${searchContext}`)
    }

    if (repoPattern?.length > 0) {
        terms.push(`repo:${repoPattern}`)
    }
    if (repoNames?.length > 0) {
        terms.push(`repo:${repoNames}$`)
    }
    if (filePaths?.length > 0) {
        terms.push(`file:${filePaths}`)
    }
    if (useForks === 'yes' || useForks === 'only') {
        terms.push(`fork:${useForks}`)
    }
    if (useArchive === 'yes' || useArchive === 'only') {
        terms.push(`archived:${useArchive}`)
    }
    if (languageFilter?.length > 0 && languageFilter !== 'Choose') {
        terms.push(`lang:${languageFilter}`)
    }

    // do these last

    if (literalContent?.length > 0) {
        terms.push(literalContent)
    } else if (regexpContent?.length > 0) {
        terms.push(`/${regexpContent}/`)
    }

    return terms.join(' ')
}

export const CodeSearchSimpleSearch: FC<SimpleSearchProps> = ({ onSimpleSearchUpdate, onSubmit }) => {
    const [repoPattern, setRepoPattern] = useState<string>('')
    const [repoNames, setRepoNames] = useState<string>('')
    const [filePaths, setFilePaths] = useState<string>('')
    const [useForks, setUseForks] = useState<string>('')
    const [useArchive, setUseArchive] = useState<string>('')
    const [languageFilter, setLanguageFilter] = useState<string>('')
    const [searchContext, setSearchContext] = useState<string>('global')

    const [literalContent, setLiteralContent] = useState<string>('')
    const [regexpContent, setRegexpContent] = useState<string>('')

    useEffect(() => {
        // Update the query whenever any of the other fields change
        const updatedQuery = getQuery({
            repoPattern,
            repoNames,
            filePaths,
            useForks,
            literalContent,
            regexpContent,
            languageFilter,
            useArchive,
            searchContext,
        })
        onSimpleSearchUpdate(updatedQuery)
    }, [
        repoPattern,
        repoNames,
        filePaths,
        useForks,
        literalContent,
        regexpContent,
        languageFilter,
        useArchive,
        searchContext,
    ])

    return (
        <div>
            <Form className="mt-4" onSubmit={onSubmit}>
                <div id="contentFilterSection">
                    <div className="form-group row">
                        <Label htmlFor="repoName" className="col-4 col-form-label">
                            Match literal string
                            <Tooltip content="Search for matching content with an exact match.">
                                <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                            </Tooltip>
                        </Label>

                        <div className="col-8">
                            <div className="input-group">
                                <Input
                                    disabled={regexpContent?.length > 0}
                                    id="repoName"
                                    name="repoName"
                                    placeholder="class CustomerManager"
                                    type="text"
                                    onChange={event => setLiteralContent(event.target.value)}
                                />
                            </div>
                        </div>
                    </div>

                    <div className="form-group row">
                        <Label htmlFor="repoName" className="col-4 col-form-label">
                            Match regular expression
                            <Tooltip content="Search for matching content using a regular expression.">
                                <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                            </Tooltip>
                        </Label>

                        <div className="col-8">
                            <div className="input-group">
                                <Input
                                    disabled={literalContent?.length > 0}
                                    id="repoName"
                                    name="repoName"
                                    placeholder="class \w*Manager"
                                    type="text"
                                    onChange={event => setRegexpContent(event.target.value)}
                                />
                            </div>
                        </div>
                    </div>
                </div>

                <hr className="mt-4 mb-4" />

                <div id="repoFilterSection">
                    <div className="form-group row">
                        <Label htmlFor="repoName" className="col-4 col-form-label">
                            In these repos
                            <Tooltip content="Match repository names exactly.">
                                <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                            </Tooltip>
                        </Label>

                        <div className="col-8">
                            <div className="input-group">
                                <Input
                                    id="repoName"
                                    name="repoName"
                                    placeholder="sourcegraph/sourcegraph"
                                    type="text"
                                    onChange={event => setRepoNames(event.target.value)}
                                />
                            </div>
                        </div>
                    </div>

                    <div className="form-group row">
                        <Label htmlFor="repoNamePatterns" className="col-4 col-form-label">
                            In matching repos
                            <Tooltip content="Use a regular expression pattern to match against repository names.">
                                <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                            </Tooltip>
                        </Label>
                        <div className="col-8">
                            <Input
                                id="repoNamePatterns"
                                name="repoNamePatterns"
                                placeholder="sourcegraph.*"
                                type="text"
                                onChange={event => setRepoPattern(event.target.value)}
                            />
                        </div>
                    </div>

                    <div className="form-group row">
                        <Label htmlFor="searchForks" className="col-4 col-form-label">
                            Search over repository forks?
                            <Tooltip content="Choose an option to include or exclude forks from the search, or search only over forks.">
                                <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                            </Tooltip>
                        </Label>
                        <div className="col-2">
                            <Select
                                id="searchForks"
                                name="searchForks"
                                onChange={event => setUseForks(event.target.value)}
                            >
                                <option value="no">No</option>
                                <option value="yes">Yes</option>
                                <option value="only">Only forks</option>
                            </Select>
                        </div>

                        <Label htmlFor="searchArchive" className="col-4 col-form-label">
                            Search over archived repositories?
                            <Tooltip content="Choose an option to include or exclude archived repos from the search, or search only over archived repos.">
                                <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                            </Tooltip>
                        </Label>
                        <div className="col-2">
                            <Select
                                id="searchArchive"
                                name="searchArchive"
                                onChange={event => setUseArchive(event.target.value)}
                            >
                                <option value="no">No</option>
                                <option value="yes">Yes</option>
                                <option value="only">Only archives</option>
                            </Select>
                        </div>
                    </div>
                </div>

                <hr className="mt-4 mb-4" />

                <div className="form-group row">
                    <Label htmlFor="text" className="col-4 col-form-label">
                        In matching file paths
                        <Tooltip content="Use a regular expression pattern to match against file paths, for example sourcegraph/.*/internal">
                            <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                        </Tooltip>
                    </Label>
                    <div className="col-8">
                        <Input
                            id="text"
                            name="text"
                            type="text"
                            placeholder="enterprise/.*"
                            onChange={event => setFilePaths(event.target.value)}
                        />
                    </div>
                </div>

                <div className="form-group row">
                    <Label htmlFor="searchLang" className="col-4 col-form-label">
                        Which programming language?
                        <Tooltip content="Only match files for a given programming language.">
                            <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                        </Tooltip>
                    </Label>
                    <div className="col-8">
                        <Select
                            id="searchLang"
                            name="searchLang"
                            onChange={event => setLanguageFilter(event.target.value)}
                        >
                            <option hidden>Any</option>
                            {languages.map(lang => (
                                <option value={lang}>{lang}</option>
                            ))}
                        </Select>
                    </div>
                </div>

                <div className="form-group row">
                    <Label htmlFor="searchContext" className="col-4 col-form-label">
                        Search context
                        <Tooltip content="Only match files inside a search context. A search context is a Sourcegraph entity to provide shareable and repeatable filters, such as common sets of repositories. The global context  will search over all code on Sourcegraph.">
                            <Icon className="ml-2" svgPath={mdiHelpCircleOutline} />
                        </Tooltip>
                    </Label>
                    <div className="col-8">
                        <Input
                            value={searchContext}
                            id="text"
                            name="text"
                            type="text"
                            onChange={event => setSearchContext(event.target.value)}
                        />
                    </div>
                </div>

                <div className="form-group row">
                    <div className="offset-4 col-8">
                        <Button variant="primary" name="submit" type="submit" className="btn btn-primary">
                            Submit
                        </Button>
                    </div>
                </div>
            </Form>
        </div>
    )
}
