import React, { Fragment, useEffect, useState } from 'react'

import styled from '@emotion/styled'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import DirectionsRunIcon from '@mui/icons-material/DirectionsRun'
import PauseCircleOutlineIcon from '@mui/icons-material/PauseCircleOutline'
import { Alert, Box, Grid, LinearProgress, LinearProgressProps, Stack, Typography } from '@mui/material'
import classnames from 'classnames'

import { call } from './api'

type Task = {
    title: string
    description: string
    started: boolean
    finished: boolean
    progress: number
    lastUpdate: number
}

function LinearProgressWithLabel(props: LinearProgressProps & { value: number }) {
    return (
        <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <Box sx={{ width: '100%', mr: 1 }}>
                <LinearProgress variant="determinate" {...props} />
            </Box>
            <Box sx={{ minWidth: 35 }}>
                <Typography variant="body2">{`${Math.round(props.value)}%`}</Typography>
            </Box>
        </Box>
    )
}

const BorderLinearProgress = styled(LinearProgress)(() => ({
    width: 200,
    height: 10,
}))

const ProgressBar: React.FC<{ value: number }> = ({ value }) => (
    <Box sx={{ display: 'flex', alignItems: 'center' }}>
        <Box sx={{ width: '100%', mr: 1 }}>
            <BorderLinearProgress sx={{ innerHeight: 10 }} variant="determinate" value={value} />
        </Box>
        <Box sx={{ minWidth: 35 }}>
            <Typography>{`${Math.round(value)}%`}</Typography>
        </Box>
    </Box>
)

const TaskGrid: React.FC<{ tasks: Task[] }> = ({ tasks }) =>
    tasks.length > 0 ? (
        <Grid container spacing={2} className="task-grid">
            {tasks.map((t: Task) => {
                const className = classnames('task-grid-item', t.started && !t.finished ? 'running' : null)
                return (
                    <Fragment key={t.title}>
                        <Grid item xs={1} className={className}>
                            {t.finished ? (
                                <CheckCircleOutlineIcon />
                            ) : t.started ? (
                                <Stack direction="row" sx={{ alignItems: 'center', gap: 1 }}>
                                    <DirectionsRunIcon />
                                </Stack>
                            ) : (
                                <PauseCircleOutlineIcon />
                            )}
                        </Grid>
                        <Grid item xs={2} className={className}>
                            {t.title}
                        </Grid>
                        <Grid item xs={6} className={className}>
                            {t.description}
                        </Grid>
                        <Grid item xs={3} className={className}>
                            <Box sx={{ width: '100%' }}>
                                {t.started && <LinearProgressWithLabel value={t.progress} />}
                            </Box>
                        </Grid>
                    </Fragment>
                )
            })}
        </Grid>
    ) : null

export const Progress: React.FC<{
    action: 'install' | 'upgrade'
}> = ({ action }) => {
    const [version, setVersion] = useState<string>()
    const [progress, setProgress] = useState<number>(0)
    const [error, setError] = useState<string>()
    const [tasks, setTasks] = useState<Task[]>([])

    useEffect(() => {
        const timer = setInterval(() => {
            call('/api/operator/v1beta1/install/progress')
                .then(result => result.json())
                .then(result => {
                    setVersion(result.version)
                    setProgress(result.progress)
                    setError(result.error)
                    setTasks(result.tasks)
                })
                .catch(err => setError(err.message))
        }, 1000)
        return () => clearInterval(timer)
    }, [])

    return (
        <div className="progress">
            {progress === 100 && (
                <Alert severity="success">
                    {action === 'install' ? 'Installation' : 'Upgrade'} successful. Please wait for Admin UI to appear.
                </Alert>
            )}
            <Typography variant="h5">
                {version === undefined || version === ''
                    ? action === 'install'
                        ? `Installing...`
                        : `Upgrading...`
                    : action === 'install'
                    ? `Installing version ${version}...`
                    : `Upgrading to version ${version}...`}
            </Typography>
            <ProgressBar value={progress} />
            {error && <Alert severity="error">{error}</Alert>}
            <TaskGrid tasks={tasks} />
        </div>
    )
}
