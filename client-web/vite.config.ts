import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
    plugins: [react(), tailwindcss()],
    server: {
        // same origin as the go server in production, which embeds the build
        proxy: { '/api': 'http://localhost:8080' },
    },
})
