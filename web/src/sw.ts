/// <reference lib="webworker" />

/**
 * Service worker custom (strategi `injectManifest`, lihat vite.config.ts) — dua
 * tanggung jawab: (1) precache app shell lewat manifest yang di-generate Vite,
 * (2) tampilkan Web Push masuk & buka tautan saat notifikasi diklik (docs/08).
 *
 * Precompiled terpisah dari `tsconfig.app.json` (lib DOM) karena file ini butuh
 * lib `webworker` — lihat `tsconfig.worker.json`.
 */
import { cleanupOutdatedCaches, precacheAndRoute } from 'workbox-precaching';
import { clientsClaim } from 'workbox-core';

interface WebPushPayload {
  title?: string;
  body?: string;
  link?: string;
}

// Retipe `self` (bukan buat binding baru — `const self = self as unknown as ...`
// dinamai-ulang jadi identifier lain oleh bundler karena bentrok dengan `self`
// yang dipakai modul workbox internal, yang bikin literal `self.__WB_MANIFEST`
// di bawah hilang dan `injectManifest` gagal menemukan titik suntik). `declare
// const` murni penanda tipe (dihapus saat kompilasi), setara `self as unknown
// as ServiceWorkerGlobalScope` tapi tanpa risiko itu.
declare const self: ServiceWorkerGlobalScope & {
  __WB_MANIFEST: Array<{ url: string; revision: string | null }>;
};

// registerType: 'autoUpdate' (vite.config.ts) — SW baru langsung mengambil alih
// tanpa menunggu semua tab lama ditutup.
self.skipWaiting();
clientsClaim();

cleanupOutdatedCaches();
precacheAndRoute(self.__WB_MANIFEST);

self.addEventListener('push', (event: PushEvent) => {
  let payload: WebPushPayload = {};
  try {
    if (event.data) payload = event.data.json() as WebPushPayload;
  } catch {
    // Payload bukan JSON valid — tampilkan notifikasi generik di bawah.
  }

  const title = payload.title?.trim() || 'NouSchool';
  const body = payload.body ?? '';
  const link = payload.link ?? '/';

  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      data: { link },
    }),
  );
});

self.addEventListener('notificationclick', (event: NotificationEvent) => {
  event.notification.close();
  const link = (event.notification.data as { link?: string } | undefined)?.link ?? '/';

  event.waitUntil(
    (async () => {
      const allClients = await self.clients.matchAll({ type: 'window', includeUncontrolled: true });
      const targetUrl = new URL(link, self.location.origin).href;

      for (const client of allClients) {
        if ('focus' in client) {
          if ('navigate' in client) {
            await (client as WindowClient).navigate(targetUrl);
          }
          await (client as WindowClient).focus();
          return;
        }
      }

      await self.clients.openWindow(targetUrl);
    })(),
  );
});
