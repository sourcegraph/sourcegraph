import { error } from '@sveltejs/kit'

import { getGraphQLClient, mapOrThrow } from '$lib/graphql'

import type { PageLoad } from './$types'
import { communityPageConfigs } from './config'
import { CommunitySearchPage_SearchContext } from './page.gql'

export const load: PageLoad = ({ params }) => {
    const community = params.community.toLowerCase()
    const config = communityPageConfigs[community]
    if (!config) {
        // This should never happen because the router won't even load this page if the
        // parameter is not a known community name.
        error(404, `Community search context not found: ${params.community}`)
    }

    return {
        ...config,
        community,
        repositories: getGraphQLClient()
            .query(CommunitySearchPage_SearchContext, {
                spec: config.spec,
            })
            .then(mapOrThrow(({ data }) => data?.searchContextBySpec?.repositories ?? [])),
    }
}
