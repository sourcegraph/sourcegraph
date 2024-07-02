import { useState } from 'react'

import { Button, Checkbox, FormControl, InputLabel, MenuItem, Paper, Select, Stack, Typography } from '@mui/material'

import search from '../assets/sourcegraph.png'

import { changeStage } from './debugBar'

interface InstallerProps {
    allowDisable: boolean
}

export const Install: React.FC = () => {
    const [version, setVersion] = useState<string>('5.3.1')
    const [installSearch, setInstallSearch] = useState<boolean>(true)

    const install = () => {
        changeStage({ action: 'installing', data: version })
    }

    const SearchInstaller: React.FC<InstallerProps> = ({ allowDisable = false }) => (
        <Paper
            sx={{
                p: 2,
                display: 'flex',
                flexDirection: 'row',
                alignItems: 'flex-start',
                width: '100%',
                gap: 2,
            }}
            onClick={allowDisable ? () => setInstallSearch(prevSarch => !prevSarch) : undefined}
        >
            <img src={search} />
            <Stack sx={{ flex: 1 }}>
                <Typography variant="subtitle2">
                    <b>Search Suite</b>
                </Typography>
                <Typography variant="caption">
                    Sourcegraph search suite: Code Search, Code Intelligence, <br />
                    Batch Changes, and Own.
                </Typography>
            </Stack>
            <Checkbox sx={{ p: 0 }} color="default" size="small" checked={installSearch} />
        </Paper>
    )

    const allowInstall = installSearch

    return (
        <div className="install">
            <Typography variant="h5">Install Sourcegraph Appliance</Typography>
            <Paper elevation={3} sx={{ p: 4 }}>
                <Stack direction="column" spacing={2} sx={{ alignItems: 'center' }}>
                    <FormControl sx={{ minWidth: 200 }}>
                        <InputLabel id="demo-simple-select-label">Version</InputLabel>
                        <Select
                            value={version}
                            label="Age"
                            onChange={e => setVersion(e.target.value)}
                            sx={{ width: 200 }}
                        >
                            <MenuItem value={'5.3.1'}>5.3.1</MenuItem>
                            <MenuItem value={'5.4.0'}>5.4.0 [Merge Demo Only]</MenuItem>
                            <MenuItem value={'5.4.1 (beta)'}>5.4.0 (beta) [Merge Demo Only]</MenuItem>
                        </Select>
                    </FormControl>
                    <Typography variant="subtitle1">Select Components To Install</Typography>
                    <div className="components">
                        <SearchInstaller allowDisable={false} />
                    </div>
                    <div className="message">
                        {allowInstall ? (
                            <Typography variant="caption">Press install to begin installation.</Typography>
                        ) : (
                            <Typography variant="caption" color="error">
                                Please select at least one component to install.
                            </Typography>
                        )}
                    </div>
                    <Button variant="contained" sx={{ width: 200 }} onClick={install} disabled={!allowInstall}>
                        Install
                    </Button>
                </Stack>
            </Paper>
        </div>
    )
}
