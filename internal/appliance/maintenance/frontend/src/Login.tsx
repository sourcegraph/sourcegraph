import { createRef, useEffect, useState } from 'react'

import Maintenance from '@mui/icons-material/Engineering'
import { Box, Button, Paper, TextField, Typography } from '@mui/material'
import { useOutletContext } from 'react-router-dom'

import { OutletContext } from './Frame'

const PublicMessage: React.FC<{ onLoginRequest: () => void }> = ({ onLoginRequest }) => (
    <div className="public-message">
        <Maintenance sx={{ fontSize: 64 }} />
        <Typography>This system is currently undergoing maintenance.</Typography>
        <Typography>Please try again later or contact your administrator for more information.</Typography>
        <Box sx={{ marginTop: 20 }}>
            <Button variant="outlined" onClick={onLoginRequest}>
                Administrator Login
            </Button>
        </Box>
    </div>
)

const Form: React.FC<LoginProps> = ({ onLogin, failed }) => {
    const [password, setPassword] = useState<string>('')
    const passwordRef = createRef<HTMLInputElement>()
    const [loggingIn, setLoggingIn] = useState<boolean>(false)

    useEffect(() => {
        if (failed && loggingIn) {
            setLoggingIn(false)
            setPassword('')
            if (passwordRef.current) {
                passwordRef.current.focus()
            }
        }
    }, [failed, loggingIn, passwordRef])

    const login = () => {
        setLoggingIn(true)
        onLogin(password)
    }

    useEffect(() => {
        if (passwordRef.current) {
            passwordRef.current.focus()
        }
    }, [passwordRef])

    return (
        <div className="login">
            <Paper
                sx={{
                    p: 4,
                    display: 'flex',
                    flexDirection: 'column',
                    textAlign: 'center',
                    gap: 2,
                }}
            >
                <Typography variant="h5">Login</Typography>
                <TextField
                    inputRef={passwordRef}
                    type="password"
                    placeholder="Maintenance Password"
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    onKeyDown={e => {
                        if (e.key === 'Enter') {
                            login()
                        }
                    }}
                ></TextField>
                <Button variant="contained" onClick={login} disabled={loggingIn}>
                    Login
                </Button>
                {failed && <Typography color="error">Incorrect password</Typography>}
                <Typography variant="caption">
                    You can find the maintenance password in the cluster secret config.
                </Typography>
            </Paper>
        </div>
    )
}

interface LoginProps {
    onLogin: (password: string) => void
    failed: boolean
}

export const Login: React.FC<LoginProps> = ({ onLogin, failed }) => {
    const context = useOutletContext<OutletContext>()
    const [publicMessage, setPublicMessage] = useState<boolean>(true)

    useEffect(() => {
        // const delta = context.lastOnline - Date.now();
        // console.log(delta);
        // if (context.online && context.lastOnline) {
        // }
    }, [context])

    return publicMessage ? (
        <PublicMessage onLoginRequest={() => setPublicMessage(false)} />
    ) : (
        <Form onLogin={onLogin} failed={failed} />
    )
}
