import copy from 'copy-to-clipboard'
import ContentCopyIcon from 'mdi-react/ContentCopyIcon'
import React, { useCallback, useState } from 'react'
import { ExternalServiceKind } from '../../../graphql-operations'

const configInstructionLinks: Record<ExternalServiceKind, string> = {
    [ExternalServiceKind.GITHUB]:
        'https://docs.github.com/en/github/authenticating-to-github/adding-a-new-ssh-key-to-your-github-account',
    [ExternalServiceKind.GITLAB]: 'https://docs.gitlab.com/ee/ssh/#add-an-ssh-key-to-your-gitlab-account',
    [ExternalServiceKind.BITBUCKETSERVER]:
        'https://confluence.atlassian.com/bitbucketserver/ssh-user-keys-for-personal-use-776639793.html',
    [ExternalServiceKind.AWSCODECOMMIT]: 'unsupported',
    [ExternalServiceKind.BITBUCKETCLOUD]: 'unsupported',
    [ExternalServiceKind.GITOLITE]: 'unsupported',
    [ExternalServiceKind.OTHER]: 'unsupported',
    [ExternalServiceKind.PERFORCE]: 'unsupported',
    [ExternalServiceKind.PHABRICATOR]: 'unsupported',
}

export interface CodeHostSshPublicKeyProps {
    externalServiceKind: ExternalServiceKind
    sshPublicKey: string
    label?: string
    showInstructionsLink?: boolean
    showCopyButton?: boolean
}

export const CodeHostSshPublicKey: React.FunctionComponent<CodeHostSshPublicKeyProps> = ({
    externalServiceKind,
    sshPublicKey,
    showInstructionsLink = true,
    showCopyButton = true,
    label = 'Public SSH key',
}) => {
    const [copied, setCopied] = useState<boolean>(false)
    const onCopy = useCallback(() => {
        copy(sshPublicKey)
        setCopied(true)
    }, [sshPublicKey])
    return (
        <>
            <div className="d-flex justify-content-between align-items-end mb-2">
                <h4 className="mb-1">{label}</h4>
                {showCopyButton && (
                    <button type="button" className="btn btn-secondary" onClick={onCopy}>
                        <ContentCopyIcon className="icon-inline" />
                        {copied ? 'Copied!' : 'Copy'}
                    </button>
                )}
            </div>
            <textarea className="form-control text-monospace mb-3" rows={5} value={sshPublicKey} />
            {showInstructionsLink && (
                <p>
                    <a href={configInstructionLinks[externalServiceKind]} target="_blank" rel="noopener">
                        Configuration instructions
                    </a>
                </p>
            )}
        </>
    )
}
