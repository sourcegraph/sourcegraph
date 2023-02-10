import { FC, HTMLAttributes } from 'react'

import classNames from 'classnames'
import { Routes, Route, matchPath, useLocation } from 'react-router-dom-v5-compat'

import { Container, Text } from '@sourcegraph/wildcard'

import { CustomNextButton } from '../setup-steps'

import { CodeHostsPicker } from './components/code-host-picker'
import { CodeHostsNavigation } from './components/navigation'

import styles from './RemoteRepositoriesStep.module.scss'

interface RemoteRepositoriesStepProps extends HTMLAttributes<HTMLDivElement> {}

export const RemoteRepositoriesStep: FC<RemoteRepositoriesStepProps> = props => {
    const { className, ...attributes } = props

    const location = useLocation()
    const editConnectionRouteMatch = matchPath('/setup/remote-repositories/:codehostid/edit', location.pathname)
    const newConnectionRouteMatch = matchPath('/setup/remote-repositories/:codehost/create', location.pathname)

    return (
        <div {...attributes} className={classNames(className, styles.root)}>
            <Text className="mb-2">Connect remote code hosts where your source code lives.</Text>

            <section className={styles.content}>
                <Container className={styles.contentNavigation}>
                    <CodeHostsNavigation
                        activeConnectionId={editConnectionRouteMatch?.params?.codehostid}
                        addNewCodeHost={newConnectionRouteMatch !== null}
                        className={styles.navigation}
                    />
                </Container>

                <Container className={styles.contentMain}>
                    <Routes>
                        <Route index={true} element={<CodeHostsPicker />} />
                        <Route path=":codehost/create" element={<span>Hello creation UI</span>} />
                        <Route path=":codehostid/edit" element={<span>Hello edit UI</span>} />
                    </Routes>
                </Container>
            </section>

            <CustomNextButton label="Custom next step label" disabled={true} />
        </div>
    )
}
