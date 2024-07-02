import { CircularProgress, Typography } from '@mui/material'

import './App.css'

import { useOutletContext } from 'react-router-dom'

import { OutletContext } from './Frame'

export const Home: React.FC = () => {
    const context = useOutletContext<OutletContext>()

    return (
        <div className="home">
            <CircularProgress size={18} />
            {context.online || context.needsLogin ? (
                <>
                    <Typography>Appliance connected. Please wait...</Typography>
                </>
            ) : (
                <>
                    <Typography>Please wait, while the Sourcegraph Appliance connects...</Typography>
                </>
            )}
        </div>
    )
}
