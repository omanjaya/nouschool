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
    // Backend dev jalan di 8210 (port 8080 direserve Hyper-V di mesin ini — lihat README)
    proxy: {
      '/api': 'http://localhost:8210',
    },
  },
})
