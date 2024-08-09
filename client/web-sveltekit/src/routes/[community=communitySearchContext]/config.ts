import { SearchPatternType } from '$lib/graphql-types'
import type { Community } from '$lib/search/communityPages'

interface ExampleQuery {
    title: string
    description?: string
    query: string
    patternType: SearchPatternType
}

interface CommunitySearchContextMetadata {
    /**
     * The title of the community search context. This is displayed on the search homepage, and is typically prose. E.g. Refactor python 2 to 3.
     */
    title: string
    /**
     * The name of the community search context, must match the community search context name as configured in settings. E.g. python2-to-3.
     */
    spec: string
    /**
     * A list of example queries using the community search context. Don't include the `context:name` portion of the query. It will be automatically added.
     */
    examples: ExampleQuery[]
    /**
     * A description of the community search context to be displayed on the page.
     */
    description: string
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

export const communityPageConfigs: Record<string, CommunitySearchContextMetadata> = {
    backstage: {
        title: 'Backstage',
        spec: 'backstage',
        description: 'Explore over 25 different Backstage related repositories. Search with examples below.',
        examples: [
            {
                title: 'Browse diffs for recent code changes',
                query: 'type:diff after:"1 week ago"',
                patternType: SearchPatternType.standard,
            },
        ],
        homepageDescription: 'Search within the Backstage community.',
        homepageIcon: 'https://raw.githubusercontent.com/sourcegraph-community/backstage-context/main/backstage.svg',
    },
    julia: {
        title: 'Julia',
        spec: 'julia',
        description: 'Explore over 20 different Julia repositories. Search with examples below.',
        examples: [
            {
                title: "List all TODO's in Julia code",
                query: 'lang:Julia TODO case:yes',
                patternType: SearchPatternType.standard,
            },
            {
                title: 'Browse diffs for recent code changes',
                query: 'type:diff after:"1 week ago"',
                patternType: SearchPatternType.standard,
            },
        ],
        homepageDescription: 'Search within the Julia community.',
        homepageIcon: 'https://raw.githubusercontent.com/JuliaLang/julia/master/doc/src/assets/logo.svg',
    },
    kubernetes: {
        title: 'Kubernetes',
        spec: 'kubernetes',
        description: 'Explore Kubernetes repositories on GitHub. Search with examples below.',
        examples: [
            {
                title: 'Use a ReplicationController configuration to ensure specified number of pod replicas are running at any one time',
                query: 'file:pod.yaml content:"kind: ReplicationController"',
                patternType: SearchPatternType.standard,
            },
            {
                title: 'Look for outdated `apiVersions` of admission webhooks',
                description: `This apiVersion has been deprecated in favor of "admissionregistration.k8s.io/v1".
            You can read more about this at https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/`,
                query: 'content:"apiVersion: admissionregistration.k8s.io/v1beta1"',
                patternType: SearchPatternType.standard,
            },
            {
                title: 'Find Prometheus usage in YAML files',
                query: 'lang:yaml prom/prometheus',
                patternType: SearchPatternType.standard,
            },
            {
                title: 'Search for examples of the sidecar pattern in Go',
                query: 'lang:go sidecar',
                patternType: SearchPatternType.standard,
            },
            {
                title: 'Browse diffs for recent code changes',
                query: 'type:diff after:"1 week ago"',
                patternType: SearchPatternType.standard,
            },
        ],
        homepageDescription: 'Search within the Kubernetes community.',
        homepageIcon: 'https://code.benco.io/icon-collection/logos/kubernetes.svg',
    },
    stackstorm: {
        title: 'StackStorm',
        spec: 'stackstorm',
        description: '',
        examples: [
            {
                title: 'Passive sensor examples',
                patternType: SearchPatternType.standard,
                query: 'from st2reactor.sensor.base import Sensor',
            },
            {
                title: 'Polling sensor examples',
                patternType: SearchPatternType.standard,
                query: 'from st2reactor.sensor.base import PollingSensor',
            },
            {
                title: 'Trigger examples in rules',
                patternType: SearchPatternType.standard,
                query: 'repo:Exchange trigger: file:.yaml$',
            },
            {
                title: 'Actions that use the Orquesta runner',
                patternType: SearchPatternType.regexp,
                query: 'repo:Exchange runner_type:\\s*"orquesta"',
            },
        ],
        homepageDescription: 'Search within the StackStorm and StackStorm Exchange community.',
        homepageIcon: 'https://avatars.githubusercontent.com/u/4969009?s=200&v=4',
    },
    stanford: {
        title: 'Stanford University',
        spec: 'stanford',
        description: 'Explore open-source code from Stanford students, faculty, research groups, and clubs.',
        examples: [
            {
                title: 'Find all mentions of "machine learning" in Stanford projects.',
                patternType: SearchPatternType.standard,
                query: 'machine learning',
            },
            {
                title: 'Explore the code of specific research groups like Hazy Research, a group that investigates machine learning models and automated training set creation.',
                patternType: SearchPatternType.standard,
                query: 'repo:/HazyResearch/',
            },
            {
                title: 'Explore the code of a specific user or organization such as Stanford University School of Medicine.',
                patternType: SearchPatternType.standard,
                query: 'repo:/susom/',
            },
            {
                title: 'Search for repositories related to introductory programming concepts.',
                patternType: SearchPatternType.standard,
                query: 'repo:recursion',
            },
            {
                title: 'Explore the README files of thousands of projects.',
                patternType: SearchPatternType.standard,
                query: 'file:README.txt',
            },
            {
                title: 'Find old-style string formatted print statements.',
                patternType: SearchPatternType.structural,
                query: 'lang:python print(:[args] % :[v])',
            },
        ],
        homepageDescription: 'Explore Stanford open-source code.',
        homepageIcon:
            'https://upload.wikimedia.org/wikipedia/commons/thumb/a/aa/Icons8_flat_graduation_cap.svg/120px-Icons8_flat_graduation_cap.svg.png',
    },
    temporal: {
        title: 'Temporal',
        spec: 'temporalio',
        description: '',
        examples: [
            {
                title: 'All test functions',
                patternType: SearchPatternType.standard,
                query: 'type:symbol Test',
            },
            {
                title: 'Search for a specifc function or class',
                patternType: SearchPatternType.standard,
                query: 'type:symbol SimpleSslContextBuilder',
            },
        ],
        homepageDescription: 'Search within the Temporal organization.',
        homepageIcon: 'https://avatars.githubusercontent.com/u/56493103?s=200&v=4',
    },
    chakraui: {
        title: 'CHAKRA UI',
        spec: 'chakraui',
        description: '',
        examples: [
            {
                title: 'Search for Chakra UI packages',
                patternType: SearchPatternType.standard,
                query: 'file:package.json',
            },
            {
                title: 'Browse diffs for recent code changes',
                patternType: SearchPatternType.standard,
                query: 'type:diff after:"1 week ago"',
            },
        ],
        homepageDescription: 'Search within the Chakra UI organization.',
        homepageIcon: 'https://raw.githubusercontent.com/chakra-ui/chakra-ui/main/logo/logomark-colored.svg',
    },
    cncf: {
        title: 'Cloud Native Computing Foundation (CNCF)',
        spec: 'cncf',
        description: 'Search the [CNCF projects](https://landscape.cncf.io/project=hosted)',
        examples: [],
        homepageDescription: 'Search CNCF projects.',
        homepageIcon: 'https://raw.githubusercontent.com/cncf/artwork/master/other/cncf/icon/color/cncf-icon-color.png',
        lowProfile: true,
    },
    o3de: {
        title: 'O3DE',
        spec: 'o3de',
        description: '',
        examples: [
            {
                title: 'Search for O3DE gems',
                patternType: SearchPatternType.standard,
                query: 'file:gem.json',
            },
            {
                title: 'Browse diffs for recent code changes',
                patternType: SearchPatternType.standard,
                query: 'type:diff after:"1 week ago"',
            },
        ],
        homepageDescription: 'Search within the O3DE organization.',
        homepageIcon:
            'https://raw.githubusercontent.com/o3de/artwork/19b89e72e15824f20204a8977a007f53d5fcd5b8/o3de/03_O3DE%20Application%20Icon/SVG/O3DE-Circle-Icon.svg',
    },
} satisfies Record<Community, CommunitySearchContextMetadata>
