import { storiesOf } from '@storybook/react'
import { noop } from 'lodash'
import React from 'react'
import { BatchChangesCredentialFields, ExternalServiceKind } from '../../../graphql-operations'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { ViewCredentialModal } from './ViewCredentialModal'

const { add } = storiesOf('web/batches/settings/ViewCredentialModal', module)
    .addDecorator(story => <div className="p-3 container web-content">{story()}</div>)
    .addParameters({
        chromatic: {
            // Delay screenshot taking, so the modal has opened by the time the screenshot is taken.
            delay: 2000,
        },
    })

const credential: BatchChangesCredentialFields = {
    id: '123',
    createdAt: new Date().toISOString(),
    sshPublicKey:
        'ssh-rsa randorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorandorando',
}

add('View', () => (
    <EnterpriseWebStory>
        {props => (
            <ViewCredentialModal
                {...props}
                codeHost={{
                    credential,
                    externalServiceKind: ExternalServiceKind.GITHUB,
                    externalServiceURL: 'https://github.com/',
                    requiresSSH: true,
                }}
                credential={credential}
                onClose={noop}
            />
        )}
    </EnterpriseWebStory>
))
