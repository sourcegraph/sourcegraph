import { storiesOf } from '@storybook/react'
import React from 'react'
import { of } from 'rxjs'
import { ExternalServiceKind } from '../../../graphql-operations'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { BatchChangesSettingsArea } from './BatchChangesSettingsArea'

const { add } = storiesOf('web/batches/settings/BatchChangesSettingsArea', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

add('Overview', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangesSettingsArea
                {...props}
                user={{ id: 'user-id-1' }}
                queryUserBatchChangesCodeHosts={() =>
                    of({
                        totalCount: 3,
                        pageInfo: {
                            endCursor: null,
                            hasNextPage: false,
                        },
                        nodes: [
                            {
                                credential: null,
                                externalServiceKind: ExternalServiceKind.GITHUB,
                                externalServiceURL: 'https://github.com/',
                                requiresSSH: false,
                            },
                            {
                                credential: null,
                                externalServiceKind: ExternalServiceKind.GITLAB,
                                externalServiceURL: 'https://gitlab.com/',
                                requiresSSH: false,
                            },
                            {
                                credential: null,
                                externalServiceKind: ExternalServiceKind.BITBUCKETSERVER,
                                externalServiceURL: 'https://bitbucket.sgdev.org/',
                                requiresSSH: true,
                            },
                        ],
                    })
                }
            />
        )}
    </EnterpriseWebStory>
))

add('Config added', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangesSettingsArea
                {...props}
                user={{ id: 'user-id-2' }}
                queryUserBatchChangesCodeHosts={() =>
                    of({
                        totalCount: 3,
                        pageInfo: {
                            endCursor: null,
                            hasNextPage: false,
                        },
                        nodes: [
                            {
                                credential: {
                                    id: '123',
                                    createdAt: new Date().toISOString(),
                                    sshPublicKey:
                                        'rsa-ssh randorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorando',
                                },
                                externalServiceKind: ExternalServiceKind.GITHUB,
                                externalServiceURL: 'https://github.com/',
                                requiresSSH: false,
                            },
                            {
                                credential: {
                                    id: '123',
                                    createdAt: new Date().toISOString(),
                                    sshPublicKey:
                                        'rsa-ssh randorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorando',
                                },
                                externalServiceKind: ExternalServiceKind.GITLAB,
                                externalServiceURL: 'https://gitlab.com/',
                                requiresSSH: false,
                            },
                            {
                                credential: {
                                    id: '123',
                                    createdAt: new Date().toISOString(),
                                    sshPublicKey:
                                        'rsa-ssh randorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorando',
                                },
                                externalServiceKind: ExternalServiceKind.BITBUCKETSERVER,
                                externalServiceURL: 'https://bitbucket.sgdev.org/',
                                requiresSSH: true,
                            },
                        ],
                    })
                }
            />
        )}
    </EnterpriseWebStory>
))
