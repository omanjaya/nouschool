import { useEffect, useRef, useState } from 'react';

/**
 * Klien realtime (Fase 12, docs/06 dkk — kontrak `GET /api/ws`) — satu koneksi
 * WebSocket persisten per sesi login di HOST TENANT saja (host platform/super
 * admin tidak punya endpoint ini, lihat `useRealtimeConnection`). Native
 * `WebSocket` + `setTimeout`/`setInterval` browser, TANPA dependency baru.
 *
 * Event dari server SELALU berbentuk sinyal `{type, data}` — pemanggil tidak
 * pernah memakai `data` sebagai sumber data langsung, hanya penanda "ada yang
 * berubah, refetch lewat REST existing" (lihat mapping di `App.tsx`).
 */

/** Sepuluh tipe event sinyal dari backend (belum termasuk `hello`/`pong` — keduanya ditangani internal, tidak pernah sampai ke `subscribe`). */
export type RealtimeEventType =
  | 'attendance.session'
  | 'attendance.summary'
  | 'teaching.status'
  | 'notification'
  | 'leave'
  | 'announcement'
  | 'schedule'
  | 'billing'
  | 'students'
  | 'discipline'
  | 'studentleave'
  | 'teacherqr'
  | 'exitpermit'
  | 'latearrival'
  | 'grading';

const ALL_REALTIME_EVENT_TYPES: RealtimeEventType[] = [
  'attendance.session',
  'attendance.summary',
  'teaching.status',
  'notification',
  'leave',
  'announcement',
  'schedule',
  'billing',
  'students',
  'discipline',
  'studentleave',
  'teacherqr',
  'exitpermit',
  'latearrival',
  'grading',
];

/** Bentuk minimal `data` per tipe event, sesuai kontrak — dipakai hanya untuk keputusan kecil (mis. cocokkan `session_id`), bukan sumber tampilan. */
export interface RealtimeEventDataMap {
  'attendance.session': { session_id: string | number; class_id: string; date: string };
  'attendance.summary': { date: string };
  'teaching.status': { date: string };
  notification: Record<string, never>;
  leave: { request_id: string | number };
  announcement: Record<string, never>;
  schedule: Record<string, never>;
  billing: Record<string, never>;
  students: Record<string, never>;
  /** Fase 14 Gelombang A (docs/12-sion-parity.md) — pelanggaran/surat peringatan berubah untuk satu siswa. */
  discipline: { student_id: string | number };
  /** Fase 14 Gelombang B1 (docs/12-sion-parity.md Gelombang B) — pengajuan izin siswa berubah status. */
  studentleave: { request_id: string | number };
  /** Fase 14 Gelombang B2 — QR token guru dipakai siswa (scan) → guru pemilik harus regenerate token segera. */
  teacherqr: Record<string, never>;
  /** Fase 14 Gelombang B2 — dispensasi keluar (rantai QR) berubah tahap/status. */
  exitpermit: { permit_id: string | number };
  /** Fase 14 Gelombang B2 — laporan keterlambatan berubah tahap/status. */
  latearrival: Record<string, never>;
  /** Fase 14 Gelombang C (docs/12-sion-parity.md Gelombang C) — komponen/nilai/publikasi/bintang berubah untuk satu kelas-mapel. */
  grading: { class_id: string | number };
}

export interface RealtimeEvent<T extends RealtimeEventType = RealtimeEventType> {
  type: T;
  data: RealtimeEventDataMap[T];
}

export type RealtimeConnectionState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

type EventListener = (event: RealtimeEvent) => void;
type StateListener = (state: RealtimeConnectionState) => void;

const HEARTBEAT_INTERVAL_MS = 25_000;
/** Tidak ada pesan APA PUN (termasuk pong) selama ini → anggap koneksi mati diam-diam (mis. NAT timeout), paksa reconnect. */
const DEAD_CONNECTION_MS = 60_000;
const BACKOFF_BASE_MS = 1_000;
const BACKOFF_MAX_MS = 30_000;

function wsUrl(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${proto}//${window.location.host}/api/ws`;
}

/** Backoff eksponensial 1s→2s→4s...maks 30s, dengan jitter penuh (0..base) supaya banyak client tidak reconnect di detik yang sama persis setelah outage backend. */
function backoffDelayMs(attempt: number): number {
  const base = Math.min(BACKOFF_BASE_MS * 2 ** attempt, BACKOFF_MAX_MS);
  return Math.random() * base;
}

/**
 * Klien WS bergaya observable kecil: `connect`/`disconnect` mengatur siklus
 * hidup socket, `subscribe(type, handler)` mendaftar reaksi per tipe event,
 * `subscribeState(handler)` mengikuti status koneksi (dipakai indikator UI).
 * Satu instance singleton (`realtimeClient`) dipakai seluruh app — hooks di
 * bawah adalah cara React memasangnya tanpa duplikasi socket per komponen.
 */
export class RealtimeClient {
  private ws: WebSocket | null = null;
  private state: RealtimeConnectionState = 'disconnected';
  private attempt = 0;
  private stopped = true;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private watchdogTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly eventListeners = new Set<EventListener>();
  private readonly stateListeners = new Set<StateListener>();
  private readonly handleOnline = () => {
    if (this.stopped) return;
    // Jaringan pulih — reset backoff & coba sambung sekarang juga, jangan
    // menunggu sisa delay backoff dari percobaan sebelumnya.
    this.attempt = 0;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (!this.ws) this.open();
  };

  /** Idempotent — aman dipanggil berkali-kali (mis. StrictMode double-effect) selama sudah berjalan. */
  connect(): void {
    if (!this.stopped) return;
    this.stopped = false;
    this.attempt = 0;
    window.addEventListener('online', this.handleOnline);
    this.open();
  }

  /** Tutup socket & bersihkan semua timer — WAJIB dipanggil saat logout/unmount supaya tidak ada reconnect liar di background. */
  disconnect(): void {
    this.stopped = true;
    window.removeEventListener('online', this.handleOnline);
    this.clearTimers();
    if (this.ws) {
      const socket = this.ws;
      this.ws = null;
      socket.onopen = null;
      socket.onmessage = null;
      socket.onclose = null;
      socket.onerror = null;
      socket.close();
    }
    this.setState('disconnected');
  }

  subscribe<T extends RealtimeEventType>(type: T, handler: (event: RealtimeEvent<T>) => void): () => void {
    const listener: EventListener = (event) => {
      if (event.type === type) handler(event as RealtimeEvent<T>);
    };
    this.eventListeners.add(listener);
    return () => {
      this.eventListeners.delete(listener);
    };
  }

  subscribeState(handler: StateListener): () => void {
    this.stateListeners.add(handler);
    handler(this.state);
    return () => {
      this.stateListeners.delete(handler);
    };
  }

  getState(): RealtimeConnectionState {
    return this.state;
  }

  private open(): void {
    if (this.stopped) return;
    this.setState(this.attempt === 0 ? 'connecting' : 'reconnecting');

    let socket: WebSocket;
    try {
      socket = new WebSocket(wsUrl());
    } catch {
      this.scheduleReconnect();
      return;
    }
    this.ws = socket;

    socket.onopen = () => {
      if (this.ws !== socket) return;
      this.attempt = 0;
      this.setState('connected');
      this.startHeartbeat();
      this.resetWatchdog();
    };

    socket.onmessage = (ev) => {
      if (this.ws !== socket) return;
      // Pesan apa pun (bukan cuma pong) membuktikan koneksi masih hidup.
      this.resetWatchdog();

      let parsed: { type?: unknown; data?: unknown };
      try {
        parsed = JSON.parse(ev.data as string);
      } catch {
        return;
      }
      if (typeof parsed.type !== 'string') return;
      // "hello" (salam awal) & "pong" (balasan heartbeat) murni sinyal
      // transport — tidak pernah diteruskan ke subscriber fitur.
      if (parsed.type === 'hello' || parsed.type === 'pong') return;

      const event = { type: parsed.type, data: parsed.data } as RealtimeEvent;
      this.eventListeners.forEach((listener) => listener(event));
    };

    socket.onclose = () => {
      if (this.ws !== socket) return;
      this.ws = null;
      this.clearHeartbeat();
      this.clearWatchdog();
      if (this.stopped) return;
      this.scheduleReconnect();
    };

    socket.onerror = () => {
      // Browser selalu memanggil onclose setelah onerror — reconnect dijadwalkan di sana.
    };
  }

  private scheduleReconnect(): void {
    if (this.stopped) return;
    this.setState('reconnecting');
    const delay = backoffDelayMs(this.attempt);
    this.attempt += 1;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.open();
    }, delay);
  }

  private startHeartbeat(): void {
    this.clearHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }));
      }
    }, HEARTBEAT_INTERVAL_MS);
  }

  private clearHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private resetWatchdog(): void {
    this.clearWatchdog();
    this.watchdogTimer = setTimeout(() => {
      this.ws?.close();
    }, DEAD_CONNECTION_MS);
  }

  private clearWatchdog(): void {
    if (this.watchdogTimer) {
      clearTimeout(this.watchdogTimer);
      this.watchdogTimer = null;
    }
  }

  private clearTimers(): void {
    this.clearHeartbeat();
    this.clearWatchdog();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private setState(state: RealtimeConnectionState): void {
    if (this.state === state) return;
    this.state = state;
    this.stateListeners.forEach((listener) => listener(state));
  }
}

/** Satu instance dipakai seluruh app — lihat komentar di atas `RealtimeClient`. */
export const realtimeClient = new RealtimeClient();

/**
 * Pasang di titik paling atas yang hidup sepanjang sesi login (App.tsx,
 * SEBELUM early-return apa pun — lihat aturan rules-of-hooks), bukan di
 * `AuthenticatedShell` — role `display` tidak pernah merender shell itu
 * (langsung diarahkan ke `/tv`) tapi tetap butuh koneksi realtime.
 * `enabled` = sedang login DAN di host tenant (`Boolean(me?.school)`).
 */
export function useRealtimeConnection(enabled: boolean): void {
  useEffect(() => {
    if (!enabled) return;
    realtimeClient.connect();
    return () => realtimeClient.disconnect();
  }, [enabled]);
}

/** Status koneksi realtime saat ini, reaktif — dipakai indikator UI (titik TV, garis "Menyambung ulang…"). */
export function useRealtimeState(): RealtimeConnectionState {
  const [state, setState] = useState<RealtimeConnectionState>(() => realtimeClient.getState());
  useEffect(() => realtimeClient.subscribeState(setState), []);
  return state;
}

/**
 * Daftarkan reaksi per tipe event sekali per mount — `handlers` boleh berupa
 * literal objek baru tiap render (pola "latest ref"), pemanggil TIDAK perlu
 * `useCallback` tiap handler. Handler yang tidak disertakan untuk suatu tipe
 * cukup diabaikan (tidak ada langganan ekstra dibuat/dibongkar per render).
 */
export function useRealtimeEvents(
  handlers: Partial<{ [K in RealtimeEventType]: (event: RealtimeEvent<K>) => void }>,
): void {
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;

  useEffect(() => {
    // Diindeks lewat tipe longgar di sini (bukan mapped type publik di atas)
    // supaya TS tidak memperlakukan `handlersRef.current[type]` sebagai
    // gabungan fungsi yang saling kontravarian (union-of-functions) — bentuk
    // publik hook ini tetap type-safe per pemanggil.
    const looseHandlers = handlersRef as { current: Record<string, ((event: RealtimeEvent) => void) | undefined> };
    const unsubscribers = ALL_REALTIME_EVENT_TYPES.map((type) =>
      realtimeClient.subscribe(type, (event) => {
        looseHandlers.current[type]?.(event);
      }),
    );
    return () => unsubscribers.forEach((unsubscribe) => unsubscribe());
    // Sengaja hanya sekali per mount — handler terbaru selalu dibaca lewat `handlersRef`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
}
