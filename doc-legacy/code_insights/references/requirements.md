# Requirements

Code Insights has requirements for the Sourcegraph server version, its deployment type, and its connected code hosts. 

## Sourcegraph server

While the latest version of Sourcegraph server is always recommended: 

- Version 3.28 is the minimum version required to use Code Insights over individual repositories
- Version 3.31.1 is the minimum version required to use Code Insights over all repositories

## Sourcegraph deployment 

You can only use Code Insights on a [Docker Compose](../../admin/deploy/docker-compose/index.md) or [Kubernetes](../../admin/deploy/kubernetes/index.md) Sourcegraph deployment. 

## Code hosts

Sourcegraph Code Insights is compatible with any [Sourcegraph-compatible code host](../../admin/repo/index.md), except: 

* Perforce repositories making use of sub-repo permissions are not supported 