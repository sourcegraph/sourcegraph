# Environment variables

## Overview

`src` requires two environment variables to be set to authenticate against your Sourcegraph instance.

## `SRC_ENDPOINT`

`SRC_ENDPOINT` defines the base URL for your Sourcegraph instance. In most cases, this will be a simple HTTPS URL, such as the following:

```
https://sourcegraph.com
```

If you're unsure what the URL for your Sourcegraph instance is, please contact your site administrator.

If the environment variable is not set, it'll default to "https://sourcegraph.com"

## `SRC_ACCESS_TOKEN`

`src` uses an access token to authenticate as you to your Sourcegraph instance. This token needs to be in the `SRC_ACCESS_TOKEN` environment variable.

To create an access token, please refer to "[Creating an access token](../how-tos/creating_an_access_token.md)".

## Adding request headers with `SRC_HEADER_`

If your instance is behind an authenticating proxy that requires additional headers, they can be supplied via environment variables. Any environment variable passed starting with the string `SRC_HEADER_{string-A}="String-B"` will be passed into the request with form `String-A: String-B`. See examples below:

Example passing env vars src command:
```bash
SRC_HEADER_AUTHORIZATION="Bearer $(curl http://service.internal.corp)" SRC_HEADER_EXTRA=metadata src search 'foobar'
```

In the above example, the headers `authorization: Bearer my-generated-token` and `extra: metadata` will be threaded to all HTTP requests to your instance. Multiple such headers can be supplied.

Example passing env vars via a shell config file:
In the .zshrc -
```bash
...

# src proxy auth
export SRC_HEADER_AUTHORIZATION="Bearer $(curl -H "Accept: text/plain" https://icanhazdadjoke.com/)"

...
```
Using a `src search` with `-get-curl` to expose the network request:
```bash
src search -get-curl 'repogroup:swarm'
```
```bash
curl \
   -H 'Authorization: token <REDACTED>' \
   -H 'authorization: Bearer What did the judge say to the dentist? Do you swear to pull the tooth, the whole tooth and nothing but the tooth?' \
   -d '{"query":"fragment FileMatchFields on FileMatch {\n\t\t\t\trepository {\n\t\t\t\t\tname\n\t\t\t\t\turl\n\t\t\t\t}\n\t\t\t\tfile {\n\t\t\t\t\tname\n\t\t\t\t\tpath\n\t\t\t\t\turl\n\t\t\t\t\tcontent\n\t\t\t\t\tcommit {\n\t\t\t\t\t\toid\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tlineMatches {\n\t\t\t\t\tpreview\n\t\t\t\t\tlineNumber\n\t\t\t\t\toffsetAndLengths\n\t\t\t\t\tlimitHit\n\t\t\t\t}\n\t\t\t}\n\n\t\t\tfragment CommitSearchResultFields on CommitSearchResult {\n\t\t\t\tmessagePreview {\n\t\t\t\t\tvalue\n\t\t\t\t\thighlights{\n\t\t\t\t\t\tline\n\t\t\t\t\t\tcharacter\n\t\t\t\t\t\tlength\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tdiffPreview {\n\t\t\t\t\tvalue\n\t\t\t\t\thighlights {\n\t\t\t\t\t\tline\n\t\t\t\t\t\tcharacter\n\t\t\t\t\t\tlength\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tlabel {\n\t\t\t\t\thtml\n\t\t\t\t}\n\t\t\t\turl\n\t\t\t\tmatches {\n\t\t\t\t\turl\n\t\t\t\t\tbody {\n\t\t\t\t\t\thtml\n\t\t\t\t\t\ttext\n\t\t\t\t\t}\n\t\t\t\t\thighlights {\n\t\t\t\t\t\tcharacter\n\t\t\t\t\t\tline\n\t\t\t\t\t\tlength\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tcommit {\n\t\t\t\t\trepository {\n\t\t\t\t\t\tname\n\t\t\t\t\t}\n\t\t\t\t\toid\n\t\t\t\t\turl\n\t\t\t\t\tsubject\n\t\t\t\t\tauthor {\n\t\t\t\t\t\tdate\n\t\t\t\t\t\tperson {\n\t\t\t\t\t\t\tdisplayName\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\n\t\t  fragment RepositoryFields on Repository {\n\t\t\tname\n\t\t\turl\n\t\t\texternalURLs {\n\t\t\t  serviceType\n\t\t\t  url\n\t\t\t}\n\t\t\tlabel {\n\t\t\t\thtml\n\t\t\t}\n\t\t  }\n\n\t\t  query ($query: String!) {\n\t\t\tsite {\n\t\t\t\tbuildVersion\n\t\t\t}\n\t\t\tsearch(query: $query) {\n\t\t\t  results {\n\t\t\t\tresults{\n\t\t\t\t  __typename\n\t\t\t\t  ... on FileMatch {\n\t\t\t\t\t...FileMatchFields\n\t\t\t\t  }\n\t\t\t\t  ... on CommitSearchResult {\n\t\t\t\t\t...CommitSearchResultFields\n\t\t\t\t  }\n\t\t\t\t  ... on Repository {\n\t\t\t\t\t...RepositoryFields\n\t\t\t\t  }\n\t\t\t\t}\n\t\t\t\tlimitHit\n\t\t\t\tcloning {\n\t\t\t\t  name\n\t\t\t\t}\n\t\t\t\tmissing {\n\t\t\t\t  name\n\t\t\t\t}\n\t\t\t\ttimedout {\n\t\t\t\t  name\n\t\t\t\t}\n\t\t\t\tmatchCount\n\t\t\t\telapsedMilliseconds\n\t\t\t\t...SearchResultsAlertFields\n\t\t\t  }\n\t\t\t}\n\t\t  }\n\t\t\n\tfragment SearchResultsAlertFields on SearchResults {\n\t\talert {\n\t\t\ttitle\n\t\t\tdescription\n\t\t\tproposedQueries {\n\t\t\t\tdescription\n\t\t\t\tquery\n\t\t\t}\n\t\t}\n\t}\n","variables":{"query":"repogroup:swarm"}}' \
   https://cse-k8s.sgdev.org/.api/graphql
```

