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
      '/api': process.env.VITE_API_PROXY ?? 'http://localhost:8210',
    },
  },
})
