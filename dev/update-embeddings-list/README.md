## Overview

This folder contains the scripts needed to automatically update the list of embedded repos on Sourcegraph.com. It calls the `RepoEmbeddingJobs` GraphQL endpoint to generate the list of embeddings and converts it into markdown.

## Development

To work on this file:

1. get access from [here](https://start.1password.com/open/i?a=HEDEDSLHPBFGRBTKAKJWE23XX4&v=dnrhbauihkhjs5ag6vszsme45a&i=za6swt25wax766z6pe7wpczxxe&h=team-sourcegraph.1password.com)
2. set access token `set SOURCEGRAPH_DOCS_ACCESS_TOKEN=<access_token>` or hard code
   `const access_token = <access_token>`
3. run `ts-node src/index.ts`

Alternatively you can also:

1. run `pnpm run start`
Hello World
