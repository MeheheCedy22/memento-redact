import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In `npm run dev`, forward API calls to the Go server (go run ./cmd/server) so the UI stays same-origin.
const api = { target: 'http://localhost:8080', changeOrigin: true }

export default defineConfig({
  plugins: [react()],
  // Monaco is one large chunk by design; it is served from your own server and cached.
  build: { chunkSizeWarningLimit: 5000 },
  server: {
    port: 5173,
    proxy: { '/v1': api, '/health': api },
  },
})
