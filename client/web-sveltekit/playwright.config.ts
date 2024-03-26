import type { PlaywrightTestConfig } from '@playwright/test'
import { devices } from '@playwright/test'

const PORT = process.env.PORT ? Number(process.env.PORT) : 4173

const config: PlaywrightTestConfig = {
    testMatch: 'src/**/*.spec.ts',
    reporter: 'list',
    // note: if you proxy into a locally running vite preview, you may have to raise this to 60 seconds
    timeout: 5_000,
    // webServer: {
    //     command: 'npm run preview',
    //     port: PORT,
    //     reuseExistingServer: !process.env.CI,
    // },
    use: {
        baseURL: `http://localhost:${PORT}`,
    },
    projects: [
        {
            name: 'chromium',
            use: { ...devices['Desktop Chrome'] },
        },
    ]
}

export default config
