import { storiesOf } from '@storybook/react'
import React from 'react'
import { BatchChangeInfoByline } from './BatchChangeInfoByline'
import { subDays } from 'date-fns'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/BatchChangeInfoByline', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

const THREE_DAYS_AGO = subDays(new Date(), 3).toISOString()

add('Never updated', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangeInfoByline
                {...props}
                createdAt={THREE_DAYS_AGO}
                initialApplier={{ url: 'http://test.test/alice', username: 'alice' }}
                lastAppliedAt={THREE_DAYS_AGO}
                lastApplier={{ url: 'http://test.test/alice', username: 'alice' }}
            />
        )}
    </EnterpriseWebStory>
))

add('Updated (same user)', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangeInfoByline
                {...props}
                createdAt={THREE_DAYS_AGO}
                initialApplier={{ url: 'http://test.test/alice', username: 'alice' }}
                lastAppliedAt={subDays(new Date(), 1).toISOString()}
                lastApplier={{ url: 'http://test.test/alice', username: 'alice' }}
            />
        )}
    </EnterpriseWebStory>
))

add('Updated (different users)', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangeInfoByline
                {...props}
                createdAt={THREE_DAYS_AGO}
                initialApplier={{ url: 'http://test.test/alice', username: 'alice' }}
                lastAppliedAt={subDays(new Date(), 1).toISOString()}
                lastApplier={{ url: 'http://test.test/bob', username: 'bob' }}
            />
        )}
    </EnterpriseWebStory>
))
