import { type FC, useState, useEffect } from 'react'

import { mdiChevronDown } from '@mdi/js'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import {
    Text,
    H1,
    ButtonLink,
    Link,
    Menu,
    MenuButton,
    MenuList,
    MenuItem,
    Icon,
    Position,
    MenuLink,
} from '@sourcegraph/wildcard'

import { getLicenseFeatures } from '../../util/license'
import { CodyColorIcon } from '../chat/CodyPageIcon'
import { IntelliJIcon } from '../upsell/IntelliJ'
import { VSCodeIcon } from '../upsell/vs-code'

import { UpsellImage } from './UpsellImage'

import styles from './CodyDashboardPage.module.scss'

interface SetupOption {
    icon: JSX.Element
    maker: string
    name: string
    setupLink: string
}

const setupOptions: SetupOption[] = [
    {
        icon: <VSCodeIcon className={styles.linkSelectorIcon} />,
        maker: 'Microsoft',
        name: 'VSCode',
        setupLink: 'https://sourcegraph.com/docs/cody/clients/install-vscode',
    },
    {
        icon: <IntelliJIcon className={styles.linkSelectorIcon} />,
        maker: 'Jetbrains',
        name: 'IntelliJ',
        setupLink: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
    },
]

interface CodyDashboardPageProps extends TelemetryV2Props {}

export const CodyDashboardPage: FC<CodyDashboardPageProps> = ({ telemetryRecorder }) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('cody.dashboard', 'view')
    }, [telemetryRecorder])

    const isLightTheme = useIsLightTheme()
    const codySetupLink = 'https://sourcegraph.com/docs/cody'
    const features = getLicenseFeatures()
    return (
        <section className={styles.dashboardContainer}>
            <section className={styles.dashboardHero}>
                <CodyColorIcon className={styles.dashboardCodyIcon} />
                <H1 className={styles.dashboardHeroHeader}>
                    Get started with <span className={styles.codyGradient}>Cody</span>
                </H1>
                <Text className={styles.dashboardHeroTagline}>
                    Hey! 👋 Let’s get started with Cody — your new AI coding assistant.
                </Text>
            </section>

            <section className={styles.dashboardOnboarding}>
                <section className={styles.dashboardOnboardingIde}>
                    <Text className={styles.dashboardText}>Download Cody for your favorite IDE</Text>
                    <LinkSelector options={setupOptions} />
                    <Text className="text-muted">
                        Struggling with setup?{' '}
                        <Link to={codySetupLink} className={styles.dashboardOnboardingIdeInstallationLink}>
                            Explore installation docs
                        </Link>
                        .
                    </Text>
                </section>
                <section className={styles.dashboardOnboardingWeb}>
                    <Text className={styles.dashboardText}>... or try it on the web</Text>
                    <ButtonLink to="/cody/chat" outline={true} className={styles.dashboardOnboardingWebLink}>
                        <CodyColorIcon className={styles.dashboardOnboardingCodyIcon} />
                        <span>Cody for web</span>
                    </ButtonLink>
                </section>
            </section>

            {!features.isCodeSearchEnabled && (
                <section className={styles.dashboardUpsell}>
                    <section className={styles.dashboardUpsellMeta}>
                        <SearchIcon />
                        <Text className={styles.dashboardUpsellTitle}>
                            Take control of your codebases with Code Search.
                        </Text>
                        <Text className={styles.dashboardUpsellDescription}>
                            Code Search allows you to search, understand and fix code, across massive codebases.
                            Discover vulnerabilities, improve code quality and more.
                        </Text>
                        <Link to="/search">Explore Code Search</Link>
                    </section>
                    <UpsellImage isLightTheme={isLightTheme} className="w-100" />
                </section>
            )}
        </section>
    )
}

interface LinkSelectorProps {
    options: SetupOption[]
}

const LinkSelector: FC<LinkSelectorProps> = ({ options }) => {
    const [firstOption] = options
    const [selectedOption, setSelectedOption] = useState<SetupOption>(firstOption)
    return (
        <section className={styles.linkSelectorContainer}>
            <Menu>
                <MenuLink
                    as={Link}
                    className={styles.linkSelectorInfo}
                    to={selectedOption.setupLink}
                    target="_blank"
                    rel="noreferrer"
                >
                    {selectedOption.icon}
                    <section>
                        <Text className={styles.linkSelectorOptionMaker}>{selectedOption.maker}</Text>
                        <Text className={styles.linkSelectorOptionName}>{selectedOption.name}</Text>
                    </section>
                </MenuLink>
                <MenuButton variant={undefined} className={styles.linkSelectorBtn}>
                    <Icon size="md" aria-hidden={true} svgPath={mdiChevronDown} />
                </MenuButton>

                <MenuList position={Position.bottomEnd} className={styles.linkSelectorDropdown}>
                    {options.map((option, index) => (
                        <MenuItem
                            key={index}
                            className={styles.linkSelectorItem}
                            onSelect={() => setSelectedOption(option)}
                        >
                            <Text className="m-0">Install Cody on {option.name}</Text>
                        </MenuItem>
                    ))}
                </MenuList>
            </Menu>
        </section>
    )
}

const SearchIcon: FC = () => (
    <svg xmlns="http://www.w3.org/2000/svg" width="37" height="40" fill="none" viewBox="0 0 37 40">
        <path
            fill="url(#paint0_linear_1070_2338)"
            fillRule="evenodd"
            d="M18.067 4.53c-7.441 0-13.5 6.029-13.5 13.5 0 7.47 6.059 13.5 13.5 13.5a2.274 2.274 0 012.284 2.264 2.274 2.274 0 01-2.284 2.265C8.074 36.059 0 27.972 0 18.029 0 8.087 8.074 0 18.067 0c9.994 0 18.068 8.087 18.068 18.03 0 4.964-2.013 9.463-5.268 12.724l5.393 5.386a2.251 2.251 0 01-.011 3.202 2.296 2.296 0 01-3.23-.01l-7.101-7.094a2.254 2.254 0 01.243-3.402 13.476 13.476 0 005.408-10.807c0-7.47-6.06-13.5-13.502-13.5z"
            clipRule="evenodd"
        />
        <defs>
            <linearGradient
                id="paint0_linear_1070_2338"
                x1="0.885"
                x2="30.949"
                y1="26.786"
                y2="27.094"
                gradientUnits="userSpaceOnUse"
            >
                <stop stopColor="#00CBEC" />
                <stop offset="0.51" stopColor="#A112FF" />
                <stop offset="1" stopColor="#FF5543" />
            </linearGradient>
        </defs>
    </svg>
)
