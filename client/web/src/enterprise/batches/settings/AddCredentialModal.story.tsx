import { select } from '@storybook/addon-knobs'
import { useCallback } from '@storybook/addons'
import { storiesOf } from '@storybook/react'
import { noop } from 'lodash'
import React from 'react'
import { BatchChangesCredentialFields, ExternalServiceKind } from '../../../graphql-operations'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { AddCredentialModal } from './AddCredentialModal'

const { add } = storiesOf('web/batches/settings/AddCredentialModal', module)
    .addDecorator(story => <div className="p-3 container web-content">{story()}</div>)
    .addParameters({
        chromatic: {
            // Delay screenshot taking, so the modal has opened by the time the screenshot is taken.
            delay: 2000,
        },
    })

add('Requires SSH - step 1', () => {
    const createBatchChangesCredential = useCallback(
        (): Promise<BatchChangesCredentialFields> =>
            Promise.resolve({
                id: '123',
                createdAt: new Date().toISOString(),
                sshPublicKey:
                    'ssh-rsa randorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorando',
            }),
        []
    )
    return (
        <EnterpriseWebStory>
            {props => (
                <AddCredentialModal
                    {...props}
                    userID="user-id-1"
                    externalServiceKind={select(
                        'External service kind',
                        Object.values(ExternalServiceKind),
                        ExternalServiceKind.GITHUB
                    )}
                    externalServiceURL="https://github.com/"
                    requiresSSH={true}
                    afterCreate={noop}
                    onCancel={noop}
                    createBatchChangesCredential={createBatchChangesCredential}
                />
            )}
        </EnterpriseWebStory>
    )
})
add('Requires SSH - step 2', () => (
    <EnterpriseWebStory>
        {props => (
            <AddCredentialModal
                {...props}
                userID="user-id-1"
                externalServiceKind={select(
                    'External service kind',
                    Object.values(ExternalServiceKind),
                    ExternalServiceKind.GITHUB
                )}
                externalServiceURL="https://github.com/"
                requiresSSH={true}
                afterCreate={noop}
                onCancel={noop}
                initialStep="get-ssh-key"
            />
        )}
    </EnterpriseWebStory>
))

add('GitHub', () => (
    <EnterpriseWebStory>
        {props => (
            <AddCredentialModal
                {...props}
                userID="user-id-1"
                externalServiceKind={ExternalServiceKind.GITHUB}
                externalServiceURL="https://github.com/"
                requiresSSH={false}
                afterCreate={noop}
                onCancel={noop}
            />
        )}
    </EnterpriseWebStory>
))

add('GitLab', () => (
    <EnterpriseWebStory>
        {props => (
            <AddCredentialModal
                {...props}
                userID="user-id-1"
                externalServiceKind={ExternalServiceKind.GITLAB}
                externalServiceURL="https://gitlab.com/"
                requiresSSH={false}
                afterCreate={noop}
                onCancel={noop}
            />
        )}
    </EnterpriseWebStory>
))

add('Bitbucket Server', () => (
    <EnterpriseWebStory>
        {props => (
            <AddCredentialModal
                {...props}
                userID="user-id-1"
                externalServiceKind={ExternalServiceKind.BITBUCKETSERVER}
                externalServiceURL="https://bitbucket.sgdev.org/"
                requiresSSH={false}
                afterCreate={noop}
                onCancel={noop}
            />
        )}
    </EnterpriseWebStory>
))
