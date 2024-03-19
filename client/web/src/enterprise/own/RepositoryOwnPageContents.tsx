import { useEffect, useState } from 'react'

import { Timestamp } from '@sourcegraph/branded/src/components/Timestamp'
import { useQuery } from '@sourcegraph/http-client'
import { H3, Text, Code, Card, LoadingSpinner, ErrorAlert } from '@sourcegraph/wildcard'

import type {
    GetIngestedCodeownersResult,
    GetIngestedCodeownersVariables,
    IngestedCodeowners,
    RepositoryFields,
} from '../../graphql-operations'

import { DeleteFileButton } from './DeleteFileButton'
import { GET_INGESTED_CODEOWNERS_QUERY } from './graphqlQueries'
import { IngestedFileViewer } from './IngestedFileViewer'
import type { RepositoryOwnAreaPageProps } from './RepositoryOwnEditPage'
import { UploadFileButton } from './UploadFileButton'

import styles from './RepositoryOwnPageContents.module.scss'

export interface CodeownersIngestedFile {
    contents: string
    updatedAt: string
}

export const RepositoryOwnPageContents: React.FunctionComponent<
    Pick<RepositoryOwnAreaPageProps, 'repo' | 'authenticatedUser' | 'telemetryRecorder'>
> = ({ repo, authenticatedUser, telemetryRecorder }) => {
    const isAdmin = authenticatedUser?.siteAdmin

    const { data, error, loading } = useQuery<GetIngestedCodeownersResult, GetIngestedCodeownersVariables>(
        GET_INGESTED_CODEOWNERS_QUERY,
        {
            variables: {
                repoID: repo.id,
            },
        }
    )

    const [codeownersIngestedFile, setCodeownersIngestedFile] = useState<IngestedCodeowners | null>(null)
    useEffect(() => {
        if (data?.node?.__typename === 'Repository') {
            if (data.node.ingestedCodeowners?.__typename === 'CodeownersIngestedFile') {
                setCodeownersIngestedFile(data.node.ingestedCodeowners)
            } else {
                setCodeownersIngestedFile(null)
            }
        }
    }, [data?.node])

    if (loading) {
        return (
            <div className="container d-flex justify-content-center mt-3">
                <LoadingSpinner /> Loading...
            </div>
        )
    }

    if (error) {
        return <ErrorAlert className="mt-3" error={error} prefix="Error loading ownership info for this repository" />
    }

    return (
        <>
            <Card className={styles.columns}>
                <div>
                    <H3>{isAdmin ? 'Upload a CODEOWNERS file' : 'Ask your site admin to upload a CODEOWNERS file'}</H3>
                    <Text>
                        {!isAdmin && 'A site admin can manually upload a CODEOWNERS file for this repository. '} Each
                        owner must be either a Sourcegraph username, a Sourcegraph team name, or an email address.
                    </Text>

                    {isAdmin && (
                        <UploadFileButton
                            repo={repo}
                            onComplete={file => {
                                setCodeownersIngestedFile(file)
                                telemetryRecorder.recordEvent('repo.ownership.edit.file', 'upload')
                            }}
                            fileAlreadyExists={!!codeownersIngestedFile}
                        />
                    )}
                </div>

                <div className={styles.or}>
                    <div className={styles.orLine} />
                    <div className="py-2">or</div>
                    <div className={styles.orLine} />
                </div>

                <div>
                    <H3>Commit a CODEOWNERS file</H3>
                    <Text>
                        Add a <Code>CODEOWNERS</Code> file to the root of this repository. Owners must be{' '}
                        {getCodeHostName(repo)} usernames or email addresses.
                    </Text>
                    {codeownersIngestedFile && (
                        <Text className={styles.commitWarning}>
                            <em>
                                Any committed CODEOWNERS file in this repository will be ignored unless the uploaded
                                file is deleted.
                            </em>
                        </Text>
                    )}
                </div>
            </Card>

            {codeownersIngestedFile && (
                <div className="mt-5">
                    <H3>Uploaded CODEOWNERS file</H3>
                    <div className="d-flex align-items-baseline justify-content-between">
                        <Text>
                            The following CODEOWNERS file was uploaded to Sourcegraph{' '}
                            <Timestamp date={codeownersIngestedFile.updatedAt} />.
                        </Text>
                        {isAdmin && (
                            <DeleteFileButton
                                repo={repo}
                                onComplete={() => {
                                    setCodeownersIngestedFile(null)
                                    telemetryRecorder.recordEvent('repo.ownership.edit.file', 'delete')
                                }}
                            />
                        )}
                    </div>
                    <IngestedFileViewer contents={codeownersIngestedFile.contents} />
                </div>
            )}
        </>
    )
}

const getCodeHostName = (repo: RepositoryFields): string => {
    const externalServiceKind = repo.externalURLs[0]?.serviceKind

    switch (externalServiceKind) {
        case 'GITHUB': {
            return 'GitHub'
        }
        case 'GITLAB': {
            return 'GitLab'
        }
        default: {
            return 'code host'
        }
    }
}
