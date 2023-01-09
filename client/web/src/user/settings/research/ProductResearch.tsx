import React, { useEffect } from 'react'

import { mdiOpenInNew } from '@mdi/js'

import { TelemetryService } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Container, PageHeader, ButtonLink, Icon, Text } from '@sourcegraph/wildcard'

import { AuthenticatedUser } from '../../../auth'
import { PageTitle } from '../../../components/PageTitle'

interface Props {
    telemetryService: TelemetryService
    authenticatedUser: Pick<AuthenticatedUser, 'emails'>
}

const SIGN_UP_FORM_URL = 'https://info.sourcegraph.com/product-research'

export const ProductResearchPage: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    telemetryService,
    authenticatedUser,
}) => {
    useEffect(() => {
        telemetryService.logViewEvent('UserSettingsProductResearch')
    }, [telemetryService])

    const signUpForm = new URL(SIGN_UP_FORM_URL)
    const primaryEmail = authenticatedUser.emails.find(email => email.isPrimary)
    if (primaryEmail) {
        signUpForm.searchParams.set('email', primaryEmail.email)
    }

    return (
        <>
            <PageTitle title="Product research" />
            <PageHeader headingElement="h2" path={[{ text: 'Product research and feedback' }]} className="mb-3" />
            <Container>
                <Text>
                    Our product team conducts occasional research to learn about how you use Sourcegraph and ask for
                    feedback about upcoming ideas. Sign up to participate in our research and help us shape the future
                    of our product!
                </Text>
                <ButtonLink to={signUpForm.href} target="_blank" rel="noopener noreferrer" variant="primary">
                    Sign up now <Icon aria-hidden={true} svgPath={mdiOpenInNew} />
                </ButtonLink>
            </Container>
        </>
    )
}
