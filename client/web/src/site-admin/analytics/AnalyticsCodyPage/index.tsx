import React from 'react'

import { Card, Link, Text } from '@sourcegraph/wildcard'

import { AnalyticsPageTitle } from '../components/AnalyticsPageTitle'

export const AnalyticsCodyPage: React.FC = () => (
    <>
        <AnalyticsPageTitle>Cody</AnalyticsPageTitle>

        <Card className="p-3">
            <Text>
                Cody analytics, including active users, completions, chat, and commands can be found at{' '}
                <Link to="https://cody-analytics.sourcegraph.com" target="_blank" rel="noopener">
                    cody-analytics.sourcegraph.com
                </Link>
                .
            </Text>
            <Text>To request access, please contact your account team.</Text>
        </Card>
    </>
)
