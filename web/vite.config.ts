import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      // Fase 9 (docs/08-notification.md): strategi custom `injectManifest` — SW
      // sendiri di src/sw.ts precache manifest + handler `push`/`notificationclick`
      // untuk Web Push. Sebelumnya `generateSW` (SW auto-generate tanpa handler push).
      strategies: 'injectManifest',
      srcDir: 'src',
      filename: 'sw.ts',
      injectManifest: {
        // SW sudah memanggil cleanupOutdatedCaches() sendiri (src/sw.ts).
        globPatterns: ['**/*.{js,css,html,svg,png,ico}'],
      },
      devOptions: {
        enabled: false,
      },
      manifest: {
        name: 'NouSchool',
        short_name: 'NouSchool',
        description: 'Absensi, izin guru, dan monitoring sekolah NouSchool',
        theme_color: '#0E6B4E',
        background_color: '#FFFFFF',
        display: 'standalone',
        start_url: '/',
        icons: [
          {
            src: '/pwa-icon.svg',
            sizes: 'any',
            type: 'image/svg+xml',
            purpose: 'any',
          },
        ],
      },
    }),
  ],
  server: {
    host: true,
    // Backend dev di 8210 (8080 direserve Hyper-V — lihat README).
    // Di dalam docker compose, set VITE_API_PROXY=http://api:8080
    proxy: {
      '/api': {
        target: process.env.VITE_API_PROXY ?? 'http://localhost:8210',
        // Host header asli (mis. demo.localhost:5173) HARUS diteruskan —
        // backend memakai Host untuk resolusi tenant multi-sekolah.
        changeOrigin: false,
      },
    },
  },
})
