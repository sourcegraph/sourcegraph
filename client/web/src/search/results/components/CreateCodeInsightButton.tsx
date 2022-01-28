import React from 'react'

import { SearchPatternTypeProps } from '@sourcegraph/search'
import { ButtonLink } from '@sourcegraph/wildcard'

import { AuthenticatedUser } from '../../../auth'
import { CodeInsightsIcon } from '../../../insights/Icons'

interface CreateCodeInsightButtonProps extends SearchPatternTypeProps {
    /** Search query string. */
    query?: string

    /** The currently authenticated user or null. */
    authenticatedUser: Pick<AuthenticatedUser, 'id'> | null

    /** Whether the code insights feature flag is enabled. */
    enableCodeInsights?: boolean
}

/**
 * Displays code insights creation button from search query.
 *
 * Basically it navigates user to search based insight creation UI with
 * prefilled repositories and data series query field according to
 * search page query.
 */
export const CreateCodeInsightButton: React.FunctionComponent<CreateCodeInsightButtonProps> = props => {
    if (!props.enableCodeInsights || !props.query || !props.authenticatedUser) {
        return null
    }

    const searchParameters = new URLSearchParams()
    searchParameters.set('query', `${props.query} patterntype:${props.patternType}`)
    const toURL = `/insights/create/search?${searchParameters.toString()}`

    return (
        <li data-tooltip="Create Insight based on this search query" data-delay={10000} className="nav-item mr-2">
            <ButtonLink to={toURL} size="sm" outline={true} variant="secondary">
                <CodeInsightsIcon className="icon-inline mr-1" />
                Create Insight
            </ButtonLink>
        </li>
    )
}
