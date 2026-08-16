# `components/blocks/` — bahan mentah dari registry, WAJIB diadaptasi

Folder ini menampung block yang diambil lewat `npx shadcn@latest add
@shadcnblocks/<nama-block>` (atau registry shadcn/ui resmi). Block di sini
**bukan** produk jadi — mereka BAHAN yang harus disesuaikan ke design system
"Rapor" (`docs/10-design-system.md`, FINAL, jangan didesain ulang) sebelum
dipakai di fitur mana pun.

Jangan pernah render langsung sebuah block dari folder ini di halaman
produk tanpa melewati checklist di bawah.

## Kenapa terpisah dari `src/components/ui/`

`src/components/ui/` berisi komponen bersama Rapor yang sudah final dan
dipakai lintas fitur (Button, Card, ListRow, StatusChip, EmptyState, dst —
lihat daftar di docs/10 §6). Block registry TIDAK masuk ke situ — mereka
punya styling default shadcn (palet neutral/gray, shadow, dst) yang belum
tentu sesuai Rapor, dan kalau ditaruh campur bisa diam-diam menggeser
konvensi yang sudah dikunci user.

## Checklist wajib sebelum sebuah block dianggap selesai diadaptasi

Setiap block yang diambil dari registry harus lolos SEMUA poin ini
(mengacu ke aturan UI di `CLAUDE.md` dan `docs/10-design-system.md`)
sebelum dipindah/dipakai di `src/features/*`:

1. **Warna** — ganti semua kelas palet default Tailwind (`gray-*`, `zinc-*`,
   `slate-*`, `neutral-*`, atau warna primary bawaan shadcn) ke kelas
   semantik token Rapor: `bg-bg`, `bg-surface`, `bg-surface-2`, `text-ink`,
   `text-muted`, `border-line`, `bg-primary` / `text-primary-ink` /
   `bg-primary-soft`, `text-danger` / `bg-danger-soft`, `text-st-*`. Tidak
   ada hex/rgb hardcode.
2. **Radius** — samakan ke 8px (kelas `rounded-lg` sesuai `--radius` token,
   atau `rounded-full`/`999px` khusus chip/pill). Hapus radius lain yang
   dibawa block.
3. **Elevasi** — hapus shadow dekoratif. Shadow HANYA boleh dipertahankan
   untuk overlay (dialog/sheet/dropdown), memakai token yang sama dengan
   `Dialog.tsx` (`0 8px 24px rgba(23,35,58,0.12)`). Semua permukaan lain:
   hairline `border-line`, tanpa shadow.
   Hapus juga gradien dan efek glassmorphism/blur — dilarang keras.
4. **Icon** — ganti seluruh icon ke `lucide-react` (`currentColor`,
   stroke-width 2). Tidak ada icon set lain, tidak ada emoji di teks/label.
5. **Tipografi & spacing** — sesuaikan ke skala docs/10 §2-3 (11/12/14/16/
   18/21/28px; spacing 4/8/12/16/20/24/32; bobot 400/600/700 saja).
6. **Tidak menggantikan komponen bersama existing** — kalau block membawa
   primitif yang namanya bentrok dengan `src/components/ui/` (Button,
   Card, Dialog, dst), JANGAN biarkan CLI menimpa file di `ui/`. Kalau
   sudah kepasang, hapus/rename file baru itu dan pakai versi Rapor kita.
   Varian yang benar-benar baru ditambahkan sebagai varian di komponen
   bersama, bukan komponen paralel.
7. **State wajib** — kalau block ini akan dipakai sebagai layar/section
   produk: pastikan tetap ada loading (Skeleton), kosong (EmptyState),
   error (ErrorState + retry), sukses (Toast) sesuai pola fitur lain.
8. Setelah lolos semua poin di atas, **pindahkan** hasil adaptasi keluar
   dari `blocks/` ke folder fitur terkait (`src/features/<fitur>/`), atau
   biarkan di sini hanya jika benar-benar dipakai lintas fitur DAN sudah
   100% mengikuti token Rapor (bukan lagi "bahan mentah").

## Alur kerja

CLI shadcn TIDAK tahu soal folder `blocks/` ini — alias `components` di
`components.json` mengarah ke `@/components` (root), jadi file block
(mis. `hero1.tsx`) mendarat langsung di `src/components/hero1.tsx`, bukan
otomatis ke sini. Primitif pendukungnya (mis. `badge.tsx`, `button.tsx`)
mendarat di `src/components/ui/` lewat alias `ui` — cek diff dulu (poin 6)
sebelum menimpa apa pun di sana.

```
npx shadcn@latest add @shadcnblocks/<nama-block> --dry-run   # cek dulu
npx shadcn@latest add @shadcnblocks/<nama-block>              # lalu apply
# → block mendarat di src/components/<nama-block>.tsx — PINDAHKAN manual
#   ke src/components/blocks/ sebelum disentuh lebih lanjut
# → primitif pendukung mendarat di src/components/ui/ — cek tidak menimpa
#   komponen Rapor existing (poin 6)
# → adaptasi manual sesuai checklist di atas, di dalam blocks/
# → pindah ke src/features/<fitur>/ setelah lolos
```

Lihat `../../../README.md` bagian "Frontend blocks (shadcnblocks)" untuk
cara set API key registry.
