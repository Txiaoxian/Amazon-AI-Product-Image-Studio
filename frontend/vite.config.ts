import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/relay2': {
        target: 'https://api.flymux.com',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/relay2/, ''),
      },
    },
  },
})
