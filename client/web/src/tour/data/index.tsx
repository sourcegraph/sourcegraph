import { mdiCheckCircle, mdiMagnify, mdiPuzzleOutline, mdiShieldSearch, mdiNotebook, mdiCursorPointer } from '@mdi/js'

import { TourLanguage, TourTaskType } from '@sourcegraph/shared/src/settings/temporary'
import { Code, Icon } from '@sourcegraph/wildcard'

/**
 * Tour tasks for non-authenticated users
 */
export const visitorsTasks: TourTaskType[] = [
    {
        title: 'Code search use cases',
        steps: [
            {
                id: 'SymbolsSearch',
                label: 'Search multiple repos',
                action: {
                    type: 'link',
                    value: {
                        [TourLanguage.C]:
                            '/search?q=context:global+repo:torvalds/.*+lang:c+-file:.*/testing+magic&patternType=literal',
                        [TourLanguage.Go]:
                            '/search?q=context:global+r:google/+lang:go+-file:test+//+TODO&patternType=literal',
                        [TourLanguage.Java]:
                            '/search?q=context:global+r:github.com/square/+lang:java+-file:test+GiftCard&patternType=literal',
                        [TourLanguage.Javascript]:
                            '/search?q=context:global+r:react+lang:JavaScript+-file:test+createPortal&patternType=literal',
                        [TourLanguage.Php]:
                            '/search?q=context:global+repo:laravel+lang:php+-file:test+login%28&patternType=regexp&case=yes',
                        [TourLanguage.Python]:
                            '/search?q=context:global+r:aws/+lang:python+file:mock+def+test_patch&patternType=regexp&case=yes',
                        [TourLanguage.Typescript]:
                            '/search?q=context:global+r:react+lang:typescript+-file:test+createPortal%28&patternType=regexp&case=yes',
                    },
                },
                info: (
                    <>
                        <strong>Reference code in multiple repositories</strong>
                        <br />
                        The repo: query allows searching in multiple repositories matching a term. Use it to reference
                        all of your projects or find open source examples.
                    </>
                ),
            },
            {
                id: 'CommitsSearch',
                label: 'Find changes in commits',
                action: {
                    type: 'link',
                    value: {
                        [TourLanguage.C]:
                            '/search?q=context:global+repo:%5Egithub%5C.com/chref/doh%24+file:%5Edoh%5C.c+type:commit+option&patternType=literal',
                        [TourLanguage.Go]:
                            '/search?q=context:global+repo:%5Egitlab%5C.com/sourcegraph/sourcegraph%24+type:commit+bump&patternType=literal',
                        [TourLanguage.Java]:
                            '/search?q=context:global+repo:sourcegraph-testing/.*+type:commit+lang:java+version&patternType=literal',
                        [TourLanguage.Javascript]:
                            '/search?q=context:global+repo:%5Egithub%5C.com/hakimel/reveal%5C.js%24+type:commit+error&patternType=literal',
                        [TourLanguage.Php]:
                            '/search?q=context:global+repo:square/connect-api-examples+type:commit+version&patternType=regexp&case=yes',
                        [TourLanguage.Python]:
                            '/search?q=context:global+r:elastic/elasticsearch+lang:python+type:commit+request_timeout&patternType=regexp&case=yes',
                        [TourLanguage.Typescript]:
                            '/search?q=context:global+repo:%5Egitlab%5C.com/sourcegraph/sourcegraph%24+type:commit+bump&patternType=literal',
                    },
                },
                info: (
                    <>
                        <strong>Find changes in commits</strong>
                        <br />
                        Quickly find commits in history, then browse code from the commit, without checking out the
                        branch.
                    </>
                ),
            },
            {
                id: 'DiffSearch',
                label: 'Search diffs for removed code',
                action: {
                    type: 'link',
                    value: {
                        [TourLanguage.C]:
                            '/search?q=context:global+repo:chref/doh+type:diff+select:commit.diff.removed+mode&patternType=literal',
                        [TourLanguage.Go]:
                            '/search?q=context:global+repo:%5Egitlab%5C.com/sourcegraph/sourcegraph%24+type:diff+lang:go+select:commit.diff.removed+NameSpaceOrgId&patternType=literal',
                        [TourLanguage.Java]:
                            '/search?q=context:global+repo:sourcegraph-testing/sg-hadoop+lang:java+type:diff+select:commit.diff.removed+getConf&patternType=literal',
                        [TourLanguage.Javascript]:
                            '/search?q=context:global+repo:sourcegraph/sourcegraph%24+lang:javascript+-file:test+type:diff+select:commit.diff.removed+promise&patternType=literal',
                        [TourLanguage.Php]:
                            '/search?q=context:global+repo:laravel/laravel.*+lang:php++type:diff+select:commit.diff.removed+password&patternType=regexp&case=yes',
                        [TourLanguage.Python]:
                            '/search?q=context:global+repo:pallets/+lang:python+type:diff+select:commit.diff.removed+password&patternType=regexp&case=yes',
                        [TourLanguage.Typescript]:
                            '/search?q=context:global+repo:sourcegraph/sourcegraph%24+lang:typescript+type:diff+select:commit.diff.removed+authenticatedUser&patternType=regexp&case=yes',
                    },
                },
                info: (
                    <>
                        <strong>Searching diffs for removed code</strong>
                        <br />
                        Find removed code without browsing through history or trying to remember which file it was in.
                    </>
                ),
            },
        ],
    },
    {
        title: 'The power of code intel',
        steps: [
            {
                id: 'FindReferences',
                label: 'Find references',
                action: {
                    type: 'link',
                    value: {
                        [TourLanguage.C]: '/github.com/torvalds/linux/-/blob/arch/arm/kernel/atags_compat.c?L196:8',
                        [TourLanguage.Go]:
                            '/github.com/sourcegraph/sourcegraph/-/blob/internal/featureflag/featureflag.go?L9:6',
                        [TourLanguage.Java]:
                            '/github.com/square/okhttp/-/blob/samples/guide/src/main/java/okhttp3/recipes/PrintEvents.java?L126:27',
                        [TourLanguage.Javascript]:
                            '/github.com/mozilla/pdf.js/-/blob/src/display/display_utils.js?L261:16',
                        [TourLanguage.Php]:
                            '/github.com/square/connect-api-examples/-/blob/connect-examples/v1/php/payments-report.php?L164:32',
                        [TourLanguage.Python]: '/github.com/google-research/bert/-/blob/extract_features.py?L152:7',
                        [TourLanguage.Typescript]:
                            '/github.com/sourcegraph/sourcegraph/-/blob/client/shared/src/search/query/hover.ts?L202:14',
                    },
                },
                info: (
                    <>
                        <strong>FIND REFERENCES</strong>
                        <br />
                        Hover over a token in the highlighted line to open code intel, then click ‘Find References’ to
                        locate all calls of this code.
                    </>
                ),
                completeAfterEvents: ['findReferences'],
            },
            {
                id: 'GoToDefinition',
                label: 'Go to a definition',
                info: (
                    <>
                        <strong>GO TO DEFINITION</strong>
                        <br />
                        Hover over a token in the highlighted line to open code intel, then click ‘Go to definition’ to
                        locate a token definition.
                    </>
                ),
                completeAfterEvents: ['goToDefinition'],
                action: {
                    type: 'link',
                    value: {
                        [TourLanguage.C]: '/github.com/torvalds/linux/-/blob/arch/arm/kernel/bios32.c?L417:8',
                        [TourLanguage.Go]:
                            '/github.com/sourcegraph/sourcegraph/-/blob/internal/repos/observability.go?L192:22',
                        [TourLanguage.Java]:
                            '/github.com/square/okhttp/-/blob/samples/guide/src/main/java/okhttp3/recipes/CustomCipherSuites.java?L132:14',
                        [TourLanguage.Javascript]: '/github.com/mozilla/pdf.js/-/blob/src/pdf.js?L101:5',
                        [TourLanguage.Php]:
                            '/github.com/square/connect-api-examples/-/blob/connect-examples/v1/php/payments-report.php?L164:32',
                        [TourLanguage.Python]:
                            '/github.com/netdata/netdata@1c2465c816071ff767982116a4b19bad1d8b0c82/-/blob/collectors/python.d.plugin/python_modules/bases/charts.py?L303:48',
                        [TourLanguage.Typescript]:
                            '/github.com/sourcegraph/sourcegraph/-/blob/client/shared/src/search/query/parserFuzz.ts?L295:37',
                    },
                },
            },
        ],
    },
    {
        title: 'Tools to improve workflow',
        steps: [
            {
                id: 'EditorExtensions',
                label: 'IDE extensions',
                action: { type: 'new-tab-link', value: 'https://docs.sourcegraph.com/integration/editor' },
            },
            {
                id: 'BrowserExtensions',
                label: 'Browser extensions',
                action: { type: 'new-tab-link', value: 'https://docs.sourcegraph.com/integration/browser_extension' },
            },
        ],
    },
    {
        title: 'Install or sign up',
        steps: [
            {
                id: 'InstallOrSignUp',
                label: 'Get free trial',
                action: {
                    type: 'new-tab-link',
                    value: 'https://about.sourcegraph.com',
                },
                // This is done to mimic user creating an account, and signed in there is a different tour
                completeAfterEvents: ['non-existing-event'],
            },
        ],
    },
]

/**
 * Tour tasks for non-authenticated users
 */
export const visitorsTasksWithNotebook: TourTaskType[] = [
    {
        dataAttributes: {
            order: 1,
        },
        steps: [
            {
                id: 'FindAcrossYourReposNotebook',
                label: 'Find code across all your repos',
                action: {
                    type: 'new-tab-link',
                    value: 'https://sourcegraph.com/notebooks/Tm90ZWJvb2s6MTM=',
                },
            },
        ],
    },
    {
        dataAttributes: {
            order: 2,
        },
        steps: [
            {
                id: 'SearchAndReviewCommitsNotebook',
                label: 'Search & review commits',
                action: {
                    type: 'new-tab-link',
                    value: 'https://sourcegraph.com/notebooks/Tm90ZWJvb2s6MTI=',
                },
            },
        ],
    },
    {
        dataAttributes: {
            order: 3,
        },
        steps: [
            {
                id: 'RefineQueriesByFilteringNotebook',
                label: 'Refine queries by filtering',
                action: {
                    type: 'new-tab-link',
                    value: 'https://sourcegraph.com/notebooks/Tm90ZWJvb2s6MTQ=',
                },
            },
        ],
    },
    {
        dataAttributes: {
            order: 4,
        },
        steps: [
            {
                id: 'StructuralSearchBasicsNotebook',
                label: 'Structural search basics',
                action: {
                    type: 'new-tab-link',
                    value: 'https://sourcegraph.com/notebooks/Tm90ZWJvb2s6Mzk4',
                },
            },
        ],
    },
]

export const visitorsTasksWithNotebookExtraTask: TourTaskType = {
    title: 'All done!',
    icon: (
        <Icon
            className="text-success"
            svgPath={mdiCheckCircle}
            inline={false}
            aria-hidden={true}
            height="2.3rem"
            width="2.3rem"
        />
    ),
    steps: [
        {
            id: 'InstallOrSignUp',
            label: 'Get free trial',
            action: {
                type: 'new-tab-link',
                variant: 'button-primary',
                value: 'https://signup.sourcegraph.com',
            },
            // This is done to mimic user creating an account, and signed in there is a different tour
            completeAfterEvents: ['non-existing-event'],
        },
    ],
}

/**
 * Tour tasks for authenticated users. Extended/all use-cases.
 */
export const authenticatedTasks: TourTaskType[] = [
    {
        title: 'Reuse existing code',
        icon: <Icon svgPath={mdiMagnify} inline={false} aria-hidden={true} height="2.3rem" width="2.3rem" />,
        steps: [
            {
                id: 'ReuseExistingCode',
                label: 'Discover and learn how to use existing libraries.',
                action: {
                    type: 'link',
                    value: {
                        [TourLanguage.C]: '/search?q=context:global+memcpy(+-file:test+lang:c+&patternType=regexp',
                        [TourLanguage.Go]:
                            '/search?q=context:global+repo:^github.com/golang/go%24+ReadResponse(+-file:test.go&patternType=regexp',
                        [TourLanguage.Java]:
                            '/search?q=context:global+repo:github.com/square/+lang:java+-file:test+GiftCard&patternType=literal',
                        [TourLanguage.Javascript]:
                            '/search?q=context:global+repo:react+lang:JavaScript+-file:test+createPortal&patternType=literal',
                        [TourLanguage.Php]:
                            '/search?q=context:global+repo:laravel+lang:php+-file:test+login%28&patternType=regexp&case=yes',
                        [TourLanguage.Python]:
                            '/search?q=context:global+repo:^github.com/pandas-dev/pandas%24++lang:python+-file:test+pd.DataFrame(&patternType=literal&case=yes',
                        [TourLanguage.Typescript]:
                            '/search?q=context:global+repo:react+lang:typescript+-file:test+createPortal%28&patternType=regexp&case=yes',
                    },
                },
                info: (
                    <>
                        <strong>Discover code across multiple repositories</strong>
                        <br />
                        The <Code>repo:</Code> keyword allows searching in multiple repositories matching a term. Use it
                        to reference all of your projects or find open source examples.
                    </>
                ),
            },
        ],
    },
    {
        title: 'Install an IDE extension',
        icon: <Icon svgPath={mdiPuzzleOutline} inline={false} aria-hidden={true} height="2.3rem" width="2.3rem" />,
        steps: [
            {
                id: 'InstallIDEExtension',
                label: 'Integrate Sourcegraph with your favorite IDE',
                action: {
                    type: 'new-tab-link',
                    value: 'https://docs.sourcegraph.com/integration/editor?utm_medium=direct-traffic&utm_source=in-product&utm_content=getting-started',
                },
            },
        ],
    },
    {
        title: 'Find and fix vulnerabilities',
        icon: <Icon svgPath={mdiShieldSearch} inline={false} aria-hidden={true} height="2.3rem" width="2.3rem" />,
        steps: [
            {
                id: 'WatchVideo',
                label: 'Watch the 60 second video',
                action: { type: 'video', value: 'https://www.youtube.com/embed/13OqKPXqZXo' },
            },
            {
                id: 'DiffSearch',
                label: 'Find problematic code in diffs',
                action: {
                    type: 'link',
                    value: {
                        [TourLanguage.C]:
                            '/search?q=context:global+repo:chref/doh+type:diff+select:commit.diff.removed+mode&patternType=literal',
                        [TourLanguage.Go]:
                            '/search?q=context:global+repo:%5Egitlab%5C.com/sourcegraph/sourcegraph%24+type:diff+lang:go+select:commit.diff.removed+NameSpaceOrgId&patternType=literal',
                        [TourLanguage.Java]:
                            '/search?q=context:global+repo:sourcegraph-testing/sg-hadoop+lang:java+type:diff+select:commit.diff.removed+getConf&patternType=literal',
                        [TourLanguage.Javascript]:
                            '/search?q=context:global+repo:sourcegraph/sourcegraph%24+lang:javascript+-file:test+type:diff+select:commit.diff.removed+promise&patternType=literal',
                        [TourLanguage.Php]:
                            '/search?q=context:global+repo:laravel/laravel.*+lang:php++type:diff+select:commit.diff.removed+password&patternType=regexp&case=yes',
                        [TourLanguage.Python]:
                            '/search?q=context:global+repo:pallets/+lang:python+type:diff+select:commit.diff.removed+password&patternType=regexp&case=yes',
                        [TourLanguage.Typescript]:
                            '/search?q=context:global+repo:sourcegraph/sourcegraph%24+lang:typescript+type:diff+select:commit.diff.removed+authenticatedUser&patternType=regexp&case=yes',
                    },
                },
                info: (
                    <>
                        <strong>Searching diffs for removed code</strong>
                        <br />
                        Find removed code without browsing through history or trying to remember which file it was in.
                    </>
                ),
            },
        ],
    },
    {
        title: 'Respond to incidents',
        icon: <Icon svgPath={mdiNotebook} inline={false} aria-hidden={true} height="2.3rem" width="2.3rem" />,
        steps: [
            {
                id: 'Notebook',
                label: 'Document post mortems using search notebooks',
                action: {
                    type: 'new-tab-link',
                    value: 'https://sourcegraph.com/notebooks/Tm90ZWJvb2s6MQ==',
                },
            },
        ],
    },
    {
        title: 'Understand a new codebase',
        icon: <Icon svgPath={mdiCursorPointer} inline={false} aria-hidden={true} height="2.3rem" width="2.3rem" />,
        steps: [
            {
                id: 'PowerCodeNav',
                label: 'Get IDE-like code navigation features across many repositories',
                action: {
                    type: 'link',
                    value: {
                        [TourLanguage.C]: '/github.com/torvalds/linux/-/blob/arch/arm/kernel/atags_compat.c?L196:8',
                        [TourLanguage.Go]:
                            '/github.com/sourcegraph/sourcegraph/-/blob/internal/featureflag/featureflag.go?L9:6',
                        [TourLanguage.Java]:
                            '/github.com/square/okhttp/-/blob/samples/guide/src/main/java/okhttp3/recipes/PrintEvents.java?L126:27',
                        [TourLanguage.Javascript]:
                            '/github.com/mozilla/pdf.js/-/blob/src/display/display_utils.js?L261:16',
                        [TourLanguage.Php]:
                            '/github.com/square/connect-api-examples/-/blob/connect-examples/v1/php/payments-report.php?L164:32',
                        [TourLanguage.Python]: '/github.com/google-research/bert/-/blob/extract_features.py?L152:7',
                        [TourLanguage.Typescript]:
                            '/github.com/sourcegraph/sourcegraph/-/blob/client/shared/src/search/query/hover.ts?L202:14',
                    },
                },
                info: (
                    <>
                        <strong>Find References</strong>
                        <br />
                        Hover over a token in the highlighted line to open code intel, then click ‘Find References’ to
                        locate all calls of this code.
                    </>
                ),
                completeAfterEvents: ['findReferences'],
            },
        ],
    },
]

/**
 * Tour extra tasks for authenticated users.
 */
export const authenticatedExtraTask: TourTaskType = {
    title: 'All done!',
    icon: (
        <Icon
            className="text-success"
            svgPath={mdiCheckCircle}
            inline={false}
            aria-hidden={true}
            height="2.3rem"
            width="2.3rem"
        />
    ),
    steps: [
        {
            id: 'RestartTour',
            label: 'You can restart the tour to go through the steps again.',
            action: { type: 'restart', value: 'Restart tour' },
        },
    ],
}
