import { gql } from '@sourcegraph/http-client'

export const USER_CODY_PLAN = gql`
    query UserCodyPlan {
        currentUser {
            id
            codySubscription {
                status
                plan
                applyProRateLimits
                currentPeriodStartAt
                currentPeriodEndAt
                cancelAtPeriodEnd
            }
        }
    }
`

export const USER_CODY_USAGE = gql`
    query UserCodyUsage {
        currentUser {
            id
            codyCurrentPeriodChatUsage
            codyCurrentPeriodCodeUsage
            codyCurrentPeriodChatLimit
            codyCurrentPeriodCodeLimit
            codySubscription {
                status
                plan
                applyProRateLimits
                currentPeriodStartAt
                currentPeriodEndAt
                cancelAtPeriodEnd
            }
        }
    }
`
