# 07 — Leave: Izin Guru dengan Approval Engine Konfigurable

Modul: `internal/leave/`

## Settings (module `leave` di school_settings)

```go
type Settings struct {
    Types []LeaveType `json:"types"`  // izin, sakit, cuti, dinas_luar — editable per sekolah
    Chain []ChainStep `json:"chain"`  // urutan approval
}
type ChainStep struct {
    Role string `json:"role"`  // 'kepala_sekolah' | 'waka' | ... ATAU
    UserID int64 `json:"user_id,omitempty"` // approver spesifik (opsional)
}
// Default: Types [izin, sakit, dinas_luar], Chain [{role: kepala_sekolah}] (1 tingkat)
```

## Skema — steps di-SNAPSHOT saat pengajuan

Prinsip: request yang sedang berjalan tidak boleh rusak karena admin mengubah konfigurasi chain. Saat guru submit, chain dari settings **disalin** menjadi baris-baris `leave_approval_steps` milik request itu.

```sql
leave_requests (
  id          bigserial PRIMARY KEY,
  school_id   bigint NOT NULL,
  teacher_id  bigint NOT NULL,
  type        text NOT NULL,             -- dari Settings.Types
  date_start  date NOT NULL,
  date_end    date NOT NULL,
  reason      text NOT NULL,
  attachment  text,                      -- path file (surat dokter dsb), opsional
  status      text NOT NULL DEFAULT 'pending',  -- pending|approved|rejected|canceled
  created_at  timestamptz NOT NULL DEFAULT now()
)

leave_approval_steps (
  id               bigserial PRIMARY KEY,
  school_id        bigint NOT NULL,
  leave_request_id bigint NOT NULL REFERENCES leave_requests,
  step_order       int NOT NULL,         -- 1, 2, ...
  approver_role    text NOT NULL,
  approver_user_id bigint,               -- terisi jika chain menunjuk orang spesifik
  decided_by       bigint,               -- NULL = belum diputuskan
  decision         text,                 -- approved|rejected
  decided_at       timestamptz,
  comment          text,
  UNIQUE (leave_request_id, step_order)
)
```

## Aturan engine

- Step diputuskan **berurutan**: step N baru bisa diputuskan setelah step N-1 approved. Approver melihat antrian "menunggu keputusan Anda" (step aktif yang role/user-nya cocok).
- Status request = derivasi: ada step rejected → `rejected` (step berikutnya tidak berjalan); semua approved → `approved`; selainnya `pending`.
- Guru boleh `cancel` selama masih `pending`.
- Approver tidak boleh memutuskan request miliknya sendiri (kepsek mengajukan izin → butuh fallback: jika seluruh chain tereduksi kosong karena self-approval, request otomatis approved dengan catatan `auto_approved_self_chain` di audit_log — kasus sekolah kecil; sederhana dan jujur).
- Perubahan keputusan setelah diputuskan: tidak bisa; jalur koreksi = guru cancel (jika masih relevan) atau `admin_sekolah` membatalkan dengan alasan (audit_log).
- Setiap keputusan → notifikasi ke pengaju (dan approver step berikutnya) via modul notification.

## Kaitan modul lain

- **teaching (06)**: guru dengan leave approved pada tanggal berjalan tampil "Izin" (kuning) di TV/monitoring — via interface `LeaveChecker.ApprovedOn(teacherID, date)`.
- **dashboard**: rekap izin per guru per rentang (jumlah hari per tipe).
- Kalau nanti butuh kuota cuti tahunan: tambah `quota` di Settings + hitung dari requests approved — jangan bangun sekarang.
