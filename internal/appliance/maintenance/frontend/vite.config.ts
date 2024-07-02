import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const devVersion = `v0.0.${Math.floor(Date.now() / 1000 - 1703122377)}`

// https://vitejs.dev/config/
export default defineConfig({
    server: {
        port: 8889,
        proxy: {
            '/api': {
                target: 'http://127.0.0.1:8888',
            },
        },
    },
    plugins: [react()],
    define: {
        'process.env.API_ENDPOINT': JSON.stringify(''),
        'process.env.BUILD_NUMBER': JSON.stringify(
            process.env.BUILD_NUMBER ?? (process.env.NODE_ENV === 'production' ? '-no-build-number-' : devVersion)
        ),
    },
})
