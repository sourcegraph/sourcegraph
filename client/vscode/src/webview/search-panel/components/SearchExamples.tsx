export interface SearchExample {
    label: string
    trackEventName: string
    queryPreview: string
    fullQuery: string
}

export const exampleQueries: SearchExample[] = [
    {
        label: 'Search all of your repos, without escaping or regex',
        trackEventName: 'VSCEHomeSearchExamplesClick',
        queryPreview: 'repo:sourcegraph/.* Sprintf("%d -file:tests',
        fullQuery: 'repo:sourcegraph/.* Sprintf("%d -file:tests case:yes',
    },
    {
        label: 'Search and review commits faster than git log and grep',
        trackEventName: 'VSCEHomeSearchExamplesClick',
        queryPreview: 'type:diff before:"last week" TODO',
        fullQuery:
            'repo:^github.com/sourcegraph/sourcegraph$ type:diff after:"last week" select:commit.diff.added TODO',
    },
    {
        label: 'Quickly filter by language and other key attributes',
        trackEventName: 'VSCEHomeSearchExamplesClick',
        queryPreview: 'repo:sourcegraph lang:go or lang:Typescript',
        fullQuery: 'repo:sourcegraph/* -f:tests (lang:TypeScript or lang:go) Config() case:yes',
    },
]
