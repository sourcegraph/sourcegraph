import { storiesOf } from '@storybook/react'
import { noop } from 'lodash'
import React from 'react'
import { ExternalServiceKind } from '../../../../../shared/src/graphql-operations'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { RemoveCredentialModal } from './RemoveCredentialModal'

const { add } = storiesOf('web/batches/settings/RemoveCredentialModal', module)
    .addDecorator(story => <div className="p-3 container web-content">{story()}</div>)
    .addParameters({
        chromatic: {
            // Delay screenshot taking, so the modal has opened by the time the screenshot is taken.
            delay: 2000,
        },
    })

const credential = {
    id: '123',
    createdAt: new Date().toISOString(),
    sshPublicKey:
        'ssh-rsa randorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorando',
}

add('No ssh', () => (
    <EnterpriseWebStory>
        {props => (
            <RemoveCredentialModal
                {...props}
                codeHost={{
                    credential,
                    requiresSSH: false,
                    externalServiceKind: ExternalServiceKind.GITHUB,
                    externalServiceURL: 'https://github.com/',
                }}
                credential={credential}
                afterDelete={noop}
                onCancel={noop}
            />
        )}
    </EnterpriseWebStory>
))

add('Requires ssh', () => (
    <EnterpriseWebStory>
        {props => (
            <RemoveCredentialModal
                {...props}
                codeHost={{
                    credential,
                    requiresSSH: true,
                    externalServiceKind: ExternalServiceKind.GITHUB,
                    externalServiceURL: 'https://github.com/',
                }}
                credential={credential}
                afterDelete={noop}
                onCancel={noop}
            />
        )}
    </EnterpriseWebStory>
))
