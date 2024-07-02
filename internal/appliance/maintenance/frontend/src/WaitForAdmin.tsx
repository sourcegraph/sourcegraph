import { useEffect, useState } from 'react'

import { Button, CircularProgress, Stack, Typography } from '@mui/material'

import { changeStage } from './debugBar'

const TestAdminUIGoodMs = 1 * 1000
const WaitBeforeLaunchMs = 3 * 1000

export const WaitForAdmin: React.FC = () => {
    const [waitingForBalancer, setWaitingForBalancer] = useState<boolean>(false)
    const [launching, setLaunching] = useState<boolean>(false)

    useEffect(() => {
        if (launching) {
            const timer = setInterval(() => {
                changeStage({ action: 'refresh' })
            }, WaitBeforeLaunchMs)
            return () => clearInterval(timer)
        }
    }, [launching])

    useEffect(() => {
        const timer = setInterval(() => {
            fetch('/sign-in')
                .then(result => {
                    console.log('waiting for admin ui', result)
                    if (result.ok) {
                        setLaunching(true)
                        setWaitingForBalancer(false)
                    }
                })
                .catch(console.error)
        }, TestAdminUIGoodMs)
        return () => clearInterval(timer)
    }, [waitingForBalancer])

    return (
        <div className="wait-for-admin">
            <Typography variant="h5">Waiting For The Admin To Return</Typography>
            <div>
                <Typography sx={{ m: 2 }}>
                    The appliance is ready. We were waiting for you to set its security before opening it up.
                </Typography>
                <Typography sx={{ m: 2 }}>
                    Now that you're back, please press the button below to launch the Administration UI.
                </Typography>
            </div>
            <Button
                variant="contained"
                onClick={() => setWaitingForBalancer(true)}
                disabled={launching || waitingForBalancer}
            >
                Launch Admin UI
            </Button>
            {launching && (
                <Stack direction="row" spacing={2}>
                    <CircularProgress size={32} />
                    <Typography variant="h5">Launching Admin UI... Please wait...</Typography>
                </Stack>
            )}
            {waitingForBalancer && (
                <Stack direction="row" spacing={2}>
                    <CircularProgress size={32} />
                    <Typography variant="h5">Waiting for Admin UI to be ready... Please wait...</Typography>
                </Stack>
            )}
        </div>
    )
}
