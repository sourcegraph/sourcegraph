import { integrationTestContext } from '../../../../../shared/src/api/integration-test/testHelpers'
import { createPlatformContext } from '../../platform/context'
import { DEFAULT_SOURCEGRAPH_URL, getAssetsURL } from '../../util/context'

describe('Extension host', () => {
    it('Blocks GraphQL requests from extensions if they risk leaking private information to the public sourcegraph.com instance', async () => {
        window.SOURCEGRAPH_URL = DEFAULT_SOURCEGRAPH_URL

        const { requestGraphQL } = createPlatformContext(
            {
                urlToFile: () => '',
                getContext: () => ({ rawRepoName: 'foo', privateRepository: true }),
            },
            {
                sourcegraphURL: DEFAULT_SOURCEGRAPH_URL,
                assetsURL: getAssetsURL(DEFAULT_SOURCEGRAPH_URL),
            },
            false
        )

        const { extensionAPI } = await integrationTestContext({ requestGraphQL })

        return expect(
            extensionAPI.graphQL.execute(
                `
                        query ResolveRepo($repoName: String!) {
                            repository(name: $repoName) {
                                url
                            }
                        }
                    `,
                { repoName: 'foo' }
            )
        ).rejects.toMatchObject({
            message:
                'A ResolveRepo GraphQL request to the public Sourcegraph.com was blocked because the current repository is private.',
        })
    })
})
