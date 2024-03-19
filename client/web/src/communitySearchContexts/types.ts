import type { SearchPatternType } from '../graphql-operations'

export interface ExampleQuery {
    title: string
    description?: string
    query: string
    patternType: SearchPatternType
}

export type CommunitySearchContextSpecs =
    | 'backstage'
    | 'chakraui'
    | 'cncf'
    | 'temporalio'
    | 'o3de'
    | 'stackstorm'
    | 'kubernetes'
    | 'stanford'
    | 'julia'

export interface CommunitySearchContextMetadata {
    /**
     * The title of the community search context. This is displayed on the search homepage, and is typically prose. E.g. Refactor python 2 to 3.
     */
    title: string
    /**
     * The name of the community search context, must match the community search context name as configured in settings. E.g. python2-to-3.
     */
    spec: CommunitySearchContextSpecs
    /**
     * The URL pathname for the community search context page.
     */
    url: string
    /**
     * A list of example queries using the community search context. Don't include the `context:name` portion of the query. It will be automatically added.
     */
    examples: ExampleQuery[]
    /**
     * A description of the community search context to be displayed on the page.
     */
    description: JSX.Element | string
    /**
     * Base64 data uri to an icon.
     */
    homepageIcon: string
    /**
     * A description when displayed on the search homepage.
     */
    homepageDescription: string
    /**
     * Whether to display this in a minimal community search context page, without examples/repositories/descriptions below the search bar.
     */
    lowProfile?: boolean
}
