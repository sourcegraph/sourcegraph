import React from 'react'

import ReactDOM from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'

import { Frame } from './Frame.tsx'
import { Home } from './Home.tsx'

import './index.css'

import { Install } from './Install.tsx'
import { Maintenance } from './Maintenance.tsx'
import { Progress } from './Progress.tsx'
import { ThemeProvider } from './Theme.tsx'
import { WaitForAdmin } from './WaitForAdmin.tsx'

const router = createBrowserRouter([
    {
        path: '/',
        element: <Frame />,
        children: [
            {
                path: '',
                element: <Home />,
            },
            {
                path: 'install',
                element: <Install />,
            },
            {
                path: 'install/progress',
                element: <Progress action="install" />,
            },
            {
                path: 'install/wait-for-admin',
                element: <WaitForAdmin />,
            },
            {
                path: 'upgrade/progress',
                element: <Progress action="upgrade" />,
            },
            {
                path: 'maintenance',
                element: <Maintenance />,
            },
        ],
    },
])

ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
        <ThemeProvider>
            <RouterProvider router={router} />
        </ThemeProvider>
    </React.StrictMode>
)
