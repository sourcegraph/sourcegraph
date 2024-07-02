import { Fragment, useEffect, useState } from 'react'

import Unhealthy from '@mui/icons-material/CarCrashOutlined'
import Healthy from '@mui/icons-material/ThumbUp'
import { Alert, Button, CircularProgress, Grid, Stack, Typography } from '@mui/material'
import classNames from 'classnames'

import { call } from './api'
import { maintenance } from './debugBar'

const MaintenanceStatusTimerMs = 1 * 1000
const WaitToLaunchFixMs = 5 * 1000

type Service = {
    name: string
    healthy: boolean
    message: string
}

type Status = {
    services: Service[]
}

const ShowServices: React.FC<{ services: Service[] }> = ({ services }) =>
    services.length > 0 ? (
        <Grid container spacing={2} className="service-grid">
            <Grid item xs={3} className="service-header">
                <Typography variant="caption">Service</Typography>
            </Grid>
            <Grid item xs={3} className="service-header">
                <Typography variant="caption">Health</Typography>
            </Grid>
            <Grid item xs={6} className="service-header">
                <Typography variant="caption">Message</Typography>
            </Grid>
            {services.map((s: Service) => {
                const className = classNames('service-item', s.healthy ? 'healthy' : 'unhealthy')
                return (
                    <Fragment key={s.name}>
                        <Grid item xs={3} className={className}>
                            {s.name}
                        </Grid>
                        <Grid item xs={3} className={className}>
                            {s.healthy && <Healthy />}
                            {!s.healthy && <Unhealthy />}
                        </Grid>
                        <Grid item xs={6} className={className}>
                            {s.message}
                        </Grid>
                    </Fragment>
                )
            })}
        </Grid>
    ) : null

export const Maintenance: React.FC = () => {
    const [status, setStatus] = useState<Status | undefined>()
    const [fixing, setFixing] = useState<boolean>(false)

    useEffect(() => {
        const timer = setInterval(() => {
            call('/api/operator/v1beta1/maintenance/status')
                .then(response => response.json())
                .then(setStatus)
        }, MaintenanceStatusTimerMs)
        return () => clearInterval(timer)
    }, [])

    useEffect(() => {
        if (fixing) {
            const timer = setInterval(() => {
                maintenance({ healthy: true }).then(() => setFixing(false))
            }, WaitToLaunchFixMs)
            return () => clearInterval(timer)
        }
    }, [fixing])

    const ready = status?.services.length !== undefined
    const unhealthy = status?.services?.find((s: Service) => !s.healthy)

    return (
        <div className="maintenance">
            <Typography variant="h5">Maintenance Page</Typography>
            {ready ? (
                unhealthy ? (
                    <Alert severity="warning">
                        Something is wrong. Please check the logs and actions below to resolve. If does not resolve,
                        please contact support.
                    </Alert>
                ) : (
                    <Alert severity="success">Everything is pretty around here!</Alert>
                )
            ) : (
                <CircularProgress />
            )}

            {ready ? (
                <>
                    <Typography variant="h5">Service Status</Typography>
                    <ShowServices services={status?.services ?? []} />
                </>
            ) : null}

            {unhealthy && (
                <>
                    <Typography variant="h5">Actions</Typography>
                    <Stack direction="row" spacing={1}>
                        <Button variant="contained" onClick={() => setFixing(true)}>
                            Restart Cluster
                        </Button>
                        <Button variant="contained" onClick={() => alert('failed :-(')}>
                            Page On-Call
                        </Button>
                        <Button variant="contained" onClick={() => alert('failed :-(')}>
                            Call Sourcegraph Support
                        </Button>
                    </Stack>
                </>
            )}

            {fixing && (
                <Stack direction="row" spacing={2}>
                    <CircularProgress size={32} />
                    <Typography variant="h5">Fixing... Please wait...</Typography>
                </Stack>
            )}
        </div>
    )
}
