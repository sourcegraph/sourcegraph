import { encodeURIPathComponent } from '@sourcegraph/common'
import { Button, Icon, Link, Tooltip } from '@sourcegraph/wildcard'

import { type GitCommitFields, RepositoryType } from '../graphql-operations'

import { CodeHostType } from './constants'

export const isPerforceChangelistMappingEnabled = (): boolean =>
    window.context.experimentalFeatures.perforceChangelistMapping === 'enabled'

export const isPerforceDepotSource = (sourceType: string): boolean => sourceType === RepositoryType.PERFORCE_DEPOT

export const getRefType = (sourceType: RepositoryType | string): string =>
    isPerforceDepotSource(sourceType) ? 'changelist' : 'commit'

export const getCanonicalURL = (sourceType: RepositoryType | string, node: GitCommitFields): string =>
    isPerforceChangelistMappingEnabled() && isPerforceDepotSource(sourceType) && node.perforceChangelist
        ? node.perforceChangelist.canonicalURL
        : node.canonicalURL

export const getInitialSearchTerm = (repo: string): string => {
    const r = repo.split('/')
    return r.at(-1)?.trim() ?? ''
}

export const stringToCodeHostType = (codeHostType: string): CodeHostType => {
    switch (codeHostType) {
        case 'github': {
            return CodeHostType.GITHUB
        }
        case 'gitlab': {
            return CodeHostType.GITLAB
        }
        case 'bitbucketCloud': {
            return CodeHostType.BITBUCKETCLOUD
        }
        case 'bitbucketServer': {
            return CodeHostType.BITBUCKETSERVER
        }
        case 'gitolite': {
            return CodeHostType.GITOLITE
        }
        case 'awsCodeCommit': {
            return CodeHostType.AWSCODECOMMIT
        }
        case 'azureDevOps': {
            return CodeHostType.AZUREDEVOPS
        }
        default: {
            return CodeHostType.OTHER
        }
    }
}

interface RepoCommitsButtonProps {
    repoName: string
    repoType: string
    revision: string
    filePath: string
    svgPath: string
    className: string
}

export const RepoCommitsButton: React.FunctionComponent<React.PropsWithChildren<RepoCommitsButtonProps>> = props => {
    const { repoName, repoType, revision, filePath, svgPath, className } = props
    const isRepoPerforce = isPerforceChangelistMappingEnabled() && repoType === RepositoryType.PERFORCE_DEPOT
    const tooltip = isRepoPerforce ? 'Perforce changelists' : 'Git commits'
    const title = isRepoPerforce ? 'Changelists' : 'Commits'
    const revisionPath = isRepoPerforce ? 'changelists' : 'commits'
    return (
        <Tooltip content={tooltip}>
            <Button
                as={Link}
                name={title}
                className="flex-shrink-0"
                to={`/${encodeURIPathComponent(repoName)}${
                    revision && `@${encodeURIPathComponent(revision)}`
                }/-/${revisionPath}${filePath && `/${encodeURIPathComponent(filePath)}`}`}
                variant="secondary"
                outline={true}
            >
                <Icon aria-hidden={true} svgPath={svgPath} /> <span className={className}>{title}</span>
            </Button>
        </Tooltip>
    )
}
