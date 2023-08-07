import { render } from '@testing-library/react'
import GithubIcon from 'mdi-react/GithubIcon'

import { ExternalServiceGroup } from './ExternalServiceGroup'
import { GITHUB_DOTCOM } from './externalServices'

describe('ExternalServiceGroup', () => {
    test('should render correctly with enabled external services', () => {
        const props = {
            name: 'GitHub',
            services: [
                {
                    ...GITHUB_DOTCOM,
                    serviceID: 'github',
                    enabled: true,
                },
            ],
            description: 'Connect with GitHub repositories',
            renderServiceIcon: true,
            icon: GithubIcon,
        }

        const cmp = render(<ExternalServiceGroup {...props} />)
        expect(cmp.asFragment()).toMatchSnapshot()
    })

    test('should render correctly with disabled external services', () => {
        const props = {
            name: 'GitHub',
            services: [
                {
                    ...GITHUB_DOTCOM,
                    serviceID: 'github',
                    enabled: false,
                },
            ],
            description: 'Connect with GitHub repositories',
            renderServiceIcon: true,
            icon: GithubIcon,
        }

        const cmp = render(<ExternalServiceGroup {...props} />)
        expect(cmp.asFragment()).toMatchSnapshot()
    })
})
