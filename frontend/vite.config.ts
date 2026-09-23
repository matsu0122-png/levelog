import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
// `vitest/config`'s defineConfig re-exports Vite's, with the `test` field's
// types merged in — plain `vite`'s defineConfig doesn't know that field.
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: true,
    port: 5173,
  },
  preview: {
    host: true,
    port: 5173,
  },
  build: {
    // Never ship source maps to the public production bundle.
    sourcemap: false,
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
