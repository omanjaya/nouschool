/**
 * Web Push (docs/08-notification.md) — daftar/putus langganan push lewat service
 * worker `src/sw.ts` (injectManifest, lihat vite.config.ts). Semua fungsi di sini
 * dipanggil dari UI (Profil, banner Beranda) setelah gestur pengguna eksplisit
 * ("Aktifkan") — permintaan izin browser tidak pernah dipicu otomatis saat load.
 */
import { api } from './api';
import type { PushPublicKey, PushSubscribeInput } from './types';

export type PushState = 'unsupported' | 'denied' | 'enabled' | 'disabled';

/** VAPID public key (base64url) → Uint8Array yang diminta `applicationServerKey`. */
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; i += 1) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

export function isPushSupported(): boolean {
  return (
    typeof window !== 'undefined' &&
    'serviceWorker' in navigator &&
    'PushManager' in window &&
    'Notification' in window
  );
}

/** true di iPhone/iPad/iPod — Web Push di iOS hanya jalan setelah PWA di-install ke home screen. */
export function isIOS(): boolean {
  return /iphone|ipad|ipod/i.test(navigator.userAgent);
}

/** true kalau app berjalan sebagai PWA terinstal (standalone), bukan tab browser biasa. */
export function isStandalone(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(display-mode: standalone)').matches;
}

/** Status langganan push perangkat ini saat ini — dipakai UI Profil & banner Beranda. */
export async function getPushState(): Promise<PushState> {
  if (!isPushSupported()) return 'unsupported';
  if (Notification.permission === 'denied') return 'denied';

  // getRegistration() (bukan `.ready`) supaya tidak menggantung tanpa batas di
  // dev (devOptions.enabled: false di vite.config.ts — SW tidak terdaftar saat `vite dev`).
  const registration = await navigator.serviceWorker.getRegistration();
  if (!registration) return 'disabled';

  const subscription = await registration.pushManager.getSubscription();
  return subscription ? 'enabled' : 'disabled';
}

/** Minta izin notifikasi + daftarkan subscription push, lalu kirim ke backend. */
export async function enablePush(): Promise<void> {
  if (!isPushSupported()) {
    throw new Error('Push tidak didukung di perangkat/browser ini.');
  }

  const permission = await Notification.requestPermission();
  if (permission !== 'granted') {
    throw new Error('Izin notifikasi ditolak.');
  }

  const registration = await navigator.serviceWorker.ready;
  const { key } = await api.get<PushPublicKey>('/push/public-key');

  const subscription = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(key) as BufferSource,
  });

  const json = subscription.toJSON();
  if (!json.endpoint || !json.keys?.p256dh || !json.keys?.auth) {
    throw new Error('Subscription push tidak lengkap.');
  }

  const input: PushSubscribeInput = {
    endpoint: json.endpoint,
    p256dh: json.keys.p256dh,
    auth: json.keys.auth,
  };
  await api.post<undefined>('/push/subscribe', input);
}

/** Putuskan subscription push perangkat ini, lalu beri tahu backend. */
export async function disablePush(): Promise<void> {
  const registration = await navigator.serviceWorker.getRegistration();
  const subscription = await registration?.pushManager.getSubscription();
  if (!subscription) return;

  const endpoint = subscription.endpoint;
  await subscription.unsubscribe();
  await api.post<undefined>('/push/unsubscribe', { endpoint });
}
