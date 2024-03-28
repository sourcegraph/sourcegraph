import { defineProjectWithDefaults } from '../../vitest.shared'

export default defineProjectWithDefaults(__dirname, {
    test: {
        environment: 'jsdom',
    },
})
