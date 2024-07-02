import { useEffect, useState } from 'react'

import { Button, Paper, Stack, Typography } from '@mui/material'

import { call } from './api'
import { changeStage, maintenance } from './debugBar'
import { ContextProps, stage } from './Frame'

const DebugBarTimerMs = 1 * 1000

export const OperatorDebugBar: React.FC<ContextProps> = ({ context }) => {
    const [waiting, setWaiting] = useState(false)

    const setStage = (action: stage, data?: string) => changeStage({ action, data, onDone: () => setWaiting(true) })

    const startInstall = () => setStage('install')
    const installProgress = () => setStage('installing')
    const installWaitAdmin = () => setStage('wait-for-admin')
    const upgradeProgress = () => setStage('upgrading', '5.4.0 (beta1)')
    const noState = () => setStage('unknown')
    const launchAdminUI = () => setStage('refresh')
    const failInstall = () => {
        call('/api/operator/v1beta1/fake/install/fail', {
            method: 'POST',
        }).then(() => {
            setWaiting(true)
        })
    }
    const setMaintenance = ({ healthy }: { healthy: boolean }) =>
        maintenance({ healthy, onDone: () => setWaiting(true) })

    useEffect(() => {
        const timer = setInterval(() => {
            if (waiting) {
                setWaiting(false)
            }
        }, DebugBarTimerMs)
        return () => clearInterval(timer)
    }, [waiting])

    const showDebugBar = localStorage.getItem('debugbar') === 'true'

    return (
        context.online &&
        showDebugBar && (
            <Paper id="operator-debug" elevation={3} sx={{ m: 1, p: 2 }}>
                <Stack direction="column" spacing={1} sx={{ alignItems: 'center' }}>
                    <Typography variant="caption">Operator Debug Controls</Typography>
                    <Stack direction="row" spacing={1}>
                        <Stack sx={{ alignItems: 'center', p: 1, border: '1px solid lightgray' }}>
                            <Typography variant="caption">Installation</Typography>
                            <Stack direction="row">
                                <Stack direction="column">
                                    <Button disabled={waiting} onClick={startInstall}>
                                        Start
                                    </Button>
                                    <Button disabled={waiting} onClick={installProgress}>
                                        Progress...
                                    </Button>
                                </Stack>
                                <Stack direction="column">
                                    <Button disabled={waiting} onClick={installWaitAdmin}>
                                        Wait for admin
                                    </Button>
                                    <Button disabled={waiting} onClick={failInstall}>
                                        Crash
                                    </Button>
                                </Stack>
                            </Stack>
                        </Stack>
                        <Stack sx={{ alignItems: 'center', p: 1, border: '1px solid lightgray' }}>
                            <Typography variant="caption">Maintenance</Typography>
                            <Button disabled={waiting} onClick={() => setMaintenance({ healthy: false })}>
                                Unhealthy
                            </Button>
                            <Button disabled={waiting} onClick={() => setMaintenance({ healthy: true })}>
                                Healthy
                            </Button>
                        </Stack>
                        <Stack
                            sx={{
                                alignItems: 'center',
                                p: 1,
                                border: '1px solid lightgray',
                            }}
                        >
                            <Typography variant="caption">Reset</Typography>
                            <Button disabled={waiting} onClick={noState}>
                                Reset
                            </Button>
                        </Stack>
                        <Stack
                            sx={{
                                alignItems: 'center',
                                p: 1,
                                border: '1px solid lightgray',
                            }}
                        >
                            <Typography variant="caption">Upgrade</Typography>
                            <Button disabled={waiting} onClick={upgradeProgress}>
                                Start
                            </Button>
                            <Button disabled={waiting} onClick={launchAdminUI}>
                                Finish
                            </Button>
                        </Stack>
                    </Stack>
                </Stack>
            </Paper>
        )
    )
}
