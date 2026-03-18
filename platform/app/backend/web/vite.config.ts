import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => ({
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8090',
      '/ui-api': 'http://localhost:8090',
      '/metrics': 'http://localhost:8090'
    }
  },
  build: {
    rollupOptions: {
      plugins: mode === 'analyze'
        ? [import('rollup-plugin-visualizer').then(m => m.visualizer({ open: true, filename: 'bundle-stats.html' }))]
        : [],
    },
  },
}))
